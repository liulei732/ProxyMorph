package convert

import (
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
