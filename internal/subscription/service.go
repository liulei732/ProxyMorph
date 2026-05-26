package subscription

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/liulei/proxymorph/internal/convert"
	"github.com/liulei/proxymorph/internal/nodes"
	"github.com/liulei/proxymorph/internal/storage"
)

type Service struct {
	db     *storage.DB
	client *http.Client
	nodes  *nodes.Service
}

func NewService(db *storage.DB, client *http.Client) *Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &Service{db: db, client: client, nodes: nodes.NewService(db)}
}

func (s *Service) GenerateByTaskID(taskID int64) (string, error) {
	task, err := s.loadTask(taskID)
	if err != nil {
		return "", err
	}
	if !task.Enabled {
		return "", fmt.Errorf("task is disabled")
	}
	content, err := s.fetch(task.SourceURL)
	if err != nil {
		return s.cachedOrError(task.ID, err)
	}
	doc, err := convert.ParseClash(content)
	if err != nil {
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	pinned, err := s.effectivePinnedNodes(task)
	if err != nil {
		s.recordRun(task.ID, "error", err.Error())
		return s.cachedOrError(task.ID, err)
	}
	doc.Nodes = convert.MergeNodes(doc.Nodes, pinned, convert.MergeOptions{Mode: task.PinnedNodeOrderMode})
	if len(doc.Groups) == 0 {
		doc.Groups = []convert.Group{{Name: "Proxy", Type: "select", Proxies: nodeNames(doc.Nodes)}}
	}
	output := convert.RenderSurge6(doc.Nodes, doc.Groups, doc.Rules)
	if err := s.storeCache(task.ID, output); err != nil {
		return "", err
	}
	s.recordRun(task.ID, "success", "")
	return output, nil
}

func (s *Service) GenerateByToken(token string) (string, error) {
	var taskID int64
	err := s.db.SQL().QueryRow(`SELECT task_id FROM subscription_tokens WHERE token = ?`, token).Scan(&taskID)
	if err != nil {
		return "", err
	}
	return s.GenerateByTaskID(taskID)
}

func (s *Service) loadTask(taskID int64) (storage.ConversionTask, error) {
	var task storage.ConversionTask
	var enabled, mergeDefaults int
	err := s.db.SQL().QueryRow(`
		SELECT id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode,
			last_error_message
		FROM conversion_tasks
		WHERE id = ?`, taskID).Scan(
		&task.ID, &task.UserID, &task.Name, &task.InputType, &task.OutputType, &task.SourceURL,
		&enabled, &task.RefreshIntervalSeconds, &mergeDefaults, &task.PinnedNodeOrderMode,
		&task.LastErrorMessage,
	)
	task.Enabled = enabled == 1
	task.MergeDefaultPinnedNodes = mergeDefaults == 1
	return task, err
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
