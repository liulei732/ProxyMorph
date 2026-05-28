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
    skip-cert-verify: true
    network: ws
    ws-opts:
      path: /trojan
      headers:
        Host: trojan-host.example.com
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
		"skip-cert-verify=true",
		"ws=true",
		"ws-path=/trojan",
		"ws-headers=Host:trojan-host.example.com",
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

func TestRenderTrojanWebSocketTransport(t *testing.T) {
	out := RenderSurge6(
		[]Node{{
			Name:     "Edge",
			Protocol: "trojan",
			Server:   "example.com",
			Port:     2053,
			Params: map[string]string{
				"password": "secret",
				"sni":      "sni.example.com",
				"network":  "ws",
				"ws_path":  "/video",
				"ws_host":  "host.example.com",
			},
		}},
		nil,
		nil,
	)
	for _, want := range []string{
		"Edge = trojan, example.com, 2053",
		"password=secret",
		"sni=sni.example.com",
		"ws=true",
		"ws-path=/video",
		"ws-headers=Host:host.example.com",
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
    skip-cert-verify: true
    servername: sni.example.com
    flow: xtls-rprx-vision
    client-fingerprint: chrome
    reality-opts:
      public-key: public-key-value
      short-id: short-id-value
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
	if node.Params["tls"] != "reality" {
		t.Fatalf("tls = %q, want reality when reality-opts are present", node.Params["tls"])
	}
	if node.Params["client_fingerprint"] != "chrome" || node.Params["reality_public_key"] != "public-key-value" || node.Params["reality_short_id"] != "short-id-value" {
		t.Fatalf("unexpected vless reality params: %#v", node.Params)
	}
}

func TestParseClashAndRenderVMessWebSocketTLS(t *testing.T) {
	input := []byte(`
proxies:
  - name: "VMess WS"
    type: vmess
    server: vmess.example.com
    port: 443
    uuid: f47ac10b-58cc-4372-a567-0e02b2c3d479
    tls: true
    skip-cert-verify: true
    servername: sni.example.com
    network: ws
    ws-opts:
      path: /proxy
      headers:
        Host: host.example.com
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
	out := RenderSurge6(doc.Nodes, doc.Groups, doc.Rules)
	for _, want := range []string{
		"VMess WS = vmess, vmess.example.com, 443",
		"username=f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"tls=true",
		"sni=sni.example.com",
		"skip-cert-verify=true",
		"ws=true",
		"ws-path=/proxy",
		"ws-headers=Host:host.example.com",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSurge6WithOptionsRequiresVLESSRenderer(t *testing.T) {
	_, err := RenderSurge6WithOptions([]Node{{Name: "VLESS", Protocol: "vless", Server: "edge.example.com", Port: 443}}, nil, nil, RenderOptions{})
	if !errors.Is(err, ErrVLESSRendererRequired) {
		t.Fatalf("error = %v, want ErrVLESSRendererRequired", err)
	}
}

func TestSurge6SupportedNodesFiltersVLESS(t *testing.T) {
	nodes := []Node{
		{Name: "Remote", Protocol: "ss"},
		{Name: "Edge", Protocol: "vless"},
		{Name: "Legacy", Protocol: "vmess"},
		{Name: "Pinned", Protocol: "trojan"},
	}
	supported, unsupported := Surge6SupportedNodes(nodes)
	if len(supported) != 3 || supported[0].Name != "Remote" || supported[1].Name != "Legacy" || supported[2].Name != "Pinned" {
		t.Fatalf("unexpected supported nodes: %#v", supported)
	}
	if len(unsupported) != 1 || unsupported[0].Name != "Edge" {
		t.Fatalf("unexpected unsupported nodes: %#v", unsupported)
	}
}

func TestFilterGroupsForNodesRemovesUnsupportedProxyNames(t *testing.T) {
	groups := []Group{{Name: "Proxy", Type: "select", Proxies: []string{"Remote", "Edge", "Pinned"}}}
	nodes := []Node{{Name: "Remote", Protocol: "ss"}, {Name: "Pinned", Protocol: "trojan"}}
	filtered := FilterGroupsForNodes(groups, nodes)
	if len(filtered) != 1 {
		t.Fatalf("groups = %d, want 1", len(filtered))
	}
	got := strings.Join(filtered[0].Proxies, ",")
	if got != "Remote,Pinned" {
		t.Fatalf("proxies = %q, want Remote,Pinned", got)
	}
}

func TestRenderSurge6AppendsFinalRuleWhenMissing(t *testing.T) {
	out := RenderSurge6(
		[]Node{{Name: "Remote", Protocol: "ss", Server: "remote.example", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pass"}}},
		nil,
		nil,
	)
	if !strings.Contains(out, "\n[Rule]\nFINAL,Proxy\n") {
		t.Fatalf("output should end rules with FINAL,Proxy:\n%s", out)
	}
}

func TestRenderSurge6UsesFirstGroupForDefaultFinalRule(t *testing.T) {
	out := RenderSurge6(
		[]Node{{Name: "Remote", Protocol: "ss", Server: "remote.example", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pass"}}},
		[]Group{{Name: "Auto", Type: "select", Proxies: []string{"Remote"}}},
		[]string{"DOMAIN-SUFFIX,example.com,Auto"},
	)
	if !strings.Contains(out, "DOMAIN-SUFFIX,example.com,Auto\nFINAL,Auto\n") {
		t.Fatalf("output should append FINAL using first group:\n%s", out)
	}
}

func TestRenderSurge6UsesConfiguredFinalRulePolicy(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		nil,
		[]string{"DOMAIN,upstream.example,Proxy"},
		SurgeConfig{FinalRulePolicy: "DIRECT"},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	if !strings.Contains(out, "DOMAIN,upstream.example,Proxy\nFINAL,DIRECT\n") {
		t.Fatalf("output should append configured FINAL policy:\n%s", out)
	}
	if strings.Contains(out, "FINAL,Proxy") {
		t.Fatalf("output should not keep default FINAL policy when configured:\n%s", out)
	}
}

func TestRenderSurge6WithCustomRulesCustomFirst(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		nil,
		[]string{"DOMAIN,upstream.example,Proxy"},
		SurgeConfig{CustomRules: []string{"DOMAIN,custom.example,DIRECT"}, RuleMergeMode: "custom_first"},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	assertOrder(t, out, "DOMAIN,custom.example,DIRECT", "DOMAIN,upstream.example,Proxy", "FINAL,Proxy")
}

func TestRenderSurge6WithCustomRulesUpstreamFirst(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		nil,
		[]string{"DOMAIN,upstream.example,Proxy"},
		SurgeConfig{CustomRules: []string{"DOMAIN,custom.example,DIRECT"}, RuleMergeMode: "upstream_first"},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	assertOrder(t, out, "DOMAIN,upstream.example,Proxy", "DOMAIN,custom.example,DIRECT", "FINAL,Proxy")
}

func TestRenderSurge6WithCustomRulesDedupeCustomFirst(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		nil,
		[]string{"DOMAIN,dup.example,Proxy", "DOMAIN,upstream.example,Proxy"},
		SurgeConfig{CustomRules: []string{"DOMAIN,dup.example,DIRECT", "DOMAIN,custom.example,DIRECT"}, RuleMergeMode: "custom_first_dedupe"},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	assertOrder(t, out, "DOMAIN,dup.example,DIRECT", "DOMAIN,custom.example,DIRECT", "DOMAIN,upstream.example,Proxy", "FINAL,Proxy")
	if strings.Contains(out, "DOMAIN,dup.example,Proxy") {
		t.Fatalf("output should remove duplicate upstream rule:\n%s", out)
	}
}

func TestRenderSurge6WithCustomRulesDedupeUpstreamFirst(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		nil,
		[]string{"DOMAIN,dup.example,Proxy", "DOMAIN,upstream.example,Proxy"},
		SurgeConfig{CustomRules: []string{"DOMAIN,dup.example,DIRECT", "DOMAIN,custom.example,DIRECT"}, RuleMergeMode: "upstream_first_dedupe"},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	assertOrder(t, out, "DOMAIN,dup.example,Proxy", "DOMAIN,upstream.example,Proxy", "DOMAIN,custom.example,DIRECT", "FINAL,Proxy")
	if strings.Contains(out, "DOMAIN,dup.example,DIRECT") {
		t.Fatalf("output should remove duplicate custom rule:\n%s", out)
	}
}

func TestRenderSurge6WithCustomGroupsAndManagedConfig(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		[]Group{{Name: "Auto", Type: "select", Proxies: []string{"Edge"}}},
		nil,
		SurgeConfig{
			CustomGroups:        []string{"Manual = select, Auto, DIRECT"},
			ManagedConfigHeader: "#!MANAGED-CONFIG http://localhost:8080/sub/token?name=Main interval=86400 strict=false",
		},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	if !strings.HasPrefix(out, "#!MANAGED-CONFIG http://localhost:8080/sub/token?name=Main interval=86400 strict=false\n") {
		t.Fatalf("managed config header should be first line:\n%s", out)
	}
	assertOrder(t, out, "Auto = select, Edge", "Manual = select, Auto, DIRECT", "FINAL,Auto")
}

func TestRenderSurge6CustomGroupReplacesSameNamedGeneratedGroup(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{
			{Name: "美国A-线路1 | TCP", Protocol: "ss", Server: "a.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}},
			{Name: "美国A-线路2+|+TCP", Protocol: "ss", Server: "b.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}},
		},
		nil,
		nil,
		SurgeConfig{CustomGroups: []string{`Proxy = smart, "美国A-线路1 | TCP", 美国A-线路2+|+TCP, include-all-proxies=1`}},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	if strings.Contains(out, "Proxy = select") {
		t.Fatalf("custom Proxy group should replace generated Proxy group, got:\n%s", out)
	}
	if count := strings.Count(out, "Proxy = smart"); count != 1 {
		t.Fatalf("expected exactly one custom Proxy smart group, got %d:\n%s", count, out)
	}
	assertOrder(t, out, `Proxy = smart, "美国A-线路1 | TCP", 美国A-线路2+|+TCP, include-all-proxies=1`, "FINAL,Proxy")
}

func TestRenderSurge6CustomGroupReplacesSameNamedUpstreamGroup(t *testing.T) {
	out, err := RenderSurge6WithConfig(
		[]Node{{Name: "Edge", Protocol: "ss", Server: "example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"}}},
		[]Group{{Name: "Proxy", Type: "select", Proxies: []string{"Edge"}}},
		nil,
		SurgeConfig{CustomGroups: []string{"Proxy = smart, Edge, include-all-proxies=1"}},
	)
	if err != nil {
		t.Fatalf("RenderSurge6WithConfig returned error: %v", err)
	}
	if strings.Contains(out, "Proxy = select") {
		t.Fatalf("custom Proxy group should replace upstream Proxy group, got:\n%s", out)
	}
	if count := strings.Count(out, "Proxy = smart"); count != 1 {
		t.Fatalf("expected exactly one custom Proxy smart group, got %d:\n%s", count, out)
	}
	assertOrder(t, out, "Proxy = smart, Edge, include-all-proxies=1", "FINAL,Proxy")
}

func assertOrder(t *testing.T, text string, values ...string) {
	t.Helper()
	last := -1
	for _, value := range values {
		idx := strings.Index(text, value)
		if idx == -1 {
			t.Fatalf("output missing %q:\n%s", value, text)
		}
		if idx < last {
			t.Fatalf("%q should appear after previous values:\n%s", value, text)
		}
		last = idx
	}
}
