package nodes

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/liulei/proxymorph/internal/convert"
)

func ParseURI(raw string) (convert.Node, error) {
	raw = strings.TrimSpace(raw)
	if isSurgeSectionHeader(raw) {
		return convert.Node{}, fmt.Errorf("surge section header")
	}
	if strings.Contains(raw, "=") && !strings.Contains(raw, "://") {
		return parseSurgeProxyLine(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return convert.Node{}, err
	}
	switch u.Scheme {
	case "ss":
		return parseSS(u)
	case "trojan":
		return parseTrojan(u)
	case "vmess":
		return parseVMess(u)
	case "vless":
		return parseVLESS(u)
	default:
		return convert.Node{}, fmt.Errorf("unsupported proxy URI scheme %q", u.Scheme)
	}
}

func parseSurgeProxyLine(raw string) (convert.Node, error) {
	namePart, valuePart, ok := strings.Cut(raw, "=")
	if !ok {
		return convert.Node{}, fmt.Errorf("invalid surge proxy line")
	}
	name := unquoteSurgeToken(strings.TrimSpace(namePart))
	fields := splitSurgeFields(valuePart)
	if name == "" || len(fields) < 3 {
		return convert.Node{}, fmt.Errorf("invalid surge proxy line")
	}
	port, err := strconv.Atoi(unquoteSurgeToken(fields[2]))
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid surge proxy port: %w", err)
	}
	params := make(map[string]string)
	params["surge_raw"] = raw
	for _, field := range fields[3:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		addSurgeParam(params, strings.TrimSpace(key), unquoteSurgeToken(strings.TrimSpace(value)))
	}
	return convert.Node{
		Name:     name,
		Protocol: strings.TrimSpace(fields[0]),
		Server:   unquoteSurgeToken(fields[1]),
		Port:     port,
		Params:   params,
		Pinned:   true,
	}, nil
}

func addSurgeParam(params map[string]string, key, value string) {
	if value == "" {
		return
	}
	switch strings.ToLower(key) {
	case "skip-cert-verify":
		params["skip_cert_verify"] = value
	case "ws":
		if strings.EqualFold(value, "true") {
			params["network"] = "ws"
		}
	case "ws-path":
		params["ws_path"] = value
	case "ws-headers":
		params["ws_host"] = strings.TrimPrefix(value, "Host:")
	case "client-fingerprint":
		params["client_fingerprint"] = value
	default:
		params[key] = value
	}
}

func splitSurgeFields(value string) []string {
	fields := make([]string, 0)
	var current strings.Builder
	inQuote := false
	escaped := false
	for _, r := range value {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			current.WriteRune(r)
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			if field := strings.TrimSpace(current.String()); field != "" {
				fields = append(fields, field)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if field := strings.TrimSpace(current.String()); field != "" {
		fields = append(fields, field)
	}
	return fields
}

func unquoteSurgeToken(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		value = strings.TrimPrefix(strings.TrimSuffix(value, `"`), `"`)
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
	}
	return value
}

func isSurgeSectionHeader(raw string) bool {
	return strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]")
}

func parseVMess(u *url.URL) (convert.Node, error) {
	encoded := strings.TrimPrefix(u.String(), "vmess://")
	decoded, err := decodeBase64(encoded)
	if err != nil {
		return convert.Node{}, err
	}
	var payload struct {
		Name    string `json:"ps"`
		Server  string `json:"add"`
		Port    string `json:"port"`
		UUID    string `json:"id"`
		AlterID any    `json:"aid"`
		Cipher  string `json:"scy"`
		Network string `json:"net"`
		Type    string `json:"type"`
		Host    string `json:"host"`
		Path    string `json:"path"`
		TLS     string `json:"tls"`
		SNI     string `json:"sni"`
	}
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		return convert.Node{}, fmt.Errorf("invalid vmess payload: %w", err)
	}
	port, err := strconv.Atoi(payload.Port)
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid vmess port: %w", err)
	}
	params := map[string]string{
		"uuid":       payload.UUID,
		"alter_id":   anyToString(payload.AlterID, "0"),
		"cipher":     fallbackValue(payload.Cipher, "auto"),
		"vmess_type": fallbackValue(payload.Type, "none"),
	}
	copyParam(params, "tls", payload.TLS)
	copyParam(params, "sni", payload.SNI)
	copyParam(params, "network", payload.Network)
	copyParam(params, "ws_path", payload.Path)
	copyParam(params, "ws_host", payload.Host)
	return convert.Node{
		Name:     fallbackName(payload.Name, payload.Server),
		Protocol: "vmess",
		Server:   payload.Server,
		Port:     port,
		Params:   params,
		Pinned:   true,
	}, nil
}

func parseSS(u *url.URL) (convert.Node, error) {
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid ss port: %w", err)
	}
	userInfo, err := decodeBase64(u.User.String())
	if err != nil {
		return convert.Node{}, err
	}
	parts := strings.SplitN(userInfo, ":", 2)
	if len(parts) != 2 {
		return convert.Node{}, fmt.Errorf("invalid ss user info")
	}
	name, _ := url.PathUnescape(u.Fragment)
	return convert.Node{
		Name:     fallbackName(name, host),
		Protocol: "ss",
		Server:   host,
		Port:     port,
		Params: map[string]string{
			"cipher":   parts[0],
			"password": parts[1],
		},
		Pinned: true,
	}, nil
}

func parseTrojan(u *url.URL) (convert.Node, error) {
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid trojan port: %w", err)
	}
	name, _ := url.PathUnescape(u.Fragment)
	query := u.Query()
	params := map[string]string{"password": u.User.Username()}
	copyTrojanSNI(params, query.Get("peer"), query.Get("sni"))
	copyTrojanUDP(params, query.Get("udp"))
	copyParam(params, "network", query.Get("type"))
	copyParam(params, "ws_path", query.Get("path"))
	copyParam(params, "ws_host", query.Get("host"))
	if query.Get("allowInsecure") == "1" || strings.EqualFold(query.Get("allowInsecure"), "true") {
		params["skip_cert_verify"] = "true"
	}
	return convert.Node{
		Name:     fallbackName(name, u.Hostname()),
		Protocol: "trojan",
		Server:   u.Hostname(),
		Port:     port,
		Params:   params,
		Pinned:   true,
	}, nil
}

func copyTrojanSNI(params map[string]string, peer string, sni string) {
	peer = strings.TrimSpace(peer)
	sni = strings.TrimSpace(sni)
	if sni != "" {
		params["sni"] = sni
		if peer != "" && peer != sni {
			params["peer"] = peer
		}
		return
	}
	copyParam(params, "sni", peer)
}

func copyTrojanUDP(params map[string]string, udp string) {
	udp = strings.TrimSpace(udp)
	if udp == "" || udp == "1" || strings.EqualFold(udp, "true") {
		params["udp"] = "true"
	}
}

func parseVLESS(u *url.URL) (convert.Node, error) {
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid vless port: %w", err)
	}
	name, _ := url.PathUnescape(u.Fragment)
	query := u.Query()
	params := map[string]string{"uuid": u.User.Username()}
	if security := query.Get("security"); security == "tls" || security == "reality" {
		params["tls"] = security
	}
	copyParam(params, "sni", query.Get("sni"))
	copyParam(params, "flow", query.Get("flow"))
	copyParam(params, "network", query.Get("type"))
	copyParam(params, "ws_path", query.Get("path"))
	copyParam(params, "ws_host", query.Get("host"))
	copyParam(params, "grpc_service_name", query.Get("serviceName"))
	copyParam(params, "reality_public_key", query.Get("pbk"))
	copyParam(params, "reality_short_id", query.Get("sid"))
	copyParam(params, "client_fingerprint", query.Get("fp"))
	copyParam(params, "alpn", query.Get("alpn"))
	if query.Get("allowInsecure") == "1" || strings.EqualFold(query.Get("allowInsecure"), "true") {
		params["skip_cert_verify"] = "true"
	}
	return convert.Node{
		Name:     fallbackName(name, u.Hostname()),
		Protocol: "vless",
		Server:   u.Hostname(),
		Port:     port,
		Params:   params,
		Pinned:   true,
	}, nil
}

func copyParam(params map[string]string, key, value string) {
	if value != "" {
		params[key] = value
	}
}

func decodeBase64(value string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(value)
	}
	if err != nil {
		return "", fmt.Errorf("invalid base64 user info: %w", err)
	}
	return string(decoded), nil
}

func fallbackName(name, host string) string {
	if name != "" {
		return name
	}
	return host
}

func fallbackValue(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func anyToString(value any, fallback string) string {
	switch v := value.(type) {
	case string:
		if v != "" {
			return v
		}
	case float64:
		return strconv.Itoa(int(v))
	case int:
		return strconv.Itoa(v)
	}
	return fallback
}
