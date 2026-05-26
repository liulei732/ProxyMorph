package convert

import (
	"errors"
	"strings"
	"testing"
)

func TestParseClashAndRenderSurge(t *testing.T) {
	input := []byte(`
proxies:
  - name: "HK 1"
    type: ss
    server: hk.example.com
    port: 8388
    cipher: aes-256-gcm
    password: pass
  - name: "Edge"
    type: trojan
    server: edge.example.com
    port: 443
    password: secret
    sni: edge.example.com
proxy-groups:
  - name: "Proxy"
    type: select
    proxies:
      - "HK 1"
rules:
  - DOMAIN-SUFFIX,example.com,Proxy
  - FINAL,Proxy
`)
	doc, err := ParseClash(input)
	if err != nil {
		t.Fatalf("ParseClash returned error: %v", err)
	}
	if len(doc.Nodes) != 2 || len(doc.Rules) != 2 {
		t.Fatalf("unexpected parsed doc: %#v", doc)
	}
	out := RenderSurge6(doc.Nodes, doc.Groups, doc.Rules)
	for _, want := range []string{
		"[Proxy]",
		"HK 1 = ss, hk.example.com, 8388",
		"Edge = trojan, edge.example.com, 443",
		"[Proxy Group]",
		"Proxy = select, HK 1",
		"[Rule]",
		"DOMAIN-SUFFIX,example.com,Proxy",
		"FINAL,Proxy",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestParseClashPreservesVLESS(t *testing.T) {
	input := []byte(`
proxies:
  - name: "VLESS Edge"
    type: vless
    server: edge.example.com
    port: 443
    uuid: f47ac10b-58cc-4372-a567-0e02b2c3d479
    tls: true
    servername: sni.example.com
    flow: xtls-rprx-vision
    network: ws
    ws-opts:
      path: /proxy
rules:
  - FINAL,Proxy
`)
	doc, err := ParseClash(input)
	if err != nil {
		t.Fatalf("ParseClash returned error: %v", err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(doc.Nodes))
	}
	node := doc.Nodes[0]
	if node.Protocol != "vless" || node.Params["uuid"] == "" || node.Params["sni"] != "sni.example.com" {
		t.Fatalf("unexpected vless node: %#v", node)
	}
	if node.Params["network"] != "ws" || node.Params["ws_path"] != "/proxy" {
		t.Fatalf("unexpected vless transport params: %#v", node.Params)
	}
}

func TestRenderSurge6WithOptionsRequiresVLESSRenderer(t *testing.T) {
	_, err := RenderSurge6WithOptions([]Node{{Name: "VLESS", Protocol: "vless", Server: "edge.example.com", Port: 443}}, nil, nil, RenderOptions{})
	if !errors.Is(err, ErrVLESSRendererRequired) {
		t.Fatalf("error = %v, want ErrVLESSRendererRequired", err)
	}
}
