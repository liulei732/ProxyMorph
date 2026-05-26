package convert

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrVLESSRendererRequired = errors.New("vless renderer required")

type RenderOptions struct {
	VLESSRenderer func(Node) (string, error)
}

func RenderSurge6(nodes []Node, groups []Group, rules []string) string {
	output, _ := RenderSurge6WithOptions(nodes, groups, rules, RenderOptions{
		VLESSRenderer: renderSimpleVLESS,
	})
	return output
}

func RenderSurge6WithOptions(nodes []Node, groups []Group, rules []string, opts RenderOptions) (string, error) {
	var b strings.Builder
	b.WriteString("[Proxy]\n")
	for _, node := range nodes {
		rendered, err := renderNodeWithOptions(node, opts)
		if err != nil {
			return "", err
		}
		b.WriteString(rendered)
		b.WriteString("\n")
	}
	b.WriteString("\n[Proxy Group]\n")
	if len(groups) == 0 {
		names := make([]string, 0, len(nodes))
		for _, node := range nodes {
			names = append(names, node.Name)
		}
		b.WriteString("Proxy = select")
		if len(names) > 0 {
			b.WriteString(", ")
			b.WriteString(strings.Join(names, ", "))
		}
		b.WriteString("\n")
	} else {
		for _, group := range groups {
			b.WriteString(group.Name)
			b.WriteString(" = ")
			b.WriteString(group.Type)
			if len(group.Proxies) > 0 {
				b.WriteString(", ")
				b.WriteString(strings.Join(group.Proxies, ", "))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n[Rule]\n")
	for _, rule := range rules {
		b.WriteString(rule)
		b.WriteString("\n")
	}
	return b.String(), nil
}

func renderNode(node Node) string {
	rendered, _ := renderNodeWithOptions(node, RenderOptions{VLESSRenderer: renderSimpleVLESS})
	return rendered
}

func renderNodeWithOptions(node Node, opts RenderOptions) (string, error) {
	switch node.Protocol {
	case "ss":
		return fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s", node.Name, node.Server, node.Port, node.Params["cipher"], node.Params["password"]), nil
	case "trojan":
		parts := []string{fmt.Sprintf("%s = trojan, %s, %d, password=%s", node.Name, node.Server, node.Port, node.Params["password"])}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		return strings.Join(parts, ", "), nil
	case "vless":
		if opts.VLESSRenderer == nil {
			return "", ErrVLESSRendererRequired
		}
		return opts.VLESSRenderer(node)
	default:
		keys := make([]string, 0, len(node.Params))
		for key := range node.Params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := []string{fmt.Sprintf("%s = %s, %s, %d", node.Name, node.Protocol, node.Server, node.Port)}
		for _, key := range keys {
			parts = append(parts, key+"="+node.Params[key])
		}
		return strings.Join(parts, ", "), nil
	}
}

func renderSimpleVLESS(node Node) (string, error) {
	parts := []string{
		fmt.Sprintf("%s = vless, %s, %d, username=%s", node.Name, node.Server, node.Port, node.Params["uuid"]),
	}
	if tls := node.Params["tls"]; tls != "" {
		parts = append(parts, "tls=true")
	}
	if sni := node.Params["sni"]; sni != "" {
		parts = append(parts, "sni="+sni)
	}
	if network := node.Params["network"]; network != "" {
		parts = append(parts, "network="+network)
	}
	if wsPath := node.Params["ws_path"]; wsPath != "" {
		parts = append(parts, "ws-path="+wsPath)
	}
	return strings.Join(parts, ", "), nil
}
