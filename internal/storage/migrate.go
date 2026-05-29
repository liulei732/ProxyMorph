package storage

import (
	"context"
	"strings"
)

func (db *DB) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS conversion_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			input_type TEXT NOT NULL,
			output_type TEXT NOT NULL,
			source_url TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			refresh_interval_seconds INTEGER NOT NULL DEFAULT 3600,
			merge_default_pinned_nodes INTEGER NOT NULL DEFAULT 1,
			pinned_node_order_mode TEXT NOT NULL DEFAULT 'after_remote',
			last_success_at TEXT,
			last_error_at TEXT,
			last_error_message TEXT NOT NULL DEFAULT '',
			include_global_rules INTEGER NOT NULL DEFAULT 1,
			custom_rules_text TEXT NOT NULL DEFAULT '',
			rule_merge_mode TEXT NOT NULL DEFAULT 'custom_first',
			final_rule_policy TEXT NOT NULL DEFAULT '',
			custom_groups_text TEXT NOT NULL DEFAULT '',
			vless_relay_mode TEXT NOT NULL DEFAULT 'global',
			trojan_ws_relay_mode TEXT NOT NULL DEFAULT 'global',
			managed_config_mode TEXT NOT NULL DEFAULT 'global',
			managed_config_url_mode TEXT NOT NULL DEFAULT 'global',
			managed_config_custom_url TEXT NOT NULL DEFAULT '',
			managed_config_interval_mode TEXT NOT NULL DEFAULT 'global',
			managed_config_interval_seconds INTEGER NOT NULL DEFAULT 86400,
			managed_config_strict_mode TEXT NOT NULL DEFAULT 'global',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pinned_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			protocol TEXT NOT NULL,
			server TEXT NOT NULL,
			port INTEGER NOT NULL,
			parameters_json TEXT NOT NULL DEFAULT '{}',
			tags_json TEXT NOT NULL DEFAULT '[]',
			enabled INTEGER NOT NULL DEFAULT 1,
			default_include INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_node_overrides (
			task_id INTEGER NOT NULL REFERENCES conversion_tasks(id),
			node_id INTEGER NOT NULL REFERENCES pinned_nodes(id),
			mode TEXT NOT NULL CHECK (mode IN ('include', 'exclude')),
			PRIMARY KEY (task_id, node_id)
		)`,
		`CREATE TABLE IF NOT EXISTS subscription_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL REFERENCES conversion_tasks(id),
			token TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS conversion_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL REFERENCES conversion_tasks(id),
			status TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS output_cache (
			task_id INTEGER PRIMARY KEY REFERENCES conversion_tasks(id),
			content TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS vless_relay_entries (
			task_id INTEGER NOT NULL REFERENCES conversion_tasks(id),
			node_name TEXT NOT NULL,
			port INTEGER NOT NULL UNIQUE,
			node_json TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (task_id, node_name)
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.sql.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	for _, stmt := range []string{
		`ALTER TABLE conversion_tasks ADD COLUMN include_global_rules INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE conversion_tasks ADD COLUMN custom_rules_text TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE conversion_tasks ADD COLUMN rule_merge_mode TEXT NOT NULL DEFAULT 'custom_first'`,
		`ALTER TABLE conversion_tasks ADD COLUMN final_rule_policy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE conversion_tasks ADD COLUMN custom_groups_text TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE conversion_tasks ADD COLUMN vless_relay_mode TEXT NOT NULL DEFAULT 'global'`,
		`ALTER TABLE conversion_tasks ADD COLUMN trojan_ws_relay_mode TEXT NOT NULL DEFAULT 'global'`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_enabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_mode TEXT NOT NULL DEFAULT 'global'`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_url_mode TEXT NOT NULL DEFAULT 'global'`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_custom_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_interval_mode TEXT NOT NULL DEFAULT 'global'`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_interval_seconds INTEGER NOT NULL DEFAULT 86400`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_strict INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE conversion_tasks ADD COLUMN managed_config_strict_mode TEXT NOT NULL DEFAULT 'global'`,
	} {
		if _, err := db.sql.ExecContext(ctx, stmt); err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}
	if _, err := db.sql.ExecContext(ctx, `DELETE FROM app_settings WHERE key = 'global_rule_merge_mode'`); err != nil {
		return err
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}
