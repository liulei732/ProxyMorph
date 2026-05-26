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
