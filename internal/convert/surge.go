package convert

import (
	"fmt"
	"sort"
	"strings"
)

func RenderSurge6(nodes []Node, groups []Group, rules []string) string {
	var b strings.Builder
	b.WriteString("[Proxy]\n")
	for _, node := range nodes {
		b.WriteString(renderNode(node))
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
	return b.String()
}

func renderNode(node Node) string {
	switch node.Protocol {
	case "ss":
		return fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s", node.Name, node.Server, node.Port, node.Params["cipher"], node.Params["password"])
	case "trojan":
		parts := []string{fmt.Sprintf("%s = trojan, %s, %d, password=%s", node.Name, node.Server, node.Port, node.Params["password"])}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		return strings.Join(parts, ", ")
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
		return strings.Join(parts, ", ")
	}
}
