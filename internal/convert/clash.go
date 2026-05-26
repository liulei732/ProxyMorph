package convert

import "gopkg.in/yaml.v3"

type Document struct {
	Nodes  []Node
	Groups []Group
	Rules  []string
}

type Group struct {
	Name    string
	Type    string
	Proxies []string
}

type clashFile struct {
	Proxies []map[string]any `yaml:"proxies"`
	Groups  []struct {
		Name    string   `yaml:"name"`
		Type    string   `yaml:"type"`
		Proxies []string `yaml:"proxies"`
	} `yaml:"proxy-groups"`
	Rules []string `yaml:"rules"`
}

func ParseClash(data []byte) (Document, error) {
	var in clashFile
	if err := yaml.Unmarshal(data, &in); err != nil {
		return Document{}, err
	}
	doc := Document{Rules: in.Rules}
	for _, proxy := range in.Proxies {
		node := Node{
			Name:     stringValue(proxy["name"]),
			Protocol: stringValue(proxy["type"]),
			Server:   stringValue(proxy["server"]),
			Port:     intValue(proxy["port"]),
			Params:   map[string]string{},
		}
		for _, key := range []string{"cipher", "password", "sni", "network", "ws-opts"} {
			if value := stringValue(proxy[key]); value != "" {
				node.Params[key] = value
			}
		}
		doc.Nodes = append(doc.Nodes, node)
	}
	for _, group := range in.Groups {
		doc.Groups = append(doc.Groups, Group{Name: group.Name, Type: group.Type, Proxies: group.Proxies})
	}
	return doc, nil
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
