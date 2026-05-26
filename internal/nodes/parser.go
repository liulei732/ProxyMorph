package nodes

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/liulei/proxymorph/internal/convert"
)

func ParseURI(raw string) (convert.Node, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return convert.Node{}, err
	}
	switch u.Scheme {
	case "ss":
		return parseSS(u)
	case "trojan":
		return parseTrojan(u)
	case "vless":
		return parseVLESS(u)
	default:
		return convert.Node{}, fmt.Errorf("unsupported proxy URI scheme %q", u.Scheme)
	}
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
	params := map[string]string{"password": u.User.Username()}
	if sni := u.Query().Get("sni"); sni != "" {
		params["sni"] = sni
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
	copyParam(params, "grpc_service_name", query.Get("serviceName"))
	copyParam(params, "reality_public_key", query.Get("pbk"))
	copyParam(params, "reality_short_id", query.Get("sid"))
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
