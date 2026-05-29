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

type RenderResult struct {
	Output  string
	Warning string
}

func Surge6SupportedNodes(nodes []Node) ([]Node, []Node) {
	supported := make([]Node, 0, len(nodes))
	unsupported := make([]Node, 0)
	for _, node := range nodes {
		if isSurge6SupportedProtocol(node.Protocol) {
			supported = append(supported, node)
			continue
		}
		unsupported = append(unsupported, node)
	}
	return supported, unsupported
}

func FilterGroupsForNodes(groups []Group, nodes []Node) []Group {
	allowed := make(map[string]struct{}, len(nodes)+2)
	allowed["DIRECT"] = struct{}{}
	allowed["REJECT"] = struct{}{}
	for _, node := range nodes {
		allowed[node.Name] = struct{}{}
	}
	for _, group := range groups {
		allowed[group.Name] = struct{}{}
	}
	filtered := make([]Group, 0, len(groups))
	for _, group := range groups {
		next := Group{Name: group.Name, Type: group.Type, Proxies: make([]string, 0, len(group.Proxies))}
		for _, proxy := range group.Proxies {
			if _, ok := allowed[proxy]; ok {
				next.Proxies = append(next.Proxies, proxy)
			}
		}
		if len(next.Proxies) == 0 {
			next.Proxies = append(next.Proxies, "DIRECT")
		}
		filtered = append(filtered, next)
	}
	return filtered
}

func isSurge6SupportedProtocol(protocol string) bool {
	switch protocol {
	case "vless":
		return false
	default:
		return true
	}
}

func RenderSurge6(nodes []Node, groups []Group, rules []string) string {
	output, _ := RenderSurge6WithOptions(nodes, groups, rules, RenderOptions{
		VLESSRenderer: renderSimpleVLESS,
	})
	return output
}

func RenderSurge6WithOptions(nodes []Node, groups []Group, rules []string, opts RenderOptions) (string, error) {
	result, err := renderSurge6(nodes, groups, rules, SurgeConfig{}, opts)
	if err != nil {
		return result.Output, err
	}
	if result.Warning != "" {
		return result.Output, errors.New(result.Warning)
	}
	return result.Output, nil
}

func RenderSurge6WithConfig(nodes []Node, groups []Group, rules []string, cfg SurgeConfig) (string, error) {
	result, err := RenderSurge6WithWarnings(nodes, groups, rules, cfg)
	if err != nil {
		return result.Output, err
	}
	if result.Warning != "" {
		return result.Output, errors.New(result.Warning)
	}
	return result.Output, nil
}

func RenderSurge6WithWarnings(nodes []Node, groups []Group, rules []string, cfg SurgeConfig) (RenderResult, error) {
	return renderSurge6(nodes, groups, rules, cfg, RenderOptions{VLESSRenderer: renderSimpleVLESS})
}

func renderSurge6(nodes []Node, groups []Group, rules []string, cfg SurgeConfig, opts RenderOptions) (RenderResult, error) {
	var b strings.Builder
	defaultPolicy := "Proxy"
	if header := strings.TrimSpace(cfg.ManagedConfigHeader); header != "" {
		b.WriteString(header)
		b.WriteString("\n\n")
	}
	b.WriteString("[Proxy]\n")
	for _, node := range nodes {
		rendered, err := renderNodeWithOptions(node, opts)
		if err != nil {
			return RenderResult{}, err
		}
		b.WriteString(rendered)
		b.WriteString("\n")
	}
	b.WriteString("\n[Proxy Group]\n")
	customGroups := normalizeLines(cfg.CustomGroups)
	customGroupNames := policyGroupNames(customGroups)
	availablePolicies := builtinPolicyNames()
	for _, node := range nodes {
		availablePolicies[node.Name] = struct{}{}
	}
	if len(groups) == 0 {
		names := make([]string, 0, len(nodes))
		for _, node := range nodes {
			names = append(names, node.Name)
		}
		if _, replaced := customGroupNames["Proxy"]; !replaced {
			availablePolicies["Proxy"] = struct{}{}
			b.WriteString("Proxy = select")
			if len(names) > 0 {
				b.WriteString(", ")
				b.WriteString(strings.Join(names, ", "))
			}
			b.WriteString("\n")
		}
	} else {
		defaultPolicy = groups[0].Name
		for _, group := range groups {
			availablePolicies[group.Name] = struct{}{}
			if _, replaced := customGroupNames[group.Name]; replaced {
				continue
			}
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
	for _, group := range customGroups {
		if name, ok := policyGroupName(group); ok {
			availablePolicies[name] = struct{}{}
		}
		b.WriteString(group)
		b.WriteString("\n")
	}
	b.WriteString("\n[Rule]\n")
	finalPolicy := strings.TrimSpace(cfg.FinalRulePolicy)
	finalPolicyLocked := finalPolicy != ""
	if finalPolicy == "" {
		finalPolicy = defaultPolicy
	}
	mergedRules := mergeRules(rules, cfg.CustomRules, cfg.RuleMergeMode)
	warnings := nodeCompatibilityWarnings(nodes)
	if err := validateRulePolicies(mergedRules, finalPolicy, availablePolicies); err != nil {
		warnings = append(warnings, err.Error())
	}
	for _, rule := range ensureFinalRule(mergedRules, finalPolicy, finalPolicyLocked) {
		b.WriteString(rule)
		b.WriteString("\n")
	}
	return RenderResult{Output: b.String(), Warning: strings.Join(warnings, "\n")}, nil
}

func nodeCompatibilityWarnings(nodes []Node) []string {
	return nil
}

func builtinPolicyNames() map[string]struct{} {
	return map[string]struct{}{
		"DIRECT":          {},
		"REJECT":          {},
		"REJECT-DROP":     {},
		"REJECT-NO-DROP":  {},
		"REJECT-TINYGIF":  {},
		"REJECT-DICT":     {},
		"REJECT-ARRAY":    {},
		"REJECT-200":      {},
		"REJECT-IMG":      {},
		"REJECT-DEFAULT":  {},
		"REJECT-SILENT":   {},
		"REJECT-RESPONSE": {},
	}
}

func policyGroupNames(lines []string) map[string]struct{} {
	names := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		name, ok := policyGroupName(line)
		if ok {
			names[name] = struct{}{}
		}
	}
	return names
}

func policyGroupName(line string) (string, bool) {
	name, _, ok := strings.Cut(line, "=")
	if !ok {
		return "", false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	return name, true
}

func mergeRules(upstreamRules, customRules []string, mode string) []string {
	upstream := stripFinalRules(normalizeLines(upstreamRules))
	custom := stripFinalRules(normalizeLines(customRules))
	switch mode {
	case "upstream_first":
		return append(append([]string{}, upstream...), custom...)
	case "custom_first_dedupe":
		return appendDedupe(custom, upstream)
	case "upstream_first_dedupe":
		return appendDedupe(upstream, custom)
	default:
		return append(append([]string{}, custom...), upstream...)
	}
}

func normalizeLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func stripFinalRules(rules []string) []string {
	result := make([]string, 0, len(rules))
	for _, rule := range rules {
		if isExplicitFinalRule(rule) {
			continue
		}
		result = append(result, rule)
	}
	return result
}

func appendDedupe(primary, secondary []string) []string {
	result := append([]string{}, primary...)
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	for _, rule := range primary {
		seen[ruleKey(rule)] = struct{}{}
	}
	for _, rule := range secondary {
		key := ruleKey(rule)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, rule)
	}
	return result
}

func ruleKey(rule string) string {
	parts := strings.Split(rule, ",")
	if len(parts) < 2 {
		return strings.TrimSpace(rule)
	}
	return strings.ToUpper(strings.TrimSpace(parts[0])) + "," + strings.TrimSpace(parts[1])
}

func validateRulePolicies(rules []string, finalPolicy string, availablePolicies map[string]struct{}) error {
	for _, rule := range rules {
		policy, ok := rulePolicy(rule)
		if !ok || policyExists(policy, availablePolicies) {
			continue
		}
		return missingPolicyError(policy, rule, availablePolicies)
	}
	if finalPolicy != "" && !policyExists(finalPolicy, availablePolicies) {
		return missingPolicyError(finalPolicy, "FINAL,"+finalPolicy, availablePolicies)
	}
	return nil
}

func rulePolicy(rule string) (string, bool) {
	rule = stripRuleComment(rule)
	if rule == "" {
		return "", false
	}
	parts := strings.Split(rule, ",")
	if len(parts) < 2 {
		return "", false
	}
	policyIndex := len(parts) - 1
	kind := strings.ToUpper(strings.TrimSpace(parts[0]))
	if kind == "AND" || kind == "OR" || kind == "NOT" || kind == "SUB-RULE" {
		return "", false
	}
	if kind == "IP-CIDR" || kind == "IP-CIDR6" || kind == "GEOIP" {
		for i := len(parts) - 1; i >= 1; i-- {
			if isBooleanRuleOption(parts[i]) {
				continue
			}
			policyIndex = i
			break
		}
	}
	policy := strings.TrimSpace(parts[policyIndex])
	if policy == "" {
		return "", false
	}
	return policy, true
}

func stripRuleComment(rule string) string {
	rule = strings.TrimSpace(rule)
	if rule == "" || strings.HasPrefix(rule, "#") {
		return ""
	}
	for i := 0; i < len(rule)-1; i++ {
		if rule[i] == '/' && rule[i+1] == '/' && i > 0 && isRuleCommentSpace(rule[i-1]) {
			return strings.TrimSpace(rule[:i])
		}
	}
	return rule
}

func isRuleCommentSpace(value byte) bool {
	return value == ' ' || value == '\t'
}

func isBooleanRuleOption(part string) bool {
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "no-resolve", "extended-matching", "pre-matching":
		return true
	default:
		return false
	}
}

func policyExists(policy string, availablePolicies map[string]struct{}) bool {
	_, ok := availablePolicies[policy]
	return ok
}

func missingPolicyError(policy string, rule string, availablePolicies map[string]struct{}) error {
	names := make([]string, 0, len(availablePolicies))
	for name := range availablePolicies {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Errorf("规则引用了不存在的策略：%s。行内容：%s。请修改规则策略或添加同名策略组。当前可用策略：%s", policy, rule, strings.Join(names, ", "))
}

func ensureFinalRule(rules []string, defaultPolicy string, finalPolicyLocked bool) []string {
	if defaultPolicy == "" {
		defaultPolicy = "Proxy"
	}
	next := make([]string, 0, len(rules)+1)
	finalPolicy := strings.TrimSpace(defaultPolicy)
	for _, rule := range rules {
		if strings.TrimSpace(rule) == "" {
			continue
		}
		if policy, ok := finalLikeRulePolicy(rule); ok {
			if !finalPolicyLocked {
				finalPolicy = policy
			}
			continue
		}
		next = append(next, rule)
	}
	if len(next) == 0 || !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(next[len(next)-1])), "FINAL,") {
		next = append(next, "FINAL,"+finalPolicy)
	}
	return next
}

func isExplicitFinalRule(rule string) bool {
	parts := strings.Split(strings.TrimSpace(rule), ",")
	return len(parts) >= 1 && strings.ToUpper(strings.TrimSpace(parts[0])) == "FINAL"
}

func finalLikeRulePolicy(rule string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(rule), ",")
	if len(parts) < 2 {
		return "", false
	}
	kind := strings.ToUpper(strings.TrimSpace(parts[0]))
	if kind != "FINAL" && kind != "MATCH" {
		return "", false
	}
	policy := strings.TrimSpace(parts[1])
	if policy == "" {
		return "", false
	}
	return policy, true
}

func renderNode(node Node) string {
	rendered, _ := renderNodeWithOptions(node, RenderOptions{VLESSRenderer: renderSimpleVLESS})
	return rendered
}

func renderNodeWithOptions(node Node, opts RenderOptions) (string, error) {
	if raw := strings.TrimSpace(node.Params["surge_raw"]); raw != "" {
		return raw, nil
	}
	switch node.Protocol {
	case "ss":
		return fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s", node.Name, node.Server, node.Port, node.Params["cipher"], node.Params["password"]), nil
	case "socks5":
		parts := []string{fmt.Sprintf("%s = socks5, %s, %d", node.Name, node.Server, node.Port)}
		if username := node.Params["username"]; username != "" {
			parts = append(parts, "username="+username)
		}
		if password := node.Params["password"]; password != "" {
			parts = append(parts, "password="+password)
		}
		return strings.Join(parts, ", "), nil
	case "trojan":
		parts := []string{fmt.Sprintf("%s = trojan, %s, %d, password=%s", node.Name, node.Server, node.Port, node.Params["password"])}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		appendSharedSurgeParams(&parts, node)
		return strings.Join(parts, ", "), nil
	case "vmess":
		parts := []string{fmt.Sprintf("%s = vmess, %s, %d, username=%s", node.Name, node.Server, node.Port, node.Params["uuid"])}
		if tls := node.Params["tls"]; tls != "" && tls != "false" {
			parts = append(parts, "tls=true")
		}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		appendSharedSurgeParams(&parts, node)
		return strings.Join(parts, ", "), nil
	case "vless":
		if opts.VLESSRenderer == nil {
			return "", ErrVLESSRendererRequired
		}
		return opts.VLESSRenderer(node)
	case "anytls":
		parts := []string{fmt.Sprintf("%s = anytls, %s, %d, password=%s", node.Name, node.Server, node.Port, node.Params["password"])}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		appendSharedSurgeParams(&parts, node)
		return strings.Join(parts, ", "), nil
	case "hysteria2":
		parts := []string{fmt.Sprintf("%s = hysteria2, %s, %d, password=%s", node.Name, node.Server, node.Port, node.Params["password"])}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		appendSharedSurgeParams(&parts, node)
		if down := node.Params["down"]; down != "" {
			parts = append(parts, "download-bandwidth="+down)
		}
		if up := node.Params["up"]; up != "" {
			parts = append(parts, "upload-bandwidth="+up)
		}
		if ports := node.Params["ports"]; ports != "" {
			parts = append(parts, "port-hopping="+quoteSurgeValue(ports))
		}
		return strings.Join(parts, ", "), nil
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

func quoteSurgeValue(value string) string {
	if value == "" {
		return `""`
	}
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func appendSharedSurgeParams(parts *[]string, node Node) {
	if node.Params["skip_cert_verify"] == "true" {
		*parts = append(*parts, "skip-cert-verify=true")
	}
	if clientFingerprint := node.Params["client_fingerprint"]; clientFingerprint != "" {
		*parts = append(*parts, "client-fingerprint="+clientFingerprint)
	}
	if node.Params["udp"] == "true" {
		*parts = append(*parts, "udp-relay=true")
	}
	if node.Params["network"] == "ws" {
		*parts = append(*parts, "ws=true")
		if wsPath := node.Params["ws_path"]; wsPath != "" {
			*parts = append(*parts, "ws-path="+wsPath)
		}
		if wsHost := node.Params["ws_host"]; wsHost != "" {
			*parts = append(*parts, "ws-headers=Host:"+wsHost)
		}
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
