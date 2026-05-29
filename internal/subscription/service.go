package subscription

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
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
	lookupIP    func(context.Context, string) ([]net.IP, error)
	nodes       *nodes.Service
	tasks       *tasks.Service
	vlessRelay  *singbox.Manager
	relayConfig config.VLESSRelayConfig
	relayMu     sync.Mutex
}

func NewService(db *storage.DB, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if client.Timeout == 0 {
		next := *client
		next.Timeout = 20 * time.Second
		client = &next
	}
	service := &Service{
		db:     db,
		client: client,
		nodes:  nodes.NewService(db),
		tasks:  tasks.NewService(db),
	}
	if client.Transport == nil {
		resolver := net.DefaultResolver
		service.lookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
			return resolver.LookupIP(ctx, "ip", host)
		}
	}
	return service
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
	return s.generateTask(task, relayHost, true)
}

func (s *Service) GeneratePreviewByTaskID(taskID int64, draft tasks.UpdateInput) (string, error) {
	task, err := s.loadTask(taskID)
	if err != nil {
		log.Printf("subscription task=%d stage=load_task status=error error=%q", taskID, err)
		return "", err
	}
	task = applyDraft(task, draft)
	return s.generateTask(task, "", false)
}

func (s *Service) generateTask(task storage.ConversionTask, relayHost string, persist bool) (string, error) {
	log.Printf("subscription task=%d stage=load_task status=ok source=%q enabled=%t", task.ID, safeSourceLabel(task.SourceURL), task.Enabled)
	if !task.Enabled {
		err := fmt.Errorf("task is disabled")
		log.Printf("subscription task=%d stage=validate status=error error=%q", task.ID, err)
		return "", err
	}
	startedAt := time.Now()
	log.Printf("subscription task=%d stage=fetch status=start source=%q", task.ID, safeSourceLabel(task.SourceURL))
	content, err := s.fetch(task.SourceURL, task.SourceUserAgent)
	if err != nil {
		log.Printf("subscription task=%d stage=fetch status=error duration_ms=%d error=%q", task.ID, time.Since(startedAt).Milliseconds(), err)
		if !persist {
			return "", err
		}
		return s.cachedOrError(task.ID, err)
	}
	log.Printf("subscription task=%d stage=fetch status=ok duration_ms=%d bytes=%d", task.ID, time.Since(startedAt).Milliseconds(), len(content))
	parseStartedAt := time.Now()
	doc, parseInfo, err := parseSubscription(content)
	if err != nil {
		log.Printf("subscription task=%d stage=parse status=error duration_ms=%d error=%q", task.ID, time.Since(parseStartedAt).Milliseconds(), err)
		if !persist {
			return "", err
		}
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
		if !persist {
			return "", err
		}
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	log.Printf("subscription task=%d stage=pinned status=ok nodes=%d merge_default=%t", task.ID, len(pinned), task.MergeDefaultPinnedNodes)
	doc.Nodes = convert.MergeNodes(doc.Nodes, pinned, convert.MergeOptions{Mode: task.PinnedNodeOrderMode})
	vlessRelayEnabled := s.vlessRelayEnabled(task)
	trojanWSRelayEnabled := s.trojanWSRelayEnabled(task)
	if vlessRelayEnabled || trojanWSRelayEnabled {
		relayStartedAt := time.Now()
		doc.Nodes, err = s.configureTaskProtocolRelays(task.ID, doc.Nodes, relayHost, vlessRelayEnabled, trojanWSRelayEnabled)
		if err != nil {
			log.Printf("subscription task=%d stage=vless_relay status=error duration_ms=%d error=%q", task.ID, time.Since(relayStartedAt).Milliseconds(), err)
			s.recordRun(task.ID, "error", err.Error())
			return "", err
		}
		log.Printf("subscription task=%d stage=vless_relay status=ok duration_ms=%d public_host=%q ports=%d-%d", task.ID, time.Since(relayStartedAt).Milliseconds(), s.relayConfig.PublicHost, s.relayConfig.PortStart, s.relayConfig.PortEnd)
	} else if s.vlessRelay != nil {
		if err := s.clearTaskVLESSRelays(task.ID); err != nil {
			log.Printf("subscription task=%d stage=vless_relay_clear status=error error=%q", task.ID, err)
			if !persist {
				return "", err
			}
			s.recordRun(task.ID, "error", err.Error())
			return "", err
		}
	}
	var unsupported []convert.Node
	doc.Nodes, unsupported = convert.Surge6SupportedNodes(doc.Nodes)
	if len(unsupported) > 0 {
		log.Printf("subscription task=%d stage=render unsupported_nodes=%d summary=%q", task.ID, len(unsupported), unsupportedNodeSummary(unsupported))
	}
	if len(doc.Nodes) == 0 {
		err := fmt.Errorf("subscription contains no Surge 6 compatible proxy nodes; unsupported nodes: %s", unsupportedNodeSummary(unsupported))
		log.Printf("subscription task=%d stage=render status=error error=%q", task.ID, err)
		if !persist {
			return "", err
		}
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
		if !persist {
			return "", err
		}
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	rendered, err := convert.RenderSurge6WithWarnings(doc.Nodes, doc.Groups, doc.Rules, surgeConfig)
	if err != nil {
		log.Printf("subscription task=%d stage=render status=error error=%q", task.ID, err)
		if !persist {
			return "", err
		}
		s.recordRun(task.ID, "error", err.Error())
		return "", err
	}
	output := rendered.Output
	if rendered.Warning != "" {
		warnErr := errors.New(rendered.Warning)
		log.Printf("subscription task=%d stage=render status=warning warning=%q", task.ID, rendered.Warning)
		if !persist {
			return output, warnErr
		}
		if err := s.storeCache(task.ID, output); err != nil {
			log.Printf("subscription task=%d stage=cache status=error error=%q", task.ID, err)
			return "", err
		}
		s.recordRun(task.ID, "error", rendered.Warning)
		return output, warnErr
	}
	if !persist {
		log.Printf("subscription task=%d stage=render status=ok preview=true nodes=%d groups=%d output_bytes=%d", task.ID, len(doc.Nodes), len(doc.Groups), len(output))
		return output, nil
	}
	if err := s.storeCache(task.ID, output); err != nil {
		log.Printf("subscription task=%d stage=cache status=error error=%q", task.ID, err)
		return "", err
	}
	s.recordRun(task.ID, "success", "")
	log.Printf("subscription task=%d stage=render status=ok nodes=%d groups=%d output_bytes=%d", task.ID, len(doc.Nodes), len(doc.Groups), len(output))
	return output, nil
}

func applyDraft(task storage.ConversionTask, input tasks.UpdateInput) storage.ConversionTask {
	if input.Name != nil {
		task.Name = *input.Name
	}
	if input.SourceURL != nil {
		task.SourceURL = *input.SourceURL
	}
	if input.SourceUserAgent != nil {
		task.SourceUserAgent = strings.TrimSpace(*input.SourceUserAgent)
	}
	if input.RefreshIntervalSeconds != nil && *input.RefreshIntervalSeconds > 0 {
		task.RefreshIntervalSeconds = *input.RefreshIntervalSeconds
	}
	if input.Enabled != nil {
		task.Enabled = *input.Enabled
	}
	if input.MergeDefaultPinnedNodes != nil {
		task.MergeDefaultPinnedNodes = *input.MergeDefaultPinnedNodes
	}
	if input.IncludeGlobalRules != nil {
		task.IncludeGlobalRules = *input.IncludeGlobalRules
	}
	if input.CustomRulesText != nil {
		task.CustomRulesText = *input.CustomRulesText
	}
	if input.RuleMergeMode != nil {
		task.RuleMergeMode = *input.RuleMergeMode
	}
	if input.FinalRulePolicy != nil {
		task.FinalRulePolicy = strings.TrimSpace(*input.FinalRulePolicy)
	}
	if input.CustomGroupsText != nil {
		task.CustomGroupsText = *input.CustomGroupsText
	}
	if input.VLESSRelayMode != nil {
		task.VLESSRelayMode = *input.VLESSRelayMode
	}
	if input.TrojanWSRelayMode != nil {
		task.TrojanWSRelayMode = *input.TrojanWSRelayMode
	}
	if input.ManagedConfigMode != nil {
		task.ManagedConfigMode = *input.ManagedConfigMode
	}
	if input.ManagedConfigURLMode != nil {
		task.ManagedConfigURLMode = *input.ManagedConfigURLMode
	}
	if input.ManagedConfigCustomURL != nil {
		task.ManagedConfigCustomURL = strings.TrimSpace(*input.ManagedConfigCustomURL)
	}
	if input.ManagedConfigIntervalMode != nil {
		task.ManagedConfigIntervalMode = *input.ManagedConfigIntervalMode
	}
	if input.ManagedConfigIntervalSeconds != nil {
		task.ManagedConfigIntervalSeconds = *input.ManagedConfigIntervalSeconds
	}
	if input.ManagedConfigStrictMode != nil {
		task.ManagedConfigStrictMode = *input.ManagedConfigStrictMode
	}
	return task
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

func (s *Service) trojanWSRelayEnabled(task storage.ConversionTask) bool {
	if s.vlessRelay == nil {
		return false
	}
	switch normalizeTrojanWSRelayMode(task.TrojanWSRelayMode) {
	case "enabled":
		return true
	case "disabled":
		return false
	}
	enabled, err := s.tasks.TrojanWSRelayEnabled()
	if err != nil {
		log.Printf("subscription stage=trojan_ws_relay_setting status=error error=%q", err)
		return false
	}
	return enabled
}

func (s *Service) configureRelayHostLocked(requestHost string) error {
	host := relayPublicHost(s.relayConfig.PublicHost, requestHost)
	if host == s.relayConfig.PublicHost {
		return nil
	}
	if s.vlessRelay != nil {
		if err := s.vlessRelay.Stop(); err != nil {
			return err
		}
	}
	s.relayConfig.PublicHost = host
	s.vlessRelay = singbox.NewManager(s.relayConfig)
	return nil
}

func (s *Service) configureTaskVLESSRelays(taskID int64, nodes []convert.Node, relayHost string) ([]convert.Node, error) {
	return s.configureTaskProtocolRelays(taskID, nodes, relayHost, true, false)
}

func (s *Service) configureTaskProtocolRelays(taskID int64, nodes []convert.Node, relayHost string, includeVLESS bool, includeTrojanWS bool) ([]convert.Node, error) {
	if s.vlessRelay == nil {
		return nodes, nil
	}
	s.relayMu.Lock()
	defer s.relayMu.Unlock()
	if err := s.configureRelayHostLocked(relayHost); err != nil {
		return nil, err
	}
	relayNodes := make([]convert.Node, 0)
	for _, node := range nodes {
		if relayNodeEnabled(node, includeVLESS, includeTrojanWS) {
			relayNodes = append(relayNodes, node)
		}
	}
	ports, err := s.upsertTaskRelayEntries(taskID, relayNodes)
	if err != nil {
		return nil, err
	}
	next := make([]convert.Node, 0, len(nodes))
	for _, node := range nodes {
		if !relayNodeEnabled(node, includeVLESS, includeTrojanWS) {
			next = append(next, node)
			continue
		}
		port, ok := ports[node.Name]
		if !ok {
			return nil, fmt.Errorf("%s relay port missing for node %q", node.Protocol, node.Name)
		}
		next = append(next, convert.Node{
			Name:     node.Name,
			Protocol: "socks5",
			Server:   s.relayConfig.PublicHost,
			Port:     port,
			Params: map[string]string{
				"username":       s.relayConfig.Username,
				"password":       s.relayConfig.Password,
				"relay_protocol": node.Protocol,
			},
			Tags:   node.Tags,
			Pinned: node.Pinned,
		})
	}
	if err := s.rebuildAndStartVLESSRelayLocked(); err != nil {
		return nil, err
	}
	return next, nil
}

func relayNodeEnabled(node convert.Node, includeVLESS bool, includeTrojanWS bool) bool {
	if includeVLESS && node.Protocol == "vless" {
		return true
	}
	return includeTrojanWS && node.Protocol == "trojan" && node.Params["network"] == "ws"
}

func (s *Service) clearTaskVLESSRelays(taskID int64) error {
	s.relayMu.Lock()
	defer s.relayMu.Unlock()
	if _, err := s.db.SQL().Exec(`DELETE FROM vless_relay_entries WHERE task_id = ?`, taskID); err != nil {
		return err
	}
	return s.rebuildAndStartVLESSRelayLocked()
}

func (s *Service) upsertTaskRelayEntries(taskID int64, nodes []convert.Node) (map[string]int, error) {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT node_name, port FROM vless_relay_entries WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]int)
	for rows.Next() {
		var name string
		var port int
		if err := rows.Scan(&name, &port); err != nil {
			rows.Close()
			return nil, err
		}
		existing[name] = port
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	usedRows, err := tx.Query(`SELECT port FROM vless_relay_entries WHERE task_id <> ?`, taskID)
	if err != nil {
		return nil, err
	}
	used := make(map[int]bool)
	for usedRows.Next() {
		var port int
		if err := usedRows.Scan(&port); err != nil {
			usedRows.Close()
			return nil, err
		}
		if relayPortInRange(s.relayConfig, port) {
			used[port] = true
		}
	}
	if err := usedRows.Close(); err != nil {
		return nil, err
	}
	if err := usedRows.Err(); err != nil {
		return nil, err
	}
	keep := make(map[string]bool, len(nodes))
	ports := make(map[string]int, len(nodes))
	for _, node := range nodes {
		if err := validateRelayNode(node); err != nil {
			return nil, err
		}
		keep[node.Name] = true
		port := existing[node.Name]
		if port == 0 || !relayPortInRange(s.relayConfig, port) || used[port] {
			port = nextRelayPort(s.relayConfig, used)
			if port == 0 {
				return nil, fmt.Errorf("not enough relay ports: need %d, have %d", len(nodes), s.relayConfig.PortEnd-s.relayConfig.PortStart+1)
			}
		}
		used[port] = true
		ports[node.Name] = port
		content, err := json.Marshal(node)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`
			INSERT INTO vless_relay_entries (task_id, node_name, port, node_json, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(task_id, node_name) DO UPDATE SET
				port = excluded.port,
				node_json = excluded.node_json,
				updated_at = excluded.updated_at`,
			taskID, node.Name, port, string(content), time.Now().UTC().Format(time.RFC3339)); err != nil {
			return nil, err
		}
	}
	for name := range existing {
		if !keep[name] {
			if _, err := tx.Exec(`DELETE FROM vless_relay_entries WHERE task_id = ? AND node_name = ?`, taskID, name); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ports, nil
}

func validateRelayNode(node convert.Node) error {
	switch node.Protocol {
	case "vless":
		if node.Params["uuid"] == "" {
			return fmt.Errorf("vless node %q uuid is required", node.Name)
		}
	case "trojan":
		if node.Params["password"] == "" {
			return fmt.Errorf("trojan node %q password is required", node.Name)
		}
	}
	return nil
}

func nextRelayPort(cfg config.VLESSRelayConfig, used map[int]bool) int {
	for port := cfg.PortStart; port <= cfg.PortEnd; port++ {
		if !used[port] {
			return port
		}
	}
	return 0
}

func relayPortInRange(cfg config.VLESSRelayConfig, port int) bool {
	return port >= cfg.PortStart && port <= cfg.PortEnd
}

func (s *Service) RestoreVLESSRelay() error {
	if s.vlessRelay == nil {
		return nil
	}
	hasRelays, err := s.hasRestorableVLESSRelays()
	if err != nil || !hasRelays {
		return nil
	}
	s.relayMu.Lock()
	defer s.relayMu.Unlock()
	return s.rebuildAndStartVLESSRelayLocked()
}

func (s *Service) hasRestorableVLESSRelays() (bool, error) {
	var count int
	err := s.db.SQL().QueryRow(`
		SELECT COUNT(*)
		FROM vless_relay_entries r
		JOIN conversion_tasks t ON t.id = r.task_id
		WHERE t.enabled = 1`).Scan(&count)
	return count > 0, err
}

func (s *Service) rebuildAndStartVLESSRelayLocked() error {
	if err := s.normalizeRestorableRelayPortsLocked(); err != nil {
		return err
	}
	relays, err := s.loadRelayEntries()
	if err != nil {
		return err
	}
	if err := s.vlessRelay.ConfigureRelays(relays); err != nil {
		return err
	}
	if len(relays) == 0 {
		return s.vlessRelay.Stop()
	}
	return s.vlessRelay.Start()
}

type relayPortReassignment struct {
	TaskID   int64
	NodeName string
}

func (s *Service) normalizeRestorableRelayPortsLocked() error {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
		SELECT r.task_id, r.node_name, r.port
		FROM vless_relay_entries r
		JOIN conversion_tasks t ON t.id = r.task_id
		WHERE t.enabled = 1
		ORDER BY r.port ASC, r.task_id ASC, r.node_name ASC`)
	if err != nil {
		return err
	}
	used := make(map[int]bool)
	reassign := make([]relayPortReassignment, 0)
	for rows.Next() {
		var taskID int64
		var nodeName string
		var port int
		if err := rows.Scan(&taskID, &nodeName, &port); err != nil {
			rows.Close()
			return err
		}
		if relayPortInRange(s.relayConfig, port) && !used[port] {
			used[port] = true
			continue
		}
		reassign = append(reassign, relayPortReassignment{TaskID: taskID, NodeName: nodeName})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range reassign {
		port := nextRelayPort(s.relayConfig, used)
		if port == 0 {
			return fmt.Errorf("not enough VLESS relay ports in configured range %d-%d", s.relayConfig.PortStart, s.relayConfig.PortEnd)
		}
		used[port] = true
		if _, err := tx.Exec(`
			UPDATE vless_relay_entries
			SET port = ?, updated_at = ?
			WHERE task_id = ? AND node_name = ?`,
			port, time.Now().UTC().Format(time.RFC3339), item.TaskID, item.NodeName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) loadRelayEntries() ([]singbox.Relay, error) {
	rows, err := s.db.SQL().Query(`
		SELECT r.task_id, r.node_name, r.port, r.node_json
		FROM vless_relay_entries r
		JOIN conversion_tasks t ON t.id = r.task_id
		WHERE t.enabled = 1
		ORDER BY r.port ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	relays := make([]singbox.Relay, 0)
	for rows.Next() {
		var relay singbox.Relay
		var nodeJSON string
		if err := rows.Scan(&relay.TaskID, &relay.NodeName, &relay.Port, &nodeJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(nodeJSON), &relay.Node); err != nil {
			return nil, err
		}
		relays = append(relays, relay)
	}
	return relays, rows.Err()
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
	var enabled, mergeDefaults, includeGlobalRules int
	err := s.db.SQL().QueryRow(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			source_user_agent, refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_error_message,
			include_global_rules, custom_rules_text, rule_merge_mode, final_rule_policy, custom_groups_text,
			vless_relay_mode, trojan_ws_relay_mode, managed_config_mode, managed_config_url_mode, managed_config_custom_url,
			managed_config_interval_mode, managed_config_interval_seconds, managed_config_strict_mode,
			COALESCE((SELECT token FROM subscription_tokens WHERE task_id = conversion_tasks.id ORDER BY id ASC LIMIT 1), '')
		FROM conversion_tasks
		WHERE id = ?`, taskID).Scan(
		&task.ID, &task.UserID, &task.Name, &task.InputType, &task.OutputType, &task.SourceURL,
		&enabled, &task.SourceUserAgent, &task.RefreshIntervalSeconds, &mergeDefaults, &task.PinnedNodeOrderMode,
		&task.LastErrorMessage,
		&includeGlobalRules, &task.CustomRulesText, &task.RuleMergeMode, &task.FinalRulePolicy, &task.CustomGroupsText,
		&task.VLESSRelayMode, &task.TrojanWSRelayMode, &task.ManagedConfigMode, &task.ManagedConfigURLMode, &task.ManagedConfigCustomURL,
		&task.ManagedConfigIntervalMode, &task.ManagedConfigIntervalSeconds, &task.ManagedConfigStrictMode,
		&task.SubscriptionToken,
	)
	task.Enabled = enabled == 1
	task.MergeDefaultPinnedNodes = mergeDefaults == 1
	task.IncludeGlobalRules = includeGlobalRules == 1
	if task.RuleMergeMode == "" {
		task.RuleMergeMode = "custom_first"
	}
	task.FinalRulePolicy = strings.TrimSpace(task.FinalRulePolicy)
	task.VLESSRelayMode = normalizeVLESSRelayMode(task.VLESSRelayMode)
	task.TrojanWSRelayMode = normalizeTrojanWSRelayMode(task.TrojanWSRelayMode)
	task.ManagedConfigMode = normalizeTriStateMode(task.ManagedConfigMode)
	task.ManagedConfigURLMode = normalizeTaskManagedURLMode(task.ManagedConfigURLMode)
	task.ManagedConfigCustomURL = strings.TrimSpace(task.ManagedConfigCustomURL)
	task.ManagedConfigIntervalMode = normalizeIntervalMode(task.ManagedConfigIntervalMode)
	task.ManagedConfigStrictMode = normalizeTriStateMode(task.ManagedConfigStrictMode)
	if task.ManagedConfigIntervalSeconds == 0 {
		task.ManagedConfigIntervalSeconds = 86400
	}
	return task, err
}

func normalizeVLESSRelayMode(mode string) string {
	return normalizeTriStateMode(mode)
}

func normalizeTrojanWSRelayMode(mode string) string {
	return normalizeTriStateMode(mode)
}

func normalizeTriStateMode(mode string) string {
	switch mode {
	case "enabled", "disabled":
		return mode
	default:
		return "global"
	}
}

func normalizeTaskManagedURLMode(mode string) string {
	switch mode {
	case "task_subscription", "custom":
		return mode
	default:
		return "global"
	}
}

func normalizeIntervalMode(mode string) string {
	if mode == "custom" {
		return mode
	}
	return "global"
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
		CustomRules:     customRules,
		CustomGroups:    textLines(task.CustomGroupsText),
		RuleMergeMode:   task.RuleMergeMode,
		FinalRulePolicy: task.FinalRulePolicy,
	}
	managed, err := s.effectiveManagedConfig(task, relayHost)
	if err != nil {
		return convert.SurgeConfig{}, err
	}
	if managed.Enabled {
		cfg.ManagedConfigHeader = managedConfigHeader(managed)
	}
	return cfg, nil
}

type effectiveManagedConfig struct {
	Enabled         bool
	URL             string
	IntervalSeconds int
	Strict          bool
}

func (s *Service) effectiveManagedConfig(task storage.ConversionTask, relayHost string) (effectiveManagedConfig, error) {
	defaults, err := s.tasks.ManagedConfigDefaults()
	if err != nil {
		return effectiveManagedConfig{}, err
	}
	enabled := defaults.Enabled
	switch normalizeTriStateMode(task.ManagedConfigMode) {
	case "enabled":
		enabled = true
	case "disabled":
		enabled = false
	}
	if !enabled {
		return effectiveManagedConfig{Enabled: false}, nil
	}

	urlMode := task.ManagedConfigURLMode
	if urlMode == "" || urlMode == "global" {
		urlMode = defaults.URLMode
	}
	managedURL := taskSubscriptionURL(task, relayHost)
	if urlMode == "custom" {
		managedURL = strings.TrimSpace(defaults.CustomURL)
		if strings.TrimSpace(task.ManagedConfigCustomURL) != "" && task.ManagedConfigURLMode == "custom" {
			managedURL = strings.TrimSpace(task.ManagedConfigCustomURL)
		}
	}
	if managedURL == "" {
		return effectiveManagedConfig{}, fmt.Errorf("managed config url is required")
	}

	interval := defaults.IntervalSeconds
	if interval == 0 {
		interval = 86400
	}
	if task.ManagedConfigIntervalMode == "custom" {
		interval = task.ManagedConfigIntervalSeconds
	}
	if interval < 60 {
		return effectiveManagedConfig{}, fmt.Errorf("managed config interval must be at least 60 seconds")
	}

	strict := defaults.Strict
	switch normalizeTriStateMode(task.ManagedConfigStrictMode) {
	case "enabled":
		strict = true
	case "disabled":
		strict = false
	}
	return effectiveManagedConfig{Enabled: true, URL: managedURL, IntervalSeconds: interval, Strict: strict}, nil
}

func managedConfigHeader(config effectiveManagedConfig) string {
	return fmt.Sprintf("#!MANAGED-CONFIG %s interval=%d strict=%t",
		config.URL,
		config.IntervalSeconds,
		config.Strict,
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

func (s *Service) fetch(sourceURL string, userAgent ...string) ([]byte, error) {
	if err := s.validateSourceURL(sourceURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", sourceUserAgent(userAgent...))
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("upstream returned status %d", res.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(res.Body, maxSubscriptionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxSubscriptionBytes {
		return nil, fmt.Errorf("upstream response is too large")
	}
	return content, nil
}

const maxSubscriptionBytes = 5 * 1024 * 1024

const defaultSubscriptionUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"

func sourceUserAgent(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return defaultSubscriptionUserAgent
}

func (s *Service) validateSourceURL(sourceURL string) error {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("subscription source url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("subscription source url must use http or https")
	}
	host := parsed.Hostname()
	if isLocalSourceHostname(host) {
		return fmt.Errorf("subscription source url must not point to a private or local address")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicSourceIP(ip) {
			return fmt.Errorf("subscription source url must not point to a private or local address")
		}
		return nil
	}
	if s.lookupIP == nil {
		return nil
	}
	ips, err := s.lookupIP(context.Background(), host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("subscription source host has no addresses")
	}
	if slices.ContainsFunc(ips, func(ip net.IP) bool { return !isPublicSourceIP(ip) }) {
		return fmt.Errorf("subscription source url must not resolve to a private or local address")
	}
	return nil
}

func isLocalSourceHostname(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func isPublicSourceIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast()
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
