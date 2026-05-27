package subscription

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/liulei/proxymorph/internal/config"
	"github.com/liulei/proxymorph/internal/convert"
	"github.com/liulei/proxymorph/internal/nodes"
	"github.com/liulei/proxymorph/internal/singbox"
	"github.com/liulei/proxymorph/internal/storage"
	"github.com/liulei/proxymorph/internal/tasks"
)

type Service struct {
	db          *storage.DB
	client      *http.Client
	nodes       *nodes.Service
	tasks       *tasks.Service
	vlessRelay  *singbox.Manager
	relayConfig config.VLESSRelayConfig
}

func NewService(db *storage.DB, client *http.Client) *Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &Service{db: db, client: client, nodes: nodes.NewService(db), tasks: tasks.NewService(db)}
}

func (s *Service) SetVLESSRelay(cfg config.VLESSRelayConfig) {
	s.relayConfig = cfg
	if cfg.Enabled {
		s.vlessRelay = singbox.NewManager(cfg)
		return
	}
	s.vlessRelay = nil
}

func (s *Service) GenerateByTaskID(taskID int64) (string, error) {
	return s.generateByTaskID(taskID, "")
}

func (s *Service) generateByTaskID(taskID int64, relayHost string) (string, error) {
	task, err := s.loadTask(taskID)
	if err != nil {
		log.Printf("subscription task=%d stage=load_task status=error error=%q", taskID, err)
		return "", err
	}
	log.Printf("subscription task=%d stage=load_task status=ok source=%q enabled=%t", task.ID, safeSourceLabel(task.SourceURL), task.Enabled)
	if !task.Enabled {
		err := fmt.Errorf("task is disabled")
		log.Printf("subscription task=%d stage=validate status=error error=%q", task.ID, err)
		return "", err
	}
	startedAt := time.Now()
	log.Printf("subscription task=%d stage=fetch status=start source=%q", task.ID, safeSourceLabel(task.SourceURL))
	content, err := s.fetch(task.SourceURL)
	if err != nil {
		log.Printf("subscription task=%d stage=fetch status=error duration_ms=%d error=%q", task.ID, time.Since(startedAt).Milliseconds(), err)
		return s.cachedOrError(task.ID, err)
	}
	log.Printf("subscription task=%d stage=fetch status=ok duration_ms=%d bytes=%d", task.ID, time.Since(startedAt).Milliseconds(), len(content))
	parseStartedAt := time.Now()
	doc, parseInfo, err := parseSubscription(content)
	if err != nil {
		log.Printf("subscription task=%d stage=parse status=error duration_ms=%d error=%q", task.ID, time.Since(parseStartedAt).Milliseconds(), err)
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	log.Printf("subscription task=%d stage=parse status=ok duration_ms=%d format=%s nodes=%d groups=%d rules=%d skipped=%d", task.ID, time.Since(parseStartedAt).Milliseconds(), parseInfo.Format, len(doc.Nodes), len(doc.Groups), len(doc.Rules), parseInfo.Skipped)
	if parseInfo.SkipSummary != "" {
		log.Printf("subscription task=%d stage=parse skipped_summary=%q", task.ID, parseInfo.SkipSummary)
	}
	pinned, err := s.effectivePinnedNodes(task)
	if err != nil {
		log.Printf("subscription task=%d stage=pinned status=error error=%q", task.ID, err)
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	log.Printf("subscription task=%d stage=pinned status=ok nodes=%d merge_default=%t", task.ID, len(pinned), task.MergeDefaultPinnedNodes)
	doc.Nodes = convert.MergeNodes(doc.Nodes, pinned, convert.MergeOptions{Mode: task.PinnedNodeOrderMode})
	if s.vlessRelayEnabled(task) {
		relayStartedAt := time.Now()
		s.configureRelayHost(relayHost)
		doc.Nodes, err = s.vlessRelay.Configure(doc.Nodes)
		if err != nil {
			log.Printf("subscription task=%d stage=vless_relay status=error duration_ms=%d error=%q", task.ID, time.Since(relayStartedAt).Milliseconds(), err)
			s.recordRun(task.ID, "error", err.Error())
			return "", err
		}
		if err := s.vlessRelay.Start(); err != nil {
			log.Printf("subscription task=%d stage=vless_relay_start status=error duration_ms=%d error=%q", task.ID, time.Since(relayStartedAt).Milliseconds(), err)
			s.recordRun(task.ID, "error", err.Error())
			return "", err
		}
		log.Printf("subscription task=%d stage=vless_relay status=ok duration_ms=%d public_host=%q ports=%d-%d", task.ID, time.Since(relayStartedAt).Milliseconds(), s.relayConfig.PublicHost, s.relayConfig.PortStart, s.relayConfig.PortEnd)
	}
	var unsupported []convert.Node
	doc.Nodes, unsupported = convert.Surge6SupportedNodes(doc.Nodes)
	if len(unsupported) > 0 {
		log.Printf("subscription task=%d stage=render unsupported_nodes=%d summary=%q", task.ID, len(unsupported), unsupportedNodeSummary(unsupported))
	}
	if len(doc.Nodes) == 0 {
		err := fmt.Errorf("subscription contains no Surge 6 compatible proxy nodes; unsupported nodes: %s", unsupportedNodeSummary(unsupported))
		log.Printf("subscription task=%d stage=render status=error error=%q", task.ID, err)
		s.recordRun(task.ID, "error", err.Error())
		return "", err
	}
	if len(doc.Groups) == 0 {
		doc.Groups = []convert.Group{{Name: "Proxy", Type: "select", Proxies: nodeNames(doc.Nodes)}}
	} else {
		doc.Groups = convert.FilterGroupsForNodes(doc.Groups, doc.Nodes)
	}
	surgeConfig, err := s.surgeConfig(task, relayHost)
	if err != nil {
		log.Printf("subscription task=%d stage=surge_config status=error error=%q", task.ID, err)
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	output, err := convert.RenderSurge6WithConfig(doc.Nodes, doc.Groups, doc.Rules, surgeConfig)
	if err != nil {
		log.Printf("subscription task=%d stage=render status=error error=%q", task.ID, err)
		s.recordRun(task.ID, "error", err.Error())
		return "", err
	}
	if err := s.storeCache(task.ID, output); err != nil {
		log.Printf("subscription task=%d stage=cache status=error error=%q", task.ID, err)
		return "", err
	}
	s.recordRun(task.ID, "success", "")
	log.Printf("subscription task=%d stage=render status=ok nodes=%d groups=%d output_bytes=%d", task.ID, len(doc.Nodes), len(doc.Groups), len(output))
	return output, nil
}

func (s *Service) vlessRelayEnabled(task storage.ConversionTask) bool {
	if s.vlessRelay == nil {
		return false
	}
	switch normalizeVLESSRelayMode(task.VLESSRelayMode) {
	case "enabled":
		return true
	case "disabled":
		return false
	}
	enabled, err := s.tasks.VLESSRelayEnabled()
	if err != nil {
		log.Printf("subscription stage=vless_relay_setting status=error error=%q", err)
		return false
	}
	return enabled
}

func (s *Service) configureRelayHost(requestHost string) {
	host := relayPublicHost(s.relayConfig.PublicHost, requestHost)
	if host == s.relayConfig.PublicHost {
		return
	}
	s.relayConfig.PublicHost = host
	s.vlessRelay = singbox.NewManager(s.relayConfig)
}

func (s *Service) Close() error {
	if s.vlessRelay == nil {
		return nil
	}
	return s.vlessRelay.Stop()
}

func (s *Service) GenerateByToken(token string) (string, error) {
	return s.GenerateByTokenWithRelayHost(token, "")
}

func (s *Service) GenerateByTokenWithRelayHost(token, relayHost string) (string, error) {
	var taskID int64
	err := s.db.SQL().QueryRow(`SELECT task_id FROM subscription_tokens WHERE token = ?`, token).Scan(&taskID)
	if err != nil {
		log.Printf("subscription token=%q stage=resolve_token status=error error=%q", safeTokenLabel(token), err)
		return "", err
	}
	log.Printf("subscription token=%q stage=resolve_token status=ok task=%d", safeTokenLabel(token), taskID)
	return s.generateByTaskID(taskID, relayHost)
}

func (s *Service) loadTask(taskID int64) (storage.ConversionTask, error) {
	var task storage.ConversionTask
	var enabled, mergeDefaults, includeGlobalRules, managedEnabled, managedStrict int
	err := s.db.SQL().QueryRow(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_error_message,
			include_global_rules, custom_rules_text, rule_merge_mode, custom_groups_text,
			vless_relay_mode, managed_config_enabled, managed_config_interval_seconds, managed_config_strict,
			COALESCE((SELECT token FROM subscription_tokens WHERE task_id = conversion_tasks.id ORDER BY id ASC LIMIT 1), '')
		FROM conversion_tasks
		WHERE id = ?`, taskID).Scan(
		&task.ID, &task.UserID, &task.Name, &task.InputType, &task.OutputType, &task.SourceURL,
		&enabled, &task.RefreshIntervalSeconds, &mergeDefaults, &task.PinnedNodeOrderMode,
		&task.LastErrorMessage,
		&includeGlobalRules, &task.CustomRulesText, &task.RuleMergeMode, &task.CustomGroupsText,
		&task.VLESSRelayMode, &managedEnabled, &task.ManagedConfigIntervalSeconds, &managedStrict, &task.SubscriptionToken,
	)
	task.Enabled = enabled == 1
	task.MergeDefaultPinnedNodes = mergeDefaults == 1
	task.IncludeGlobalRules = includeGlobalRules == 1
	task.ManagedConfigEnabled = managedEnabled == 1
	task.ManagedConfigStrict = managedStrict == 1
	if task.RuleMergeMode == "" {
		task.RuleMergeMode = "custom_first"
	}
	task.VLESSRelayMode = normalizeVLESSRelayMode(task.VLESSRelayMode)
	if task.ManagedConfigIntervalSeconds == 0 {
		task.ManagedConfigIntervalSeconds = 86400
	}
	return task, err
}

func normalizeVLESSRelayMode(mode string) string {
	switch mode {
	case "enabled", "disabled":
		return mode
	default:
		return "global"
	}
}

func (s *Service) surgeConfig(task storage.ConversionTask, relayHost string) (convert.SurgeConfig, error) {
	customRules := make([]string, 0)
	if task.IncludeGlobalRules {
		global, err := s.tasks.GlobalRuleConfig()
		if err != nil {
			return convert.SurgeConfig{}, err
		}
		customRules = append(customRules, textLines(global.CustomRulesText)...)
	}
	customRules = append(customRules, textLines(task.CustomRulesText)...)

	cfg := convert.SurgeConfig{
		CustomRules:   customRules,
		CustomGroups:  textLines(task.CustomGroupsText),
		RuleMergeMode: task.RuleMergeMode,
	}
	if task.ManagedConfigEnabled {
		cfg.ManagedConfigHeader = managedConfigHeader(task, relayHost)
	}
	return cfg, nil
}

func managedConfigHeader(task storage.ConversionTask, relayHost string) string {
	if task.ManagedConfigIntervalSeconds == 0 {
		task.ManagedConfigIntervalSeconds = 86400
	}
	return fmt.Sprintf("#!MANAGED-CONFIG %s interval=%d strict=%t",
		taskSubscriptionURL(task, relayHost),
		task.ManagedConfigIntervalSeconds,
		task.ManagedConfigStrict,
	)
}

func taskSubscriptionURL(task storage.ConversionTask, relayHost string) string {
	token := strings.TrimSpace(task.SubscriptionToken)
	query := url.Values{}
	if name := strings.TrimSpace(task.Name); name != "" {
		query.Set("name", name)
	}
	suffix := ""
	if encoded := query.Encode(); encoded != "" {
		suffix = "?" + encoded
	}
	if host := strings.TrimSpace(relayHost); host != "" {
		return fmt.Sprintf("http://%s/sub/%s%s", host, token, suffix)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PROXYMORPH_PUBLIC_BASE_URL")), "/")
	if baseURL != "" {
		return fmt.Sprintf("%s/sub/%s%s", baseURL, token, suffix)
	}
	return fmt.Sprintf("/sub/%s%s", token, suffix)
}

func textLines(text string) []string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (s *Service) fetch(sourceURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("upstream returned status %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func (s *Service) effectivePinnedNodes(task storage.ConversionTask) ([]convert.Node, error) {
	if !task.MergeDefaultPinnedNodes {
		return nil, nil
	}
	return s.nodes.EffectiveDefaultNodes(task.UserID)
}

type parseInfo struct {
	Format      string
	Skipped     int
	SkipSummary string
}

func parseSubscription(content []byte) (convert.Document, parseInfo, error) {
	doc, clashErr := convert.ParseClash(content)
	if clashErr == nil && len(doc.Nodes) > 0 {
		return doc, parseInfo{Format: "clash"}, nil
	}
	if uriDoc, info, uriErr := parseURIListSubscription(content); uriErr == nil {
		return uriDoc, info, nil
	}
	if clashErr != nil {
		return convert.Document{}, parseInfo{}, clashErr
	}
	return convert.Document{}, parseInfo{}, fmt.Errorf("subscription contains no supported proxy nodes")
}

func parseURIListSubscription(content []byte) (convert.Document, parseInfo, error) {
	decoded := bytes.TrimSpace(content)
	format := "uri-list"
	if decodedContent, err := decodeSubscriptionBase64(decoded); err == nil {
		decoded = bytes.TrimSpace(decodedContent)
		format = "base64-uri-list"
	}
	lines := bytes.Split(decoded, []byte{'\n'})
	doc := convert.Document{}
	var skipped []string
	for _, line := range lines {
		raw := string(bytes.TrimSpace(line))
		if raw == "" {
			continue
		}
		node, err := nodes.ParseURI(raw)
		if err != nil {
			skipped = append(skipped, summarizeURIParseError(raw, err))
			continue
		}
		doc.Nodes = append(doc.Nodes, node)
	}
	if len(doc.Nodes) == 0 {
		if len(skipped) > 0 {
			return convert.Document{}, parseInfo{Format: format, Skipped: len(skipped), SkipSummary: strings.Join(limitStrings(skipped, 5), "; ")}, fmt.Errorf("subscription contains no supported proxy URIs; skipped %d entries: %s", len(skipped), strings.Join(limitStrings(skipped, 5), "; "))
		}
		return convert.Document{}, parseInfo{Format: format}, fmt.Errorf("subscription contains no supported proxy URIs")
	}
	return doc, parseInfo{Format: format, Skipped: len(skipped), SkipSummary: strings.Join(limitStrings(skipped, 5), "; ")}, nil
}

func decodeSubscriptionBase64(content []byte) ([]byte, error) {
	encoded := string(content)
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}

func summarizeURIParseError(raw string, err error) string {
	scheme := "unknown"
	if parsed, parseErr := url.Parse(raw); parseErr == nil && parsed.Scheme != "" {
		scheme = parsed.Scheme
	}
	return fmt.Sprintf("scheme=%s error=%v", scheme, err)
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	limited := make([]string, 0, limit+1)
	limited = append(limited, values[:limit]...)
	limited = append(limited, fmt.Sprintf("and %d more", len(values)-limit))
	return limited
}

func safeSourceLabel(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "invalid-url"
	}
	if parsed.Host == "" {
		return parsed.Scheme
	}
	return parsed.Scheme + "://" + parsed.Host
}

func safeTokenLabel(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func relayPublicHost(configuredHost, requestHost string) string {
	configuredHost = strings.TrimSpace(configuredHost)
	if configuredHost != "" && !isLocalRelayHost(configuredHost) {
		return configuredHost
	}
	requestHost = strings.TrimSpace(requestHost)
	if requestHost == "" {
		return configuredHost
	}
	host := requestHost
	if parsedHost, _, err := net.SplitHostPort(requestHost); err == nil {
		host = parsedHost
	}
	if host == "" {
		return configuredHost
	}
	return host
}

func isLocalRelayHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]"
}

func (s *Service) storeCache(taskID int64, content string) error {
	_, err := s.db.SQL().Exec(`
		INSERT INTO output_cache (task_id, content, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		taskID, content, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Service) cachedOrError(taskID int64, original error) (string, error) {
	s.recordRun(taskID, "error", original.Error())
	var cached string
	err := s.db.SQL().QueryRow(`SELECT content FROM output_cache WHERE task_id = ?`, taskID).Scan(&cached)
	if errors.Is(err, sql.ErrNoRows) {
		return "", original
	}
	if err != nil {
		return "", err
	}
	return cached, nil
}

func (s *Service) recordRun(taskID int64, status, message string) {
	_, _ = s.db.SQL().Exec(`INSERT INTO conversion_runs (task_id, status, message) VALUES (?, ?, ?)`, taskID, status, message)
	if status == "error" {
		_, _ = s.db.SQL().Exec(`UPDATE conversion_tasks SET last_error_at = CURRENT_TIMESTAMP, last_error_message = ? WHERE id = ?`, message, taskID)
	}
	if status == "success" {
		_, _ = s.db.SQL().Exec(`UPDATE conversion_tasks SET last_success_at = CURRENT_TIMESTAMP, last_error_message = '' WHERE id = ?`, taskID)
	}
}

func nodeNames(nodes []convert.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func unsupportedNodeSummary(nodes []convert.Node) string {
	if len(nodes) == 0 {
		return "none"
	}
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, fmt.Sprintf("%s(%s)", node.Name, node.Protocol))
	}
	return strings.Join(limitStrings(values, 5), ", ")
}
