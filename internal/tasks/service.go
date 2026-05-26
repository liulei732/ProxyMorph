package tasks

import (
	"database/sql"
	"errors"

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

func NewService(db *storage.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(userID int64, input CreateInput) (storage.ConversionTask, error) {
	if input.RefreshIntervalSeconds == 0 {
		input.RefreshIntervalSeconds = 3600
	}
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
	return s.Get(userID, id)
}

func (s *Service) Get(userID, id int64) (storage.ConversionTask, error) {
	row := s.db.SQL().QueryRow(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_error_message
		FROM conversion_tasks
		WHERE user_id = ? AND id = ?`, userID, id)
	return scanTask(row)
}

func (s *Service) List(userID int64) ([]storage.ConversionTask, error) {
	rows, err := s.db.SQL().Query(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_error_message
		FROM conversion_tasks
		WHERE user_id = ?
		ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []storage.ConversionTask
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, rows.Err()
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (storage.ConversionTask, error) {
	var task storage.ConversionTask
	var enabled, mergeDefaults int
	err := row.Scan(
		&task.ID, &task.UserID, &task.Name, &task.InputType, &task.OutputType, &task.SourceURL,
		&enabled, &task.RefreshIntervalSeconds, &mergeDefaults, &task.PinnedNodeOrderMode,
		&task.LastErrorMessage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ConversionTask{}, err
	}
	task.Enabled = enabled == 1
	task.MergeDefaultPinnedNodes = mergeDefaults == 1
	return task, err
}
