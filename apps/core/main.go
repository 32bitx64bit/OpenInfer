// openinfer-core is the Go backend of OpenInfer Studio. The desktop launcher
// starts it with a random session token, a loopback port and the launcher
// PID; the backend prints a structured readiness line on stdout and exits
// when the parent desktop process disappears.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/openinfer/openinfer-studio/internal/api"
	"github.com/openinfer/openinfer-studio/internal/auth"
	"github.com/openinfer/openinfer-studio/internal/chat"
	"github.com/openinfer/openinfer-studio/internal/config"
	"github.com/openinfer/openinfer-studio/internal/database"
	"github.com/openinfer/openinfer-studio/internal/diagnostics"
	"github.com/openinfer/openinfer-studio/internal/downloads"
	"github.com/openinfer/openinfer-studio/internal/hardware"
	"github.com/openinfer/openinfer-studio/internal/huggingface"
	"github.com/openinfer/openinfer-studio/internal/instances"
	"github.com/openinfer/openinfer-studio/internal/models"
	"github.com/openinfer/openinfer-studio/internal/proxy"
	"github.com/openinfer/openinfer-studio/internal/runtimes"
	"github.com/openinfer/openinfer-studio/migrations"
)

func main() {
	var (
		tokenFlag = flag.String("token", "", "session token issued by the desktop launcher (required)")
		portFlag  = flag.Int("port", 0, "loopback port for the control API (0 = choose)")
		ppidFlag  = flag.Int("ppid", 0, "parent desktop PID; backend exits when it disappears")
		dataDir   = flag.String("data-dir", "", "override application data directory (tests/portable mode)")
		selfTest  = flag.Bool("selftest", false, "start, print readiness, and exit (CI smoke test)")
	)
	flag.Parse()

	if *tokenFlag == "" {
		fmt.Fprintln(os.Stderr, `{"ready":false,"error":"missing --token"}`)
		os.Exit(2)
	}

	layout, err := config.Open(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"ready":false,"error":%q}`+"\n", "creating app directories: "+err.Error())
		os.Exit(1)
	}

	logs := diagnostics.NewManager(layout.AppLogs)
	defer logs.Close()
	log := logs.Logger("core", slog.LevelInfo)
	log.Info("openinfer-core starting", "goos", runtime.GOOS, "goarch", runtime.GOARCH, "data", layout.DataDir)

	db, err := database.Open(layout.Database, migrations.FS)
	if err != nil {
		log.Error("database open failed", "err", err)
		fmt.Fprintf(os.Stderr, `{"ready":false,"error":%q}`+"\n", "database: "+err.Error())
		os.Exit(1)
	}
	defer db.Close()
	settings := config.NewSettings(db.DB)

	// Services.
	hub := api.NewHub()
	hf := huggingface.NewClient()
	if t := auth.LoadHuggingFaceToken(); t != "" {
		hf.SetToken(t)
	}
	dl := downloads.NewManager(db.DB, layout.Partial, hub, logs.Logger("downloads", slog.LevelInfo).Logger,
		func(dir string) uint64 { return hardware.Detect(dir, dir).DiskFreeModels })
	dl.SetAuthHeader(func() string {
		if t := auth.LoadHuggingFaceToken(); t != "" {
			return "Bearer " + t
		}
		return ""
	})
	if n := settings.Get("downloads.concurrency", ""); n != "" {
		var v int
		if _, err := fmt.Sscanf(n, "%d", &v); err == nil {
			dl.SetConcurrency(v)
		}
	}
	if err := dl.RecoverAfterRestart(); err != nil {
		log.Warn("download recovery failed", "err", err)
	}

	lib := models.NewLibrary(db.DB, layout.Models, hub, logs.Logger("models", slog.LevelInfo).Logger)
	rt := runtimes.NewManager(db.DB, layout.Runtimes, dl, hub, logs.Logger("runtimes", slog.LevelInfo).Logger)
	im := instances.NewManager(db.DB, rt, lib, hub, logs.Logger("instances", slog.LevelInfo).Logger,
		layout.InstLogs, layout.Temp, layout.CacheDir)
	if n := settings.Get("instances.max_loaded", ""); n != "" {
		var v int
		if _, err := fmt.Sscanf(n, "%d", &v); err == nil {
			im.SetMaxLoaded(v)
		}
	}

	// Forward structured log records to WS clients for the live Logs page.
	go func() {
		ch, unsub := logs.Subscribe()
		defer unsub()
		for e := range ch {
			hub.Publish("log.entry", e)
		}
	}()

	// Rescan library when a model download completes.
	go func() {
		ch, unsub := hub.Subscribe()
		defer unsub()
		for env := range ch {
			if env.Event != "download.state_changed" {
				continue
			}
			payload, _ := env.Payload.(map[string]any)
			if payload == nil || payload["state"] != "complete" {
				continue
			}
			if _, err := lib.Scan(); err != nil {
				log.Warn("post-download scan failed", "err", err)
			}
		}
	}()

	chatEP := &chatEndpointAdapter{im: im}
	chatSvc := chat.NewService(db.DB, hub, chatEP)
	chatSvc.SetStreaming(settings.Get("chat.streaming", "1") != "0")

	proxyEP := &proxyEndpointAdapter{im: im}
	px := proxy.NewServer(proxyEP, db.DB, logs.Logger("proxy", slog.LevelInfo).Logger)
	if err := px.LoadProfile(); err != nil {
		log.Warn("loading server profile failed", "err", err)
	}
	if px.Config().Autostart {
		if err := px.Start(); err != nil {
			log.Warn("autostarting public API failed", "err", err)
		}
	}

	// Control API.
	srv := api.NewServer(auth.Token(*tokenFlag), hub, logs.Logger("api", slog.LevelInfo).Logger)
	srv.RegisterRoutes(&api.Deps{
		Hub: hub, Layout: layout, DB: db, Settings: settings, HF: hf, DL: dl,
		RT: rt, Lib: lib, IM: im, Chat: chatSvc, Proxy: px, Logs: logs,
	})
	if err := srv.Start(*portFlag); err != nil {
		fmt.Fprintf(os.Stderr, `{"ready":false,"error":%q}`+"\n", "bind: "+err.Error())
		os.Exit(1)
	}

	// Structured readiness message consumed by the desktop launcher.
	ready, _ := json.Marshal(map[string]any{
		"ready": true, "port": srv.BoundPort(), "pid": os.Getpid(), "version": "0.1.0",
	})
	fmt.Println(string(ready))

	if *selfTest {
		log.Info("selftest complete")
		return
	}

	// Parent-death watchdog: exit when the launcher disappears so inference
	// processes (and this backend) are never abandoned.
	if *ppidFlag > 0 {
		go watchParent(*ppidFlag, log, func() {
			shutdown(log, im, px, srv)
			os.Exit(0)
		})
	}

	// Signal handling for standalone/dev usage.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	shutdown(log, im, px, srv)
}

func shutdown(log *diagnostics.Logger, im *instances.Manager, px *proxy.Server, srv *api.Server) {
	log.Info("shutting down")
	px.Stop()
	im.StopAll()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// watchParent polls parent liveness (kill(pid,0) on unix; OpenProcess on
// Windows via the platform file).
func watchParent(ppid int, log *diagnostics.Logger, onDead func()) {
	for {
		if !parentAlive(ppid) {
			log.Info("parent process gone; exiting", "ppid", ppid)
			onDead()
			return
		}
		time.Sleep(2 * time.Second)
	}
}
