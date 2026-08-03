// Package hostit is a thin client for the HostIt local agent HTTP API
// (default http://127.0.0.1:7003). Compatible with HostIt's Go SDK surface:
// https://github.com/32bitx64bit/HostIt
package hostit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultAgentURL = "http://127.0.0.1:7003"
const DefaultRouteName = "openinfer-api"

type envelope struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

// Client talks to a running HostIt agent.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultAgentURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type RegisterRequest struct {
	Name       string `json:"name"`
	Proto      string `json:"proto"`
	LocalPort  int    `json:"local_port"`
	LocalHost  string `json:"local_host,omitempty"`
	PublicPort int    `json:"public_port,omitempty"`
	Domain     string `json:"domain,omitempty"`
}

type RegisterResponse struct {
	Status     string `json:"status"`
	RequestID  string `json:"request_id,omitempty"`
	RouteName  string `json:"route_name"`
	PublicAddr string `json:"public_addr,omitempty"`
	LocalAddr  string `json:"local_addr,omitempty"`
	Proto      string `json:"proto,omitempty"`
	Domain     string `json:"domain,omitempty"`
}

type StatusResponse struct {
	Connected   bool   `json:"connected"`
	Server      string `json:"server"`
	Version     string `json:"version"`
	RoutesCount int    `json:"routes_count"`
	DomainBase  string `json:"domain_base,omitempty"`
}

// ServerInfoResponse is returned by GET /api/v1/server.
// PublicAddr is the tunnel server hostname or IP with no port.
type ServerInfoResponse struct {
	PublicAddr string `json:"public_addr"`
	Connected  bool   `json:"connected"`
}

type Route struct {
	Name       string `json:"name"`
	Proto      string `json:"proto"`
	PublicAddr string `json:"public_addr"`
	LocalAddr  string `json:"local_addr"`
	Domain     string `json:"domain,omitempty"`
}

// JoinPublicAddr builds a dialable host:port from the tunnel server address
// and a route public_addr. Route addresses are often port-only (":47998").
func JoinPublicAddr(serverHost, routePublicAddr string) string {
	route := strings.TrimSpace(routePublicAddr)
	host := strings.TrimSpace(serverHost)
	if route == "" {
		return ""
	}
	if !strings.HasPrefix(route, ":") {
		return route
	}
	if host == "" {
		return route
	}
	return host + route
}

func (c *Client) do(ctx context.Context, method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("hostit agent unreachable (%s): %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var e envelope
		if json.Unmarshal(raw, &e) == nil && e.Message != "" {
			return fmt.Errorf("%s", e.Message)
		}
		return fmt.Errorf("hostit request failed: %d", resp.StatusCode)
	}
	if len(raw) == 0 || out == nil {
		return nil
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return err
	}
	if e.Status != "ok" {
		if e.Message != "" {
			return fmt.Errorf("%s", e.Message)
		}
		return fmt.Errorf("hostit status %q", e.Status)
	}
	if len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, out)
}

func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	var out StatusResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ServerInfo(ctx context.Context) (*ServerInfoResponse, error) {
	var out ServerInfoResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/server", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	if req.Proto == "" {
		req.Proto = "tcp"
	}
	if req.LocalHost == "" {
		req.LocalHost = "127.0.0.1"
	}
	var out RegisterResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/register", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RemoveRoute(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/routes/"+urlPathEscape(name), nil, nil)
}

func (c *Client) ListRoutes(ctx context.Context) ([]Route, error) {
	var out []Route
	if err := c.do(ctx, http.MethodGet, "/api/v1/routes", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Route{}
	}
	return out, nil
}

func urlPathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}
