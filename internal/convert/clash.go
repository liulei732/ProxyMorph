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
		if node.Protocol == "vless" {
			copyClashParam(node.Params, "uuid", proxy["uuid"])
			copyClashParam(node.Params, "tls", proxy["tls"])
			copyClashParam(node.Params, "sni", proxy["servername"])
			copyClashParam(node.Params, "flow", proxy["flow"])
			copyClashParam(node.Params, "network", proxy["network"])
			if wsOpts, ok := proxy["ws-opts"].(map[string]any); ok {
				copyClashParam(node.Params, "ws_path", wsOpts["path"])
			}
			if grpcOpts, ok := proxy["grpc-opts"].(map[string]any); ok {
				copyClashParam(node.Params, "grpc_service_name", grpcOpts["grpc-service-name"])
			}
		}
		doc.Nodes = append(doc.Nodes, node)
	}
	for _, group := range in.Groups {
		doc.Groups = append(doc.Groups, Group{Name: group.Name, Type: group.Type, Proxies: group.Proxies})
	}
	return doc, nil
}

func copyClashParam(params map[string]string, key string, value any) {
	switch v := value.(type) {
	case string:
		if v != "" {
			params[key] = v
		}
	case bool:
		if v {
			params[key] = "tls"
		}
	}
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
