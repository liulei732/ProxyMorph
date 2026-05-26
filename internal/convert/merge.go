package convert

import (
	"fmt"
	"sort"
	"strings"
)

func MergeNodes(remote, pinned []Node, opts MergeOptions) []Node {
	var ordered []Node
	switch opts.Mode {
	case "before_remote":
		ordered = append(append([]Node{}, pinned...), remote...)
	default:
		ordered = append(append([]Node{}, remote...), pinned...)
	}

	seenKeys := map[string]bool{}
	nameCounts := map[string]int{}
	result := make([]Node, 0, len(ordered))
	for _, node := range ordered {
		key := nodeKey(node)
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true
		nameCounts[node.Name]++
		if nameCounts[node.Name] > 1 {
			node.Name = fmt.Sprintf("%s %d", node.Name, nameCounts[node.Name])
		}
		result = append(result, node)
	}
	if opts.Mode == "sort_order" {
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
	}
	return result
}

func nodeKey(node Node) string {
	var parts []string
	for k, v := range node.Params {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join([]string{
		node.Protocol,
		node.Server,
		fmt.Sprint(node.Port),
		strings.Join(parts, "&"),
	}, "|")
}
