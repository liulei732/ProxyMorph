package tasks

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/liulei/proxymorph/internal/storage"
)

type Service struct {
	db *storage.DB
}

const defaultSubscriptionInfoKeywordsText = "剩余流量\n流量\n重置\n套餐到期\n到期\n官网\n刷新订阅\ntraffic\nexpire\nreset"

type CreateInput struct {
	Name                   string `json:"name"`
	SourceURL              string `json:"source_url"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	VLESSRelayMode         string `json:"vless_relay_mode"`
}

type UpdateInput struct {
	Name                         *string `json:"name"`
	SourceURL                    *string `json:"source_url"`
	RefreshIntervalSeconds       *int    `json:"refresh_interval_seconds"`
	Enabled                      *bool   `json:"enabled"`
	MergeDefaultPinnedNodes      *bool   `json:"merge_default_pinned_nodes"`
	IncludeGlobalRules           *bool   `json:"include_global_rules"`
	CustomRulesText              *string `json:"custom_rules_text"`
	RuleMergeMode                *string `json:"rule_merge_mode"`
	FinalRulePolicy              *string `json:"final_rule_policy"`
	CustomGroupsText             *string `json:"custom_groups_text"`
	VLESSRelayMode               *string `json:"vless_relay_mode"`
	ManagedConfigMode            *string `json:"managed_config_mode"`
	ManagedConfigURLMode         *string `json:"managed_config_url_mode"`
	ManagedConfigCustomURL       *string `json:"managed_config_custom_url"`
	ManagedConfigIntervalMode    *string `json:"managed_config_interval_mode"`
	ManagedConfigIntervalSeconds *int    `json:"managed_config_interval_seconds"`
	ManagedConfigStrictMode      *string `json:"managed_config_strict_mode"`
}

type GlobalRuleConfigInput struct {
	CustomRulesText              string `json:"custom_rules_text"`
	VLESSRelayEnabled            bool   `json:"vless_relay_enabled"`
	SubscriptionInfoKeywordsText string `json:"subscription_info_keywords_text"`
}

type ManagedConfigDefaultsInput struct {
	Enabled         bool   `json:"enabled"`
	URLMode         string `json:"url_mode"`
	CustomURL       string `json:"custom_url"`
	IntervalSeconds int    `json:"interval_seconds"`
	Strict          bool   `json:"strict"`
}

func NewService(db *storage.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(userID int64, input CreateInput) (storage.ConversionTask, error) {
	if input.RefreshIntervalSeconds == 0 {
		input.RefreshIntervalSeconds = 3600
	}
	input.Name = defaultTaskName(input.Name, input.SourceURL)
	vlessRelayMode := normalizeVLESSRelayMode(input.VLESSRelayMode)
	res, err := s.db.SQL().Exec(`
		INSERT INTO conversion_tasks (
			user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode, vless_relay_mode
		) VALUES (?, ?, 'clash', 'surge6', ?, 1, ?, 1, 'after_remote', ?)`,
		userID, input.Name, input.SourceURL, input.RefreshIntervalSeconds, vlessRelayMode,
	)
	if err != nil {
		return storage.ConversionTask{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return storage.ConversionTask{}, err
	}
	if err := s.ensureSubscriptionToken(id); err != nil {
		return storage.ConversionTask{}, err
	}
	return s.Get(userID, id)
}

func (s *Service) Get(userID, id int64) (storage.ConversionTask, error) {
	task, err := s.get(userID, id)
	if err != nil {
		return storage.ConversionTask{}, err
	}
	if task.SubscriptionToken == "" {
		if err := s.ensureSubscriptionToken(id); err != nil {
			return storage.ConversionTask{}, err
		}
		return s.get(userID, id)
	}
	return task, nil
}

func (s *Service) Update(userID, id int64, input UpdateInput) (storage.ConversionTask, error) {
	if err := validateTaskUpdate(input); err != nil {
		return storage.ConversionTask{}, err
	}
	task, err := s.Get(userID, id)
	if err != nil {
		return storage.ConversionTask{}, err
	}
	if input.Name != nil {
		task.Name = *input.Name
	}
	if input.SourceURL != nil {
		task.SourceURL = *input.SourceURL
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
		task.RuleMergeMode = normalizeRuleMergeMode(*input.RuleMergeMode)
	}
	if input.FinalRulePolicy != nil {
		task.FinalRulePolicy = normalizeFinalRulePolicy(*input.FinalRulePolicy)
	}
	if input.CustomGroupsText != nil {
		task.CustomGroupsText = *input.CustomGroupsText
	}
	if input.VLESSRelayMode != nil {
		task.VLESSRelayMode = normalizeVLESSRelayMode(*input.VLESSRelayMode)
	}
	if input.ManagedConfigMode != nil {
		task.ManagedConfigMode = normalizeTriStateMode(*input.ManagedConfigMode)
	}
	if input.ManagedConfigURLMode != nil {
		task.ManagedConfigURLMode = normalizeTaskManagedURLMode(*input.ManagedConfigURLMode)
	}
	if input.ManagedConfigCustomURL != nil {
		task.ManagedConfigCustomURL = strings.TrimSpace(*input.ManagedConfigCustomURL)
	}
	if input.ManagedConfigIntervalMode != nil {
		task.ManagedConfigIntervalMode = normalizeIntervalMode(*input.ManagedConfigIntervalMode)
	}
	if input.ManagedConfigIntervalSeconds != nil {
		task.ManagedConfigIntervalSeconds = *input.ManagedConfigIntervalSeconds
	}
	if input.ManagedConfigStrictMode != nil {
		task.ManagedConfigStrictMode = normalizeTriStateMode(*input.ManagedConfigStrictMode)
	}
	enabled := 0
	if task.Enabled {
		enabled = 1
	}
	mergeDefaults := 0
	if task.MergeDefaultPinnedNodes {
		mergeDefaults = 1
	}
	includeGlobalRules := boolToInt(task.IncludeGlobalRules)
	_, err = s.db.SQL().Exec(`
		UPDATE conversion_tasks
		SET name = ?, source_url = ?, refresh_interval_seconds = ?, enabled = ?,
			merge_default_pinned_nodes = ?, include_global_rules = ?, custom_rules_text = ?,
			rule_merge_mode = ?, final_rule_policy = ?, custom_groups_text = ?, vless_relay_mode = ?,
			managed_config_mode = ?, managed_config_url_mode = ?, managed_config_custom_url = ?,
			managed_config_interval_mode = ?, managed_config_interval_seconds = ?, managed_config_strict_mode = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		task.Name, task.SourceURL, task.RefreshIntervalSeconds, enabled, mergeDefaults,
		includeGlobalRules, task.CustomRulesText, task.RuleMergeMode, task.FinalRulePolicy, task.CustomGroupsText,
		task.VLESSRelayMode, task.ManagedConfigMode, task.ManagedConfigURLMode, task.ManagedConfigCustomURL,
		task.ManagedConfigIntervalMode, task.ManagedConfigIntervalSeconds, task.ManagedConfigStrictMode,
		userID, id,
	)
	if err != nil {
		return storage.ConversionTask{}, err
	}
	return s.Get(userID, id)
}

func (s *Service) Delete(userID, id int64) error {
	if _, err := s.Get(userID, id); err != nil {
		return err
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM output_cache WHERE task_id = ?`,
		`DELETE FROM conversion_runs WHERE task_id = ?`,
		`DELETE FROM task_node_overrides WHERE task_id = ?`,
		`DELETE FROM subscription_tokens WHERE task_id = ?`,
		`DELETE FROM conversion_tasks WHERE id = ? AND user_id = ?`,
	} {
		if stmt == `DELETE FROM conversion_tasks WHERE id = ? AND user_id = ?` {
			if _, err := tx.Exec(stmt, id, userID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(stmt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) List(userID int64) ([]storage.ConversionTask, error) {
	rows, err := s.db.SQL().Query(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_success_at, last_error_at, last_error_message,
			include_global_rules, custom_rules_text, rule_merge_mode, final_rule_policy, custom_groups_text,
			vless_relay_mode, managed_config_mode, managed_config_url_mode, managed_config_custom_url,
			managed_config_interval_mode, managed_config_interval_seconds, managed_config_strict_mode,
			COALESCE((SELECT token FROM subscription_tokens WHERE task_id = conversion_tasks.id ORDER BY id ASC LIMIT 1), '')
		FROM conversion_tasks
		WHERE user_id = ?
		ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]storage.ConversionTask, 0)
	missingTokenIDs := make([]int64, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		if task.SubscriptionToken == "" {
			missingTokenIDs = append(missingTokenIDs, task.ID)
		}
		result = append(result, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(missingTokenIDs) == 0 {
		return result, nil
	}
	for _, id := range missingTokenIDs {
		if err := s.ensureSubscriptionToken(id); err != nil {
			return nil, err
		}
	}
	return s.List(userID)
}

func (s *Service) get(userID, id int64) (storage.ConversionTask, error) {
	row := s.db.SQL().QueryRow(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_success_at, last_error_at, last_error_message,
			include_global_rules, custom_rules_text, rule_merge_mode, final_rule_policy, custom_groups_text,
			vless_relay_mode, managed_config_mode, managed_config_url_mode, managed_config_custom_url,
			managed_config_interval_mode, managed_config_interval_seconds, managed_config_strict_mode,
			COALESCE((SELECT token FROM subscription_tokens WHERE task_id = conversion_tasks.id ORDER BY id ASC LIMIT 1), '')
		FROM conversion_tasks
		WHERE user_id = ? AND id = ?`, userID, id)
	return scanTask(row)
}

func (s *Service) ensureSubscriptionToken(taskID int64) error {
	var token string
	err := s.db.SQL().QueryRow(`SELECT token FROM subscription_tokens WHERE task_id = ? ORDER BY id ASC LIMIT 1`, taskID).Scan(&token)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	token, err = randomToken()
	if err != nil {
		return err
	}
	_, err = s.db.SQL().Exec(`INSERT INTO subscription_tokens (task_id, token) VALUES (?, ?)`, taskID, token)
	return err
}

func randomToken() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (s *Service) GlobalRuleConfig() (storage.GlobalRuleConfig, error) {
	values, err := s.settings("global_custom_rules_text", "vless_relay_enabled", "subscription_info_keywords_text")
	if err != nil {
		return storage.GlobalRuleConfig{}, err
	}
	keywords := strings.TrimSpace(values["subscription_info_keywords_text"])
	if keywords == "" {
		keywords = defaultSubscriptionInfoKeywordsText
	}
	return storage.GlobalRuleConfig{
		CustomRulesText:              values["global_custom_rules_text"],
		VLESSRelayEnabled:            settingBool(values["vless_relay_enabled"]),
		SubscriptionInfoKeywordsText: keywords,
	}, nil
}

func (s *Service) UpdateGlobalRuleConfig(input GlobalRuleConfigInput) (storage.GlobalRuleConfig, error) {
	if err := validateCustomRulesText(input.CustomRulesText); err != nil {
		return storage.GlobalRuleConfig{}, err
	}
	if err := s.setSetting("global_custom_rules_text", input.CustomRulesText); err != nil {
		return storage.GlobalRuleConfig{}, err
	}
	if err := s.setSetting("vless_relay_enabled", boolSetting(input.VLESSRelayEnabled)); err != nil {
		return storage.GlobalRuleConfig{}, err
	}
	if err := s.setSetting("subscription_info_keywords_text", strings.TrimSpace(input.SubscriptionInfoKeywordsText)); err != nil {
		return storage.GlobalRuleConfig{}, err
	}
	return s.GlobalRuleConfig()
}

func (s *Service) CachedOutput(userID, taskID int64) (string, error) {
	if _, err := s.Get(userID, taskID); err != nil {
		return "", err
	}
	var content string
	err := s.db.SQL().QueryRow(`SELECT content FROM output_cache WHERE task_id = ?`, taskID).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return content, err
}

func (s *Service) VLESSRelayEnabled() (bool, error) {
	values, err := s.settings("vless_relay_enabled")
	if err != nil {
		return false, err
	}
	return settingBool(values["vless_relay_enabled"]), nil
}

func (s *Service) ManagedConfigDefaults() (storage.ManagedConfigDefaults, error) {
	values, err := s.settings(
		"managed_config_enabled",
		"managed_config_url_mode",
		"managed_config_custom_url",
		"managed_config_interval_seconds",
		"managed_config_strict",
	)
	if err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	interval := settingInt(values["managed_config_interval_seconds"], 86400)
	if interval < 60 {
		interval = 86400
	}
	return storage.ManagedConfigDefaults{
		Enabled:         settingBool(values["managed_config_enabled"]),
		URLMode:         normalizeGlobalManagedURLMode(values["managed_config_url_mode"]),
		CustomURL:       strings.TrimSpace(values["managed_config_custom_url"]),
		IntervalSeconds: interval,
		Strict:          settingBool(values["managed_config_strict"]),
	}, nil
}

func (s *Service) UpdateManagedConfigDefaults(input ManagedConfigDefaultsInput) (storage.ManagedConfigDefaults, error) {
	input.URLMode = normalizeGlobalManagedURLMode(input.URLMode)
	if input.IntervalSeconds == 0 {
		input.IntervalSeconds = 86400
	}
	if input.IntervalSeconds < 60 {
		return storage.ManagedConfigDefaults{}, fmt.Errorf("managed config interval must be at least 60 seconds")
	}
	if input.URLMode == "custom" {
		if err := validateHTTPURL(input.CustomURL, input.Enabled); err != nil {
			return storage.ManagedConfigDefaults{}, err
		}
	}
	if err := s.setSetting("managed_config_enabled", boolSetting(input.Enabled)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_url_mode", input.URLMode); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_custom_url", strings.TrimSpace(input.CustomURL)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_interval_seconds", fmt.Sprintf("%d", input.IntervalSeconds)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_strict", boolSetting(input.Strict)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	return s.ManagedConfigDefaults()
}

func validateTaskUpdate(input UpdateInput) error {
	if input.CustomRulesText != nil {
		if err := validateCustomRulesText(*input.CustomRulesText); err != nil {
			return err
		}
	}
	if input.CustomGroupsText != nil {
		if err := validateCustomGroupsText(*input.CustomGroupsText); err != nil {
			return err
		}
	}
	if input.ManagedConfigIntervalSeconds != nil && *input.ManagedConfigIntervalSeconds < 60 {
		return fmt.Errorf("managed config interval must be at least 60 seconds")
	}
	if input.ManagedConfigCustomURL != nil {
		if err := validateHTTPURL(*input.ManagedConfigCustomURL, false); err != nil {
			return err
		}
	}
	return nil
}

func validateCustomRulesText(text string) error {
	if containsSectionHeader(text, "[Rule]") {
		return fmt.Errorf("custom rules must not include [Rule] section header")
	}
	return nil
}

func validateCustomGroupsText(text string) error {
	if containsSectionHeader(text, "[Proxy Group]") {
		return fmt.Errorf("custom groups must not include [Proxy Group] section header")
	}
	return nil
}

func containsSectionHeader(text, header string) bool {
	header = strings.ToLower(strings.TrimSpace(header))
	for _, line := range strings.Split(text, "\n") {
		if strings.ToLower(strings.TrimSpace(line)) == header {
			return true
		}
	}
	return false
}

func validateHTTPURL(value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("managed config custom url is required")
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("managed config custom url must be an absolute http or https url")
	}
	return nil
}

func (s *Service) settings(keys ...string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = ""
	}
	rows, err := s.db.SQL().Query(`SELECT key, value FROM app_settings WHERE key IN (`+placeholders(len(keys))+`)`, anySlice(keys)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

func (s *Service) setSetting(key, value string) error {
	_, err := s.db.SQL().Exec(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

func placeholders(count int) string {
	values := make([]string, 0, count)
	for range count {
		values = append(values, "?")
	}
	return strings.Join(values, ",")
}

func anySlice(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func normalizeRuleMergeMode(mode string) string {
	switch mode {
	case "upstream_first", "custom_first_dedupe", "upstream_first_dedupe":
		return mode
	default:
		return "custom_first"
	}
}

func normalizeFinalRulePolicy(policy string) string {
	return strings.TrimSpace(policy)
}

func normalizeVLESSRelayMode(mode string) string {
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

func normalizeGlobalManagedURLMode(mode string) string {
	if mode == "custom" {
		return mode
	}
	return "task_subscription"
}

func normalizeIntervalMode(mode string) string {
	if mode == "custom" {
		return mode
	}
	return "global"
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func settingBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func settingInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func boolSetting(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (storage.ConversionTask, error) {
	var task storage.ConversionTask
	var enabled, mergeDefaults, includeGlobalRules int
	var lastSuccessAt, lastErrorAt sql.NullString
	err := row.Scan(
		&task.ID, &task.UserID, &task.Name, &task.InputType, &task.OutputType, &task.SourceURL,
		&enabled, &task.RefreshIntervalSeconds, &mergeDefaults, &task.PinnedNodeOrderMode,
		&lastSuccessAt, &lastErrorAt, &task.LastErrorMessage,
		&includeGlobalRules, &task.CustomRulesText, &task.RuleMergeMode, &task.FinalRulePolicy, &task.CustomGroupsText,
		&task.VLESSRelayMode, &task.ManagedConfigMode, &task.ManagedConfigURLMode, &task.ManagedConfigCustomURL,
		&task.ManagedConfigIntervalMode, &task.ManagedConfigIntervalSeconds, &task.ManagedConfigStrictMode,
		&task.SubscriptionToken,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ConversionTask{}, err
	}
	task.Enabled = enabled == 1
	task.MergeDefaultPinnedNodes = mergeDefaults == 1
	task.IncludeGlobalRules = includeGlobalRules == 1
	task.RuleMergeMode = normalizeRuleMergeMode(task.RuleMergeMode)
	task.FinalRulePolicy = normalizeFinalRulePolicy(task.FinalRulePolicy)
	task.VLESSRelayMode = normalizeVLESSRelayMode(task.VLESSRelayMode)
	task.ManagedConfigMode = normalizeTriStateMode(task.ManagedConfigMode)
	task.ManagedConfigURLMode = normalizeTaskManagedURLMode(task.ManagedConfigURLMode)
	task.ManagedConfigCustomURL = strings.TrimSpace(task.ManagedConfigCustomURL)
	task.ManagedConfigIntervalMode = normalizeIntervalMode(task.ManagedConfigIntervalMode)
	task.ManagedConfigStrictMode = normalizeTriStateMode(task.ManagedConfigStrictMode)
	if task.ManagedConfigIntervalSeconds == 0 {
		task.ManagedConfigIntervalSeconds = 86400
	}
	task.LastSuccessAt = parseOptionalTime(lastSuccessAt)
	task.LastErrorAt = parseOptionalTime(lastErrorAt)
	if task.SubscriptionToken != "" {
		task.SubscriptionURL = subscriptionURL(task.SubscriptionToken, task.Name)
	}
	return task, err
}

func parseOptionalTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value.String)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func defaultTaskName(name, sourceURL string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("name"))
}

func subscriptionURL(token, name string) string {
	baseURL := strings.TrimRight(os.Getenv("PROXYMORPH_PUBLIC_BASE_URL"), "/")
	query := url.Values{}
	if strings.TrimSpace(name) != "" {
		query.Set("name", strings.TrimSpace(name))
	}
	suffix := ""
	if encoded := query.Encode(); encoded != "" {
		suffix = "?" + encoded
	}
	if baseURL == "" {
		return fmt.Sprintf("/sub/%s%s", token, suffix)
	}
	return fmt.Sprintf("%s/sub/%s%s", baseURL, token, suffix)
}
