package singbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liulei/proxymorph/internal/config"
	"github.com/liulei/proxymorph/internal/convert"
)

type Manager struct {
	cfg       config.VLESSRelayConfig
	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan error
	logOutput io.Writer
}

func NewManager(cfg config.VLESSRelayConfig) *Manager {
	return &Manager{cfg: cfg, logOutput: log.Writer()}
}

func (m *Manager) SetLogOutput(output io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if output == nil {
		output = log.Writer()
	}
	m.logOutput = output
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	if err := New(m.cfg.SingBoxPath).Available(); err != nil {
		return err
	}
	if err := m.stopLocked(); err != nil {
		return err
	}
	cmd := exec.Command(m.cfg.SingBoxPath, "run", "-c", m.cfg.ConfigPath)
	var earlyOutput bytes.Buffer
	cmd.Stdout = io.MultiWriter(m.logOutput, &earlyOutput)
	cmd.Stderr = io.MultiWriter(m.logOutput, &earlyOutput)
	if err := cmd.Start(); err != nil {
		return err
	}
	m.cmd = cmd
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	m.done = done
	select {
	case err := <-done:
		m.cmd = nil
		m.done = nil
		if err != nil {
			return fmt.Errorf("sing-box exited after start: %w: %s", err, strings.TrimSpace(earlyOutput.String()))
		}
		return fmt.Errorf("sing-box exited after start: %s", strings.TrimSpace(earlyOutput.String()))
	case <-time.After(1 * time.Second):
		return nil
	}
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) stopLocked() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	process := m.cmd.Process
	_ = process.Kill()
	done := m.done
	if done == nil {
		done = make(chan error, 1)
		go func() {
			done <- m.cmd.Wait()
		}()
	}
	select {
	case err := <-done:
		m.cmd = nil
		m.done = nil
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				_ = exitErr
				return nil
			}
			return err
		}
		return nil
	case <-time.After(2 * time.Second):
		m.cmd = nil
		m.done = nil
		return nil
	}
}

func (m *Manager) Configure(nodes []convert.Node) ([]convert.Node, error) {
	if !m.cfg.Enabled {
		return nodes, nil
	}
	vlessCount := countVLESS(nodes)
	if vlessCount == 0 {
		return nodes, m.writeConfig(singBoxConfig{})
	}
	availablePorts := m.cfg.PortEnd - m.cfg.PortStart + 1
	if vlessCount > availablePorts {
		return nil, fmt.Errorf("not enough VLESS relay ports: need %d, have %d", vlessCount, availablePorts)
	}
	if m.cfg.PublicHost == "" {
		return nil, fmt.Errorf("VLESS relay public host is required")
	}

	next := make([]convert.Node, 0, len(nodes))
	cfg := singBoxConfig{Log: singBoxLog{Level: "warn"}}
	vlessIndex := 0
	for _, node := range nodes {
		if node.Protocol != "vless" {
			next = append(next, node)
			continue
		}
		if node.Params["uuid"] == "" {
			return nil, fmt.Errorf("vless node %q uuid is required", node.Name)
		}
		port := m.cfg.PortStart + vlessIndex
		inboundTag := fmt.Sprintf("vless-in-%d", vlessIndex)
		outboundTag := fmt.Sprintf("vless-out-%d", vlessIndex)
		cfg.Inbounds = append(cfg.Inbounds, m.socksInbound(inboundTag, port))
		cfg.Outbounds = append(cfg.Outbounds, vlessOutbound(outboundTag, node))
		cfg.Route.Rules = append(cfg.Route.Rules, singBoxRouteRule{Inbound: []string{inboundTag}, Outbound: outboundTag})
		next = append(next, convert.Node{
			Name:     node.Name,
			Protocol: "socks5",
			Server:   m.cfg.PublicHost,
			Port:     port,
			Params: map[string]string{
				"username":       m.cfg.Username,
				"password":       m.cfg.Password,
				"relay_protocol": "vless",
			},
			Tags:   node.Tags,
			Pinned: node.Pinned,
		})
		vlessIndex++
	}
	cfg.Outbounds = append(cfg.Outbounds, singBoxOutbound{Type: "direct", Tag: "direct"})
	if err := m.writeConfig(cfg); err != nil {
		return nil, err
	}
	return next, nil
}

func countVLESS(nodes []convert.Node) int {
	count := 0
	for _, node := range nodes {
		if node.Protocol == "vless" {
			count++
		}
	}
	return count
}

func (m *Manager) socksInbound(tag string, port int) singBoxInbound {
	inbound := singBoxInbound{
		Type:       "socks",
		Tag:        tag,
		Listen:     m.cfg.ListenHost,
		ListenPort: port,
	}
	if m.cfg.Username != "" || m.cfg.Password != "" {
		inbound.Users = []singBoxUser{{Username: m.cfg.Username, Password: m.cfg.Password}}
	}
	return inbound
}

func (m *Manager) writeConfig(cfg singBoxConfig) error {
	if m.cfg.ConfigPath == "" {
		return fmt.Errorf("sing-box config path is required")
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.ConfigPath), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfg.ConfigPath, append(content, '\n'), 0o644)
}

type singBoxConfig struct {
	Log       singBoxLog         `json:"log,omitempty"`
	Inbounds  []singBoxInbound   `json:"inbounds,omitempty"`
	Outbounds []singBoxOutbound  `json:"outbounds,omitempty"`
	Route     singBoxRouteConfig `json:"route,omitempty"`
}

type singBoxLog struct {
	Level string `json:"level,omitempty"`
}

type singBoxInbound struct {
	Type       string        `json:"type"`
	Tag        string        `json:"tag"`
	Listen     string        `json:"listen"`
	ListenPort int           `json:"listen_port"`
	Users      []singBoxUser `json:"users,omitempty"`
}

type singBoxUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type singBoxOutbound struct {
	Type           string            `json:"type"`
	Tag            string            `json:"tag"`
	Network        string            `json:"network,omitempty"`
	Server         string            `json:"server,omitempty"`
	ServerPort     int               `json:"server_port,omitempty"`
	UUID           string            `json:"uuid,omitempty"`
	Flow           string            `json:"flow,omitempty"`
	TLS            *singBoxTLS       `json:"tls,omitempty"`
	Transport      *singBoxTransport `json:"transport,omitempty"`
	PacketEncoding string            `json:"packet_encoding,omitempty"`
}

type singBoxTLS struct {
	Enabled    bool            `json:"enabled"`
	ServerName string          `json:"server_name,omitempty"`
	Insecure   bool            `json:"insecure,omitempty"`
	ALPN       []string        `json:"alpn,omitempty"`
	UTLS       *singBoxUTLS    `json:"utls,omitempty"`
	Reality    *singBoxReality `json:"reality,omitempty"`
}

type singBoxUTLS struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type singBoxReality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty"`
}

type singBoxTransport struct {
	Type        string            `json:"type"`
	Path        string            `json:"path,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
}

type singBoxRouteConfig struct {
	Rules []singBoxRouteRule `json:"rules,omitempty"`
}

type singBoxRouteRule struct {
	Inbound  []string `json:"inbound"`
	Outbound string   `json:"outbound"`
}

func vlessOutbound(tag string, node convert.Node) singBoxOutbound {
	outbound := singBoxOutbound{
		Type:       "vless",
		Tag:        tag,
		Network:    fallbackNetwork(node.Params["network"]),
		Server:     node.Server,
		ServerPort: node.Port,
		UUID:       node.Params["uuid"],
		Flow:       node.Params["flow"],
	}
	if tlsMode := node.Params["tls"]; tlsMode == "tls" || tlsMode == "reality" {
		outbound.TLS = &singBoxTLS{Enabled: true, ServerName: node.Params["sni"], Insecure: node.Params["skip_cert_verify"] == "true"}
		if alpn := splitCSV(node.Params["alpn"]); len(alpn) > 0 {
			outbound.TLS.ALPN = alpn
		}
		if fingerprint := node.Params["client_fingerprint"]; fingerprint != "" {
			outbound.TLS.UTLS = &singBoxUTLS{Enabled: true, Fingerprint: fingerprint}
		}
		if publicKey := node.Params["reality_public_key"]; publicKey != "" {
			outbound.TLS.Reality = &singBoxReality{
				Enabled:   true,
				PublicKey: publicKey,
				ShortID:   node.Params["reality_short_id"],
			}
		}
	}
	switch node.Params["network"] {
	case "ws":
		transport := &singBoxTransport{Type: "ws", Path: node.Params["ws_path"]}
		if host := node.Params["ws_host"]; host != "" {
			transport.Headers = map[string]string{"Host": host}
		}
		outbound.Transport = transport
	case "grpc":
		outbound.Transport = &singBoxTransport{Type: "grpc", ServiceName: node.Params["grpc_service_name"]}
	}
	return outbound
}

func fallbackNetwork(network string) string {
	if network == "" || network == "ws" || network == "grpc" {
		return "tcp"
	}
	return network
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
