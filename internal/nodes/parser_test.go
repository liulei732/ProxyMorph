package nodes

import (
	"strings"
	"testing"

	"github.com/liulei/proxymorph/internal/convert"
)

func TestParseShadowsocksURI(t *testing.T) {
	node, err := ParseURI("ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388#Home")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "Home" || node.Protocol != "ss" || node.Server != "example.com" || node.Port != 8388 {
		t.Fatalf("unexpected node: %#v", node)
	}
	if node.Params["cipher"] != "aes-256-gcm" || node.Params["password"] != "password" {
		t.Fatalf("unexpected params: %#v", node.Params)
	}
}

func TestParseTrojanURI(t *testing.T) {
	node, err := ParseURI("trojan://secret@example.com:443?sni=edge.example.com#Edge")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "Edge" || node.Protocol != "trojan" || node.Params["password"] != "secret" || node.Params["sni"] != "edge.example.com" {
		t.Fatalf("unexpected node: %#v", node)
	}
}

func TestParseTrojanURIWithWebSocketTransport(t *testing.T) {
	node, err := ParseURI("trojan://secret@example.com:2053?allowInsecure=0&peer=peer.example.com&sni=sni.example.com&type=ws&path=%2Fvideo&host=host.example.com#Edge")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	for key, want := range map[string]string{
		"password": "secret",
		"sni":      "sni.example.com",
		"peer":     "peer.example.com",
		"udp":      "true",
		"network":  "ws",
		"ws_path":  "/video",
		"ws_host":  "host.example.com",
	} {
		if node.Params[key] != want {
			t.Fatalf("param %s = %q, want %q; params=%#v", key, node.Params[key], want, node.Params)
		}
	}
	if node.Params["skip_cert_verify"] != "" {
		t.Fatalf("skip_cert_verify = %q, want empty", node.Params["skip_cert_verify"])
	}
}

func TestParseTrojanURIPreservesSNIWhenPeerIsPresent(t *testing.T) {
	node, err := ParseURI("trojan://e87a39e7-fb6e-4c13-86f0-90d0ccf34972@resolution1.private.berry-is-sweet.com:2053?allowInsecure=0&peer=berrycdn-3.com&sni=17800261472322.berrycdn-3.com&type=ws&path=%2Fvideotahyjhghmuaawe&host=j1.berrycdn-3.com#%E8%B6%8A%E5%8D%97-CMI%E4%B8%93%E7%BA%BF1")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "越南-CMI专线1" || node.Protocol != "trojan" {
		t.Fatalf("unexpected node: %#v", node)
	}
	if node.Params["sni"] != "17800261472322.berrycdn-3.com" {
		t.Fatalf("sni = %q, want URI sni value; params=%#v", node.Params["sni"], node.Params)
	}
	if node.Params["peer"] != "berrycdn-3.com" {
		t.Fatalf("peer = %q, want original peer preserved; params=%#v", node.Params["peer"], node.Params)
	}
}

func TestParseTrojanURIRendersURIValueAsSurgeSNI(t *testing.T) {
	node, err := ParseURI("trojan://e87a39e7-fb6e-4c13-86f0-90d0ccf34972@resolution1.private.berry-is-sweet.com:2053?allowInsecure=0&peer=berrycdn-3.com&sni=17800261472322.berrycdn-3.com&type=ws&path=%2Fvideotahyjhghmuaawe&host=j1.berrycdn-3.com#%E8%B6%8A%E5%8D%97-CMI%E4%B8%93%E7%BA%BF1")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	out := convert.RenderSurge6([]convert.Node{node}, nil, nil)
	for _, want := range []string{
		"越南-CMI专线1 = trojan, resolution1.private.berry-is-sweet.com, 2053",
		"password=e87a39e7-fb6e-4c13-86f0-90d0ccf34972",
		"sni=17800261472322.berrycdn-3.com",
		"udp-relay=true",
		"ws=true",
		"ws-path=/videotahyjhghmuaawe",
		"ws-headers=Host:j1.berrycdn-3.com",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "sni=berrycdn-3.com") || strings.Contains(out, "peer=") {
		t.Fatalf("output leaked peer as Surge sni:\n%s", out)
	}
}

func TestParseTrojanURIUsesPeerAsSNIWhenSNIIsMissing(t *testing.T) {
	node, err := ParseURI("trojan://secret@example.com:443?peer=peer.example.com#Edge")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Params["sni"] != "peer.example.com" {
		t.Fatalf("sni = %q, want peer fallback; params=%#v", node.Params["sni"], node.Params)
	}
}

func TestParseVLESSURI(t *testing.T) {
	node, err := ParseURI("vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@example.com:443?security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com&alpn=h2,http/1.1&allowInsecure=1#Edge")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "Edge" || node.Protocol != "vless" || node.Server != "example.com" || node.Port != 443 {
		t.Fatalf("unexpected node: %#v", node)
	}
	if node.Params["uuid"] != "f47ac10b-58cc-4372-a567-0e02b2c3d479" || node.Params["tls"] != "tls" || node.Params["sni"] != "edge.example.com" {
		t.Fatalf("unexpected security params: %#v", node.Params)
	}
	if node.Params["network"] != "ws" || node.Params["ws_path"] != "/proxy" || node.Params["ws_host"] != "cdn.example.com" {
		t.Fatalf("unexpected transport params: %#v", node.Params)
	}
	if node.Params["alpn"] != "h2,http/1.1" || node.Params["skip_cert_verify"] != "true" {
		t.Fatalf("unexpected tls params: %#v", node.Params)
	}
}

func TestParseVMessURI(t *testing.T) {
	node, err := ParseURI("vmess://eyJwcyI6IkVkZ2UiLCJhZGQiOiJ2bWVzcy5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6ImY0N2FjMTBiLTU4Y2MtNDM3Mi1hNTY3LTBlMDJiMmMzZDQ3OSIsInRscyI6InRscyIsInNuaSI6InNuaS5leGFtcGxlLmNvbSIsIm5ldCI6IndzIiwicGF0aCI6Ii9wcm94eSIsImhvc3QiOiJob3N0LmV4YW1wbGUuY29tIn0=")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "Edge" || node.Protocol != "vmess" || node.Server != "vmess.example.com" || node.Port != 443 {
		t.Fatalf("unexpected node: %#v", node)
	}
	for key, want := range map[string]string{
		"uuid":       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"tls":        "tls",
		"sni":        "sni.example.com",
		"network":    "ws",
		"ws_path":    "/proxy",
		"ws_host":    "host.example.com",
		"cipher":     "auto",
		"alter_id":   "0",
		"vmess_type": "none",
	} {
		if node.Params[key] != want {
			t.Fatalf("param %s = %q, want %q; params=%#v", key, node.Params[key], want, node.Params)
		}
	}
}

func TestParseSurgeProxyLine(t *testing.T) {
	line := "香港 03 AnyTLS = anytls, at03-hlzp2o.fork2026.com, 18611, password=9a389c5c-e2b7-3516-8871-22ee7c4e1b7d, sni=www.baidu.com, skip-cert-verify=true, client-fingerprint=firefox, tfo=true, udp-relay=true"
	node, err := ParseURI(line)
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "香港 03 AnyTLS" || node.Protocol != "anytls" || node.Server != "at03-hlzp2o.fork2026.com" || node.Port != 18611 || !node.Pinned {
		t.Fatalf("unexpected node: %#v", node)
	}
	for key, want := range map[string]string{
		"password":           "9a389c5c-e2b7-3516-8871-22ee7c4e1b7d",
		"sni":                "www.baidu.com",
		"skip_cert_verify":   "true",
		"client_fingerprint": "firefox",
		"tfo":                "true",
		"udp-relay":          "true",
		"surge_raw":          line,
	} {
		if node.Params[key] != want {
			t.Fatalf("param %s = %q, want %q; params=%#v", key, node.Params[key], want, node.Params)
		}
	}
}

func TestParseSurgeProxyLineWithQuotedCommasAndWebSocket(t *testing.T) {
	node, err := ParseURI(`"HK, Edge" = trojan, edge.example.com, 2053, password=secret, sni=sni.example.com, skip-cert-verify=true, ws=true, ws-path=/video, ws-headers=Host:host.example.com`)
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "HK, Edge" || node.Protocol != "trojan" || node.Server != "edge.example.com" || node.Port != 2053 {
		t.Fatalf("unexpected node: %#v", node)
	}
	for key, want := range map[string]string{
		"password":         "secret",
		"sni":              "sni.example.com",
		"skip_cert_verify": "true",
		"network":          "ws",
		"ws_path":          "/video",
		"ws_host":          "host.example.com",
	} {
		if node.Params[key] != want {
			t.Fatalf("param %s = %q, want %q; params=%#v", key, node.Params[key], want, node.Params)
		}
	}
}

func TestParseRejectsSurgeSectionHeader(t *testing.T) {
	if _, err := ParseURI("[Proxy]"); err == nil {
		t.Fatal("expected section header to be skipped")
	}
}

func TestParseRejectsUnsupportedURI(t *testing.T) {
	if _, err := ParseURI("http://example.com"); err == nil {
		t.Fatal("expected unsupported URI error")
	}
}
