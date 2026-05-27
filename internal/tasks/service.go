package tasks

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/liulei/proxymorph/internal/storage"
)

type Service struct {
	db *storage.DB
}

type CreateInput struct {
	Name                   string `json:"name"`
	SourceURL              string `json:"source_url"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
}

type UpdateInput struct {
	Name                    *string `json:"name"`
	SourceURL               *string `json:"source_url"`
	RefreshIntervalSeconds  *int    `json:"refresh_interval_seconds"`
	Enabled                 *bool   `json:"enabled"`
	MergeDefaultPinnedNodes *bool   `json:"merge_default_pinned_nodes"`
}

func NewService(db *storage.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(userID int64, input CreateInput) (storage.ConversionTask, error) {
	if input.RefreshIntervalSeconds == 0 {
		input.RefreshIntervalSeconds = 3600
	}
	input.Name = defaultTaskName(input.Name, input.SourceURL)
	res, err := s.db.SQL().Exec(`
		INSERT INTO conversion_tasks (
			user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode
		) VALUES (?, ?, 'clash', 'surge6', ?, 1, ?, 1, 'after_remote')`,
		userID, input.Name, input.SourceURL, input.RefreshIntervalSeconds,
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
	enabled := 0
	if task.Enabled {
		enabled = 1
	}
	mergeDefaults := 0
	if task.MergeDefaultPinnedNodes {
		mergeDefaults = 1
	}
	_, err = s.db.SQL().Exec(`
		UPDATE conversion_tasks
		SET name = ?, source_url = ?, refresh_interval_seconds = ?, enabled = ?,
			merge_default_pinned_nodes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		task.Name, task.SourceURL, task.RefreshIntervalSeconds, enabled, mergeDefaults, userID, id,
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

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (storage.ConversionTask, error) {
	var task storage.ConversionTask
	var enabled, mergeDefaults int
	var lastSuccessAt, lastErrorAt sql.NullString
	err := row.Scan(
		&task.ID, &task.UserID, &task.Name, &task.InputType, &task.OutputType, &task.SourceURL,
		&enabled, &task.RefreshIntervalSeconds, &mergeDefaults, &task.PinnedNodeOrderMode,
		&lastSuccessAt, &lastErrorAt, &task.LastErrorMessage, &task.SubscriptionToken,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ConversionTask{}, err
	}
	task.Enabled = enabled == 1
	task.MergeDefaultPinnedNodes = mergeDefaults == 1
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
