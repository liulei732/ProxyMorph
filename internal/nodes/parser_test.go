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

func TestParseRejectsUnsupportedURI(t *testing.T) {
	if _, err := ParseURI("http://example.com"); err == nil {
		t.Fatal("expected unsupported URI error")
	}
}
