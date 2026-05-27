package nodes

import "testing"

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

func TestParseRejectsUnsupportedURI(t *testing.T) {
	if _, err := ParseURI("http://example.com"); err == nil {
		t.Fatal("expected unsupported URI error")
	}
}
