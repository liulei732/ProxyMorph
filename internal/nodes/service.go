package nodes

import (
	"encoding/json"

	"github.com/liulei/proxymorph/internal/convert"
	"github.com/liulei/proxymorph/internal/storage"
)

type Service struct {
	db *storage.DB
}

type CreateInput struct {
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	Server         string            `json:"server"`
	Port           int               `json:"port"`
	Params         map[string]string `json:"params"`
	Tags           []string          `json:"tags"`
	Enabled        bool              `json:"enabled"`
	DefaultInclude bool              `json:"default_include"`
	SortOrder      int               `json:"sort_order"`
}

func NewService(db *storage.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ImportURIs(userID int64, uris []string) ([]storage.PinnedNode, error) {
	imported := make([]storage.PinnedNode, 0, len(uris))
	for _, raw := range uris {
		node, err := ParseURI(raw)
		if err != nil {
			return nil, err
		}
		stored, err := s.Create(userID, CreateInput{
			Name:     node.Name,
			Protocol: node.Protocol,
			Server:   node.Server,
			Port:     node.Port,
			Params:   node.Params,
			Tags:     node.Tags,
			Enabled:  true,
		})
		if err != nil {
			return nil, err
		}
		imported = append(imported, stored)
	}
	return imported, nil
}

func (s *Service) Create(userID int64, input CreateInput) (storage.PinnedNode, error) {
	params, err := json.Marshal(input.Params)
	if err != nil {
		return storage.PinnedNode{}, err
	}
	tags, err := json.Marshal(input.Tags)
	if err != nil {
		return storage.PinnedNode{}, err
	}
	enabled := boolInt(input.Enabled)
	res, err := s.db.SQL().Exec(`
		INSERT INTO pinned_nodes (
			user_id, name, protocol, server, port, parameters_json, tags_json,
			enabled, default_include, sort_order
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, input.Name, input.Protocol, input.Server, input.Port, string(params), string(tags),
		enabled, boolInt(input.DefaultInclude), input.SortOrder,
	)
	if err != nil {
		return storage.PinnedNode{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return storage.PinnedNode{}, err
	}
	return s.Get(userID, id)
}

func (s *Service) Get(userID, id int64) (storage.PinnedNode, error) {
	row := s.db.SQL().QueryRow(`
		SELECT id, user_id, name, protocol, server, port, parameters_json, tags_json,
			enabled, default_include, sort_order
		FROM pinned_nodes
		WHERE user_id = ? AND id = ?`, userID, id)
	return scanPinnedNode(row)
}

func (s *Service) List(userID int64) ([]storage.PinnedNode, error) {
	rows, err := s.db.SQL().Query(`
		SELECT id, user_id, name, protocol, server, port, parameters_json, tags_json,
			enabled, default_include, sort_order
		FROM pinned_nodes
		WHERE user_id = ?
		ORDER BY sort_order ASC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []storage.PinnedNode
	for rows.Next() {
		node, err := scanPinnedNode(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, node)
	}
	return result, rows.Err()
}

func (s *Service) EffectiveDefaultNodes(userID int64) ([]convert.Node, error) {
	rows, err := s.db.SQL().Query(`
		SELECT name, protocol, server, port, parameters_json, tags_json
		FROM pinned_nodes
		WHERE user_id = ? AND enabled = 1 AND default_include = 1
		ORDER BY sort_order ASC, id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []convert.Node
	for rows.Next() {
		var node convert.Node
		var paramsJSON, tagsJSON string
		if err := rows.Scan(&node.Name, &node.Protocol, &node.Server, &node.Port, &paramsJSON, &tagsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(paramsJSON), &node.Params)
		_ = json.Unmarshal([]byte(tagsJSON), &node.Tags)
		node.Pinned = true
		result = append(result, node)
	}
	return result, rows.Err()
}

type nodeScanner interface {
	Scan(dest ...any) error
}

func scanPinnedNode(row nodeScanner) (storage.PinnedNode, error) {
	var node storage.PinnedNode
	var enabled, defaultInclude int
	err := row.Scan(
		&node.ID, &node.UserID, &node.Name, &node.Protocol, &node.Server, &node.Port,
		&node.ParametersJSON, &node.TagsJSON, &enabled, &defaultInclude, &node.SortOrder,
	)
	node.Enabled = enabled == 1
	node.DefaultInclude = defaultInclude == 1
	return node, err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
