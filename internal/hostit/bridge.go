package hostit

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/openinfer/openinfer-studio/internal/config"
)

// Bridge keeps an OpenInfer public-API port registered with a HostIt agent.
type Bridge struct {
	settings *config.Settings
	log      *slog.Logger

	mu     sync.Mutex
	client *Client
}

func NewBridge(settings *config.Settings, log *slog.Logger) *Bridge {
	return &Bridge{settings: settings, log: log}
}

func (b *Bridge) Enabled() bool {
	return b.settings.Get("hostit.enabled", "0") == "1"
}

func (b *Bridge) agentURL() string {
	return b.settings.Get("hostit.agent_url", DefaultAgentURL)
}

func (b *Bridge) routeName() string {
	n := b.settings.Get("hostit.route_name", DefaultRouteName)
	if n == "" {
		return DefaultRouteName
	}
	return n
}

func (b *Bridge) domainMode() string {
	// "" = port-only, "auto" = HostIt domain suggestion
	return b.settings.Get("hostit.domain", "")
}

func (b *Bridge) clientLocked() *Client {
	url := b.agentURL()
	if b.client == nil || b.client.baseURL != strings.TrimRight(url, "/") {
		b.client = NewClient(url)
	}
	return b.client
}

// Status reports agent reachability and current tunnel info for the UI.
func (b *Bridge) Status(ctx context.Context) map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	routeAddr := b.settings.Get("hostit.route_public_addr", "")
	if routeAddr == "" {
		// Older installs only stored the (possibly port-only) joined field.
		routeAddr = b.settings.Get("hostit.public_addr", "")
	}
	serverAddr := b.settings.Get("hostit.server_addr", "")
	out := map[string]any{
		"enabled":       b.Enabled(),
		"agent_url":     b.agentURL(),
		"route_name":    b.routeName(),
		"domain":        b.domainMode(),
		"public_addr":   JoinPublicAddr(serverAddr, routeAddr),
		"server_addr":   serverAddr,
		"public_domain": b.settings.Get("hostit.public_domain", ""),
		"last_error":    b.settings.Get("hostit.last_error", ""),
		"agent":         map[string]any{"reachable": false, "connected": false},
	}
	c := b.clientLocked()
	st, err := c.Status(ctx)
	if err != nil {
		out["agent"] = map[string]any{"reachable": false, "connected": false, "error": err.Error()}
		return out
	}
	out["agent"] = map[string]any{
		"reachable":    true,
		"connected":    st.Connected,
		"server":       st.Server,
		"version":      st.Version,
		"routes_count": st.RoutesCount,
		"domain_base":  st.DomainBase,
	}
	if info, err := c.ServerInfo(ctx); err == nil && info != nil && info.PublicAddr != "" {
		serverAddr = info.PublicAddr
		_ = b.settings.Set("hostit.server_addr", serverAddr)
		out["server_addr"] = serverAddr
		out["public_addr"] = JoinPublicAddr(serverAddr, routeAddr)
	}
	return out
}

// SetConfig updates HostIt preferences. Does not register/unregister by itself.
func (b *Bridge) SetConfig(enabled bool, agentURL, domain, routeName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = b.settings.Set("hostit.enabled", bool01(enabled))
	if agentURL != "" {
		_ = b.settings.Set("hostit.agent_url", agentURL)
	}
	_ = b.settings.Set("hostit.domain", domain)
	if routeName != "" {
		_ = b.settings.Set("hostit.route_name", routeName)
	}
	b.client = NewClient(b.agentURL())
}

// Sync ensures the tunnel matches public-API running state.
// When running=true and HostIt is enabled, registers (or refreshes) the route.
// When running=false, removes the route if we previously registered it.
func (b *Bridge) Sync(ctx context.Context, localPort int, running bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !running || !b.Enabled() {
		return b.unregisterLocked(ctx)
	}
	if localPort < 1 {
		return fmt.Errorf("invalid local port for HostIt registration")
	}
	return b.registerLocked(ctx, localPort)
}

func (b *Bridge) registerLocked(ctx context.Context, localPort int) error {
	name := b.routeName()
	c := b.clientLocked()
	// Best-effort remove stale registration so port/domain updates apply.
	_ = c.RemoveRoute(ctx, name)

	resp, err := c.Register(ctx, RegisterRequest{
		Name:      name,
		Proto:     "tcp",
		LocalPort: localPort,
		LocalHost: "127.0.0.1",
		Domain:    b.domainMode(),
	})
	if err != nil {
		_ = b.settings.Set("hostit.last_error", err.Error())
		b.log.Warn("hostit register failed", "err", err)
		return err
	}

	serverAddr := ""
	if info, err := c.ServerInfo(ctx); err == nil && info != nil {
		serverAddr = info.PublicAddr
	}
	_ = b.settings.Set("hostit.server_addr", serverAddr)
	_ = b.settings.Set("hostit.route_public_addr", resp.PublicAddr)
	full := JoinPublicAddr(serverAddr, resp.PublicAddr)
	_ = b.settings.Set("hostit.public_addr", full)

	if resp.Status == "pending_domain" {
		msg := "HostIt needs a domain selection in the agent dashboard (pending_domain)"
		_ = b.settings.Set("hostit.last_error", msg)
		return fmt.Errorf("%s", msg)
	}
	_ = b.settings.Set("hostit.last_error", "")
	_ = b.settings.Set("hostit.public_domain", resp.Domain)
	b.log.Info("hostit route active",
		"name", resp.RouteName, "public", full, "domain", resp.Domain)
	return nil
}

func (b *Bridge) unregisterLocked(ctx context.Context) error {
	name := b.routeName()
	err := b.clientLocked().RemoveRoute(ctx, name)
	_ = b.settings.Set("hostit.public_addr", "")
	_ = b.settings.Set("hostit.route_public_addr", "")
	_ = b.settings.Set("hostit.server_addr", "")
	_ = b.settings.Set("hostit.public_domain", "")
	if err != nil {
		// Agent offline / route already gone — not fatal for local stop.
		b.log.Debug("hostit unregister", "err", err)
		_ = b.settings.Set("hostit.last_error", "")
		return nil
	}
	_ = b.settings.Set("hostit.last_error", "")
	return nil
}

func bool01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// Quick context helper for sync from HTTP handlers.
func (b *Bridge) SyncTimeout(localPort int, running bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return b.Sync(ctx, localPort, running)
}
