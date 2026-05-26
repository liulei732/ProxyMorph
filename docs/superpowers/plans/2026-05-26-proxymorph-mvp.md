# ProxyMorph MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first self-hosted ProxyMorph release: a Go single-binary service with admin login, Clash-to-Surge 6 conversion, pinned node merging, embedded management UI, SQLite storage, and Docker deployment.

**Architecture:** The backend is a modular Go HTTP service with SQLite persistence, tokenized public subscription endpoints, and embedded frontend assets. Conversion uses an internal normalized node model so Clash remote nodes and pinned nodes can be merged before rendering Surge 6 output.

**Tech Stack:** Go 1.22+, SQLite via `modernc.org/sqlite`, migrations in Go, `net/http`, `golang.org/x/crypto/bcrypt`, YAML parsing via `gopkg.in/yaml.v3`, frontend with Vite + React + TypeScript, Docker multi-stage build.

---

## File Structure

Create this structure:

```text
cmd/proxymorph/main.go
internal/app/app.go
internal/app/routes.go
internal/config/config.go
internal/storage/db.go
internal/storage/migrate.go
internal/storage/models.go
internal/auth/auth.go
internal/auth/handlers.go
internal/httpapi/middleware.go
internal/httpapi/respond.go
internal/tasks/handlers.go
internal/tasks/service.go
internal/nodes/handlers.go
internal/nodes/parser.go
internal/nodes/service.go
internal/convert/model.go
internal/convert/clash.go
internal/convert/surge.go
internal/convert/merge.go
internal/subscription/handlers.go
internal/subscription/service.go
web/package.json
web/index.html
web/src/main.tsx
web/src/App.tsx
web/src/api.ts
web/src/styles.css
Dockerfile
docker-compose.yml
README.md
```

Responsibility map:

- `cmd/proxymorph`: process entrypoint only.
- `internal/app`: dependency wiring, server construction, route registration.
- `internal/config`: environment parsing and defaults.
- `internal/storage`: SQLite connection, schema, and data structs.
- `internal/auth`: admin account initialization, login, password change, session cookies.
- `internal/httpapi`: shared JSON responses and auth middleware.
- `internal/tasks`: conversion task CRUD and task-level pinned-node settings.
- `internal/nodes`: pinned node CRUD, URI import, and node parsing.
- `internal/convert`: internal proxy model, Clash parser, merge logic, Surge 6 renderer.
- `internal/subscription`: public tokenized subscription URL handling and cache behavior.
- `web`: management UI.

## Task 1: Go Module And Config Foundation

**Files:**
- Create: `go.mod`
- Create: `cmd/proxymorph/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing config tests**

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PROXYMORPH_ADDR", "")
	t.Setenv("PROXYMORPH_DATA_DIR", "")
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "")
	t.Setenv("PROXYMORPH_ADMIN_USERNAME", "")
	t.Setenv("PROXYMORPH_ADMIN_PASSWORD", "")
	t.Setenv("PROXYMORPH_SESSION_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.DataDir != "/data" {
		t.Fatalf("DataDir = %q, want /data", cfg.DataDir)
	}
}

func TestLoadRequiresSessionSecret(t *testing.T) {
	t.Setenv("PROXYMORPH_SESSION_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error for dev defaults: %v", err)
	}
	if cfg.SessionSecret == "" {
		t.Fatal("SessionSecret must not be empty")
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("PROXYMORPH_ADDR", ":9090")
	t.Setenv("PROXYMORPH_DATA_DIR", "/tmp/proxymorph")
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "https://proxy.example.test")
	t.Setenv("PROXYMORPH_ADMIN_USERNAME", "root")
	t.Setenv("PROXYMORPH_ADMIN_PASSWORD", "secret")
	t.Setenv("PROXYMORPH_SESSION_SECRET", "01234567890123456789012345678901")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Addr != ":9090" || cfg.DataDir != "/tmp/proxymorph" || cfg.PublicBaseURL != "https://proxy.example.test" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.InitialAdminUsername != "root" || cfg.InitialAdminPassword != "secret" {
		t.Fatalf("unexpected admin config: %#v", cfg)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/config
```

Expected: FAIL because `go.mod` and `Load` do not exist.

- [ ] **Step 3: Create module and config implementation**

Create `go.mod`:

```go
module github.com/liulei/proxymorph

go 1.22
```

Create `internal/config/config.go`:

```go
package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
)

type Config struct {
	Addr                 string
	DataDir              string
	PublicBaseURL        string
	InitialAdminUsername string
	InitialAdminPassword string
	SessionSecret        string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                 env("PROXYMORPH_ADDR", ":8080"),
		DataDir:              env("PROXYMORPH_DATA_DIR", "/data"),
		PublicBaseURL:        os.Getenv("PROXYMORPH_PUBLIC_BASE_URL"),
		InitialAdminUsername: env("PROXYMORPH_ADMIN_USERNAME", "admin"),
		InitialAdminPassword: os.Getenv("PROXYMORPH_ADMIN_PASSWORD"),
		SessionSecret:        os.Getenv("PROXYMORPH_SESSION_SECRET"),
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = randomSecret()
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func randomSecret() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "development-session-secret-change-me"
	}
	return base64.RawURLEncoding.EncodeToString(buf[:])
}
```

Create `cmd/proxymorph/main.go`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/liulei/proxymorph/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ProxyMorph listening on %s\n", cfg.Addr)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run:

```bash
go test ./internal/config
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add go.mod cmd/proxymorph/main.go internal/config/config.go internal/config/config_test.go
git commit -m "chore: initialize Go config foundation"
```

## Task 2: SQLite Schema And Store

**Files:**
- Create: `internal/storage/models.go`
- Create: `internal/storage/db.go`
- Create: `internal/storage/migrate.go`
- Create: `internal/storage/storage_test.go`
- Modify: `go.mod`

- [ ] **Step 1: Write failing storage tests**

Create `internal/storage/storage_test.go`:

```go
package storage

import (
	"path/filepath"
	"testing"
)

func TestOpenMigratesSchema(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "proxymorph.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	tables := []string{"users", "conversion_tasks", "pinned_nodes", "task_node_overrides", "subscription_tokens", "conversion_runs", "output_cache"}
	for _, table := range tables {
		if !db.HasTableForTest(t, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/storage
```

Expected: FAIL because the storage package does not exist and SQLite dependency is missing.

- [ ] **Step 3: Implement storage models and migrations**

Create `internal/storage/models.go`:

```go
package storage

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ConversionTask struct {
	ID                       int64
	UserID                   int64
	Name                     string
	InputType                string
	OutputType               string
	SourceURL                string
	Enabled                  bool
	RefreshIntervalSeconds   int
	MergeDefaultPinnedNodes  bool
	PinnedNodeOrderMode      string
	LastSuccessAt            *time.Time
	LastErrorAt              *time.Time
	LastErrorMessage         string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type PinnedNode struct {
	ID             int64
	UserID         int64
	Name           string
	Protocol       string
	Server         string
	Port           int
	ParametersJSON string
	TagsJSON       string
	Enabled        bool
	DefaultInclude bool
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
```

Create `internal/storage/db.go`:

```go
package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(1)
	raw.SetConnMaxLifetime(time.Hour)
	db := &DB{sql: raw}
	if err := db.Migrate(context.Background()); err != nil {
		raw.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) SQL() *sql.DB {
	return db.sql
}

func (db *DB) HasTableForTest(t *testing.T, name string) bool {
	t.Helper()
	var count int
	err := db.sql.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	if err != nil {
		t.Fatalf("checking table %s: %v", name, err)
	}
	return count == 1
}
```

Create `internal/storage/migrate.go`:

```go
package storage

import "context"

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
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
	}
	for _, stmt := range statements {
		if _, err := db.sql.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Fetch dependencies and run test**

Run:

```bash
go mod tidy
go test ./internal/storage
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add go.mod go.sum internal/storage
git commit -m "feat: add sqlite storage schema"
```

## Task 3: Internal Proxy Model, Pinned URI Parser, And Merge Logic

**Files:**
- Create: `internal/convert/model.go`
- Create: `internal/convert/merge.go`
- Create: `internal/convert/merge_test.go`
- Create: `internal/nodes/parser.go`
- Create: `internal/nodes/parser_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/nodes/parser_test.go`:

```go
package nodes

import "testing"

func TestParseShadowsocksURI(t *testing.T) {
	node, err := ParseURI("ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388#Home")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "Home" || node.Protocol != "ss" || node.Server != "example.com" || node.Port != 8388 {
		t.Fatalf("unexpected node: %#v", node)
	}
	if node.Params["cipher"] != "aes-256-gcm" || node.Params["password"] != "password" {
		t.Fatalf("unexpected params: %#v", node.Params)
	}
}

func TestParseTrojanURI(t *testing.T) {
	node, err := ParseURI("trojan://secret@example.com:443?sni=edge.example.com#Edge")
	if err != nil {
		t.Fatalf("ParseURI returned error: %v", err)
	}
	if node.Name != "Edge" || node.Protocol != "trojan" || node.Params["password"] != "secret" || node.Params["sni"] != "edge.example.com" {
		t.Fatalf("unexpected node: %#v", node)
	}
}

func TestParseRejectsUnsupportedURI(t *testing.T) {
	if _, err := ParseURI("http://example.com"); err == nil {
		t.Fatal("expected unsupported URI error")
	}
}
```

- [ ] **Step 2: Write failing merge tests**

Create `internal/convert/merge_test.go`:

```go
package convert

import "testing"

func TestMergeAppendsPinnedNodesAndRenamesConflicts(t *testing.T) {
	remote := []Node{{Name: "A", Protocol: "ss", Server: "a.example", Port: 1}}
	pinned := []Node{
		{Name: "A", Protocol: "trojan", Server: "b.example", Port: 443, Pinned: true},
		{Name: "C", Protocol: "ss", Server: "c.example", Port: 8388, Pinned: true},
	}
	got := MergeNodes(remote, pinned, MergeOptions{Mode: "after_remote"})
	names := []string{got[0].Name, got[1].Name, got[2].Name}
	want := []string{"A", "A 2", "C"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %#v, want %#v", names, want)
		}
	}
}

func TestMergeDropsExactDuplicates(t *testing.T) {
	remote := []Node{{Name: "A", Protocol: "ss", Server: "a.example", Port: 1, Params: map[string]string{"password": "x"}}}
	pinned := []Node{{Name: "Pinned A", Protocol: "ss", Server: "a.example", Port: 1, Params: map[string]string{"password": "x"}, Pinned: true}}
	got := MergeNodes(remote, pinned, MergeOptions{Mode: "after_remote"})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/nodes ./internal/convert
```

Expected: FAIL because parser and merge functions do not exist.

- [ ] **Step 4: Implement model, parser, and merge**

Create `internal/convert/model.go`:

```go
package convert

type Node struct {
	Name     string
	Protocol string
	Server   string
	Port     int
	Params   map[string]string
	Tags     []string
	Pinned   bool
}

type MergeOptions struct {
	Mode string
}
```

Create `internal/nodes/parser.go`:

```go
package nodes

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/liulei/proxymorph/internal/convert"
)

func ParseURI(raw string) (convert.Node, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return convert.Node{}, err
	}
	switch u.Scheme {
	case "ss":
		return parseSS(u)
	case "trojan":
		return parseTrojan(u)
	default:
		return convert.Node{}, fmt.Errorf("unsupported proxy URI scheme %q", u.Scheme)
	}
}

func parseSS(u *url.URL) (convert.Node, error) {
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid ss port: %w", err)
	}
	userInfo, err := decodeBase64(u.User.String())
	if err != nil {
		return convert.Node{}, err
	}
	parts := strings.SplitN(userInfo, ":", 2)
	if len(parts) != 2 {
		return convert.Node{}, fmt.Errorf("invalid ss user info")
	}
	name, _ := url.PathUnescape(u.Fragment)
	return convert.Node{
		Name:     fallbackName(name, host),
		Protocol: "ss",
		Server:   host,
		Port:     port,
		Params: map[string]string{
			"cipher":   parts[0],
			"password": parts[1],
		},
		Pinned: true,
	}, nil
}

func parseTrojan(u *url.URL) (convert.Node, error) {
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return convert.Node{}, fmt.Errorf("invalid trojan port: %w", err)
	}
	name, _ := url.PathUnescape(u.Fragment)
	params := map[string]string{"password": u.User.Username()}
	if sni := u.Query().Get("sni"); sni != "" {
		params["sni"] = sni
	}
	return convert.Node{
		Name:     fallbackName(name, u.Hostname()),
		Protocol: "trojan",
		Server:   u.Hostname(),
		Port:     port,
		Params:   params,
		Pinned:   true,
	}, nil
}

func decodeBase64(value string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(value)
	}
	if err != nil {
		return "", fmt.Errorf("invalid base64 user info: %w", err)
	}
	return string(decoded), nil
}

func fallbackName(name, host string) string {
	if name != "" {
		return name
	}
	return host
}
```

Create `internal/convert/merge.go`:

```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
go test ./internal/nodes ./internal/convert
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/convert internal/nodes
git commit -m "feat: add proxy node parsing and merge logic"
```

## Task 4: Clash Parser And Surge 6 Renderer

**Files:**
- Create: `internal/convert/clash.go`
- Create: `internal/convert/surge.go`
- Create: `internal/convert/converter_test.go`
- Modify: `go.mod`

- [ ] **Step 1: Write failing converter tests**

Create `internal/convert/converter_test.go`:

```go
package convert

import (
	"strings"
	"testing"
)

func TestParseClashAndRenderSurge(t *testing.T) {
	input := []byte(`
proxies:
  - name: "HK 1"
    type: ss
    server: hk.example.com
    port: 8388
    cipher: aes-256-gcm
    password: pass
  - name: "Edge"
    type: trojan
    server: edge.example.com
    port: 443
    password: secret
    sni: edge.example.com
proxy-groups:
  - name: "Proxy"
    type: select
    proxies:
      - "HK 1"
rules:
  - DOMAIN-SUFFIX,example.com,Proxy
  - FINAL,Proxy
`)
	doc, err := ParseClash(input)
	if err != nil {
		t.Fatalf("ParseClash returned error: %v", err)
	}
	if len(doc.Nodes) != 2 || len(doc.Rules) != 2 {
		t.Fatalf("unexpected parsed doc: %#v", doc)
	}
	out := RenderSurge6(doc.Nodes, doc.Groups, doc.Rules)
	for _, want := range []string{
		"[Proxy]",
		"HK 1 = ss, hk.example.com, 8388",
		"Edge = trojan, edge.example.com, 443",
		"[Proxy Group]",
		"Proxy = select, HK 1",
		"[Rule]",
		"DOMAIN-SUFFIX,example.com,Proxy",
		"FINAL,Proxy",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/convert
```

Expected: FAIL because `ParseClash` and `RenderSurge6` do not exist.

- [ ] **Step 3: Implement Clash parser and Surge renderer**

Create `internal/convert/clash.go`:

```go
package convert

import "gopkg.in/yaml.v3"

type Document struct {
	Nodes  []Node
	Groups []Group
	Rules  []string
}

type Group struct {
	Name    string
	Type    string
	Proxies []string
}

type clashFile struct {
	Proxies []map[string]any `yaml:"proxies"`
	Groups  []struct {
		Name    string   `yaml:"name"`
		Type    string   `yaml:"type"`
		Proxies []string `yaml:"proxies"`
	} `yaml:"proxy-groups"`
	Rules []string `yaml:"rules"`
}

func ParseClash(data []byte) (Document, error) {
	var in clashFile
	if err := yaml.Unmarshal(data, &in); err != nil {
		return Document{}, err
	}
	doc := Document{Rules: in.Rules}
	for _, proxy := range in.Proxies {
		node := Node{
			Name:     stringValue(proxy["name"]),
			Protocol: stringValue(proxy["type"]),
			Server:   stringValue(proxy["server"]),
			Port:     intValue(proxy["port"]),
			Params:   map[string]string{},
		}
		for _, key := range []string{"cipher", "password", "sni", "network", "ws-opts"} {
			if value := stringValue(proxy[key]); value != "" {
				node.Params[key] = value
			}
		}
		doc.Nodes = append(doc.Nodes, node)
	}
	for _, group := range in.Groups {
		doc.Groups = append(doc.Groups, Group{Name: group.Name, Type: group.Type, Proxies: group.Proxies})
	}
	return doc, nil
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
```

Create `internal/convert/surge.go`:

```go
package convert

import (
	"fmt"
	"sort"
	"strings"
)

func RenderSurge6(nodes []Node, groups []Group, rules []string) string {
	var b strings.Builder
	b.WriteString("[Proxy]\n")
	for _, node := range nodes {
		b.WriteString(renderNode(node))
		b.WriteString("\n")
	}
	b.WriteString("\n[Proxy Group]\n")
	if len(groups) == 0 {
		names := make([]string, 0, len(nodes))
		for _, node := range nodes {
			names = append(names, node.Name)
		}
		b.WriteString("Proxy = select")
		if len(names) > 0 {
			b.WriteString(", ")
			b.WriteString(strings.Join(names, ", "))
		}
		b.WriteString("\n")
	} else {
		for _, group := range groups {
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
	b.WriteString("\n[Rule]\n")
	for _, rule := range rules {
		b.WriteString(rule)
		b.WriteString("\n")
	}
	return b.String()
}

func renderNode(node Node) string {
	switch node.Protocol {
	case "ss":
		return fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s", node.Name, node.Server, node.Port, node.Params["cipher"], node.Params["password"])
	case "trojan":
		parts := []string{fmt.Sprintf("%s = trojan, %s, %d, password=%s", node.Name, node.Server, node.Port, node.Params["password"])}
		if sni := node.Params["sni"]; sni != "" {
			parts = append(parts, "sni="+sni)
		}
		return strings.Join(parts, ", ")
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
		return strings.Join(parts, ", ")
	}
}
```

- [ ] **Step 4: Fetch dependency and run test**

Run:

```bash
go mod tidy
go test ./internal/convert
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add go.mod go.sum internal/convert
git commit -m "feat: convert clash subscriptions to surge"
```

## Task 5: Auth Service And Login API

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/handlers.go`
- Create: `internal/auth/auth_test.go`
- Modify: `internal/storage/db.go`
- Modify: `go.mod`

- [ ] **Step 1: Write failing auth tests**

Create `internal/auth/auth_test.go`:

```go
package auth

import (
	"path/filepath"
	"testing"

	"github.com/liulei/proxymorph/internal/storage"
)

func TestEnsureAdminAndAuthenticate(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "password"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	user, err := service.Authenticate("admin", "password")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if user.Username != "admin" || user.ID == 0 {
		t.Fatalf("unexpected user: %#v", user)
	}
	if _, err := service.Authenticate("admin", "wrong"); err == nil {
		t.Fatal("expected wrong password error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/auth
```

Expected: FAIL because auth package does not exist.

- [ ] **Step 3: Implement auth service**

Create `internal/auth/auth.go`:

```go
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/liulei/proxymorph/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	db     *storage.DB
	secret []byte
}

func NewService(db *storage.DB, secret []byte) *Service {
	return &Service{db: db, secret: secret}
}

func (s *Service) EnsureAdmin(username, password string) error {
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "admin"
	}
	var count int
	if err := s.db.SQL().QueryRow(`SELECT count(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, string(hash))
	return err
}

func (s *Service) Authenticate(username, password string) (storage.User, error) {
	var user storage.User
	err := s.db.SQL().QueryRow(`SELECT id, username, password_hash FROM users WHERE username = ?`, username).Scan(&user.ID, &user.Username, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return storage.User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return storage.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Service) SignUserID(userID int64) string {
	payload := fmt.Sprint(userID)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func (s *Service) VerifyToken(token string) (int64, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, false
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(want)) {
		return 0, false
	}
	var id int64
	if _, err := fmt.Sscan(parts[0], &id); err != nil {
		return 0, false
	}
	return id, true
}

func (s *Service) UserByID(ctx context.Context, id int64) (storage.User, error) {
	var user storage.User
	err := s.db.SQL().QueryRowContext(ctx, `SELECT id, username, password_hash FROM users WHERE id = ?`, id).Scan(&user.ID, &user.Username, &user.PasswordHash)
	return user, err
}
```

Create `internal/auth/handlers.go`:

```go
package auth

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	Service *Service
}

func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	user, err := h.Service.Authenticate(input.Username, input.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "proxymorph_session",
		Value:    h.Service.SignUserID(user.ID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
```

- [ ] **Step 4: Fetch dependency and run test**

Run:

```bash
go mod tidy
go test ./internal/auth
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add go.mod go.sum internal/auth internal/storage
git commit -m "feat: add admin authentication"
```

## Task 6: HTTP App Wiring And Health Endpoint

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/routes.go`
- Create: `internal/app/app_test.go`
- Create: `internal/httpapi/respond.go`
- Create: `internal/httpapi/middleware.go`
- Modify: `cmd/proxymorph/main.go`

- [ ] **Step 1: Write failing app test**

Create `internal/app/app_test.go`:

```go
package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/liulei/proxymorph/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.Config{Addr: ":0", DataDir: t.TempDir(), SessionSecret: "test-secret", InitialAdminUsername: "admin", InitialAdminPassword: "password"}
	app, err := New(cfg, filepath.Join(cfg.DataDir, "test.db"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "ok\n" {
		t.Fatalf("unexpected response: %d %q", res.Code, res.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/app
```

Expected: FAIL because app package does not exist.

- [ ] **Step 3: Implement app and shared HTTP helpers**

Create `internal/httpapi/respond.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}
```

Create `internal/httpapi/middleware.go`:

```go
package httpapi

import "net/http"

func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			Error(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}
```

Create `internal/app/app.go`:

```go
package app

import (
	"net/http"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/config"
	"github.com/liulei/proxymorph/internal/storage"
)

type App struct {
	cfg     config.Config
	db      *storage.DB
	authSvc *auth.Service
	mux     *http.ServeMux
}

func New(cfg config.Config, dbPath string) (*App, error) {
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(db, []byte(cfg.SessionSecret))
	if err := authSvc.EnsureAdmin(cfg.InitialAdminUsername, cfg.InitialAdminPassword); err != nil {
		db.Close()
		return nil, err
	}
	a := &App{cfg: cfg, db: db, authSvc: authSvc, mux: http.NewServeMux()}
	a.routes()
	return a, nil
}

func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) Close() error {
	return a.db.Close()
}
```

Create `internal/app/routes.go`:

```go
package app

import (
	"net/http"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/httpapi"
)

func (a *App) routes() {
	authHandler := auth.Handler{Service: a.authSvc}
	a.mux.HandleFunc("/healthz", httpapi.Method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	}))
	a.mux.HandleFunc("/api/login", httpapi.Method(http.MethodPost, authHandler.Login))
}
```

Modify `cmd/proxymorph/main.go`:

```go
package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/liulei/proxymorph/internal/app"
	"github.com/liulei/proxymorph/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	application, err := app.New(cfg, filepath.Join(cfg.DataDir, "proxymorph.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close()
	log.Printf("ProxyMorph listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, application.Handler()))
}
```

- [ ] **Step 4: Run test**

Run:

```bash
go test ./internal/app
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add cmd/proxymorph internal/app internal/httpapi
git commit -m "feat: wire http application"
```

## Task 7: Task And Pinned Node APIs

**Files:**
- Create: `internal/tasks/service.go`
- Create: `internal/tasks/handlers.go`
- Create: `internal/tasks/service_test.go`
- Create: `internal/nodes/service.go`
- Create: `internal/nodes/handlers.go`
- Create: `internal/nodes/service_test.go`
- Modify: `internal/app/routes.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/tasks/service_test.go` with tests for creating and listing a conversion task:

```go
package tasks

import (
	"path/filepath"
	"testing"

	"github.com/liulei/proxymorph/internal/storage"
)

func TestCreateAndListTasks(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/clash.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.InputType != "clash" || task.OutputType != "surge6" || !task.Enabled {
		t.Fatalf("unexpected task defaults: %#v", task)
	}
	tasks, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Name != "Main" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}
```

Create `internal/nodes/service_test.go` with tests for URI import:

```go
package nodes

import (
	"path/filepath"
	"testing"

	"github.com/liulei/proxymorph/internal/storage"
)

func TestImportURIs(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	imported, err := service.ImportURIs(userID, []string{"trojan://secret@example.com:443?sni=edge.example.com#Edge"})
	if err != nil {
		t.Fatalf("ImportURIs returned error: %v", err)
	}
	if len(imported) != 1 || imported[0].Name != "Edge" || imported[0].Protocol != "trojan" {
		t.Fatalf("unexpected import: %#v", imported)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tasks ./internal/nodes
```

Expected: FAIL because services are missing.

- [ ] **Step 3: Implement services and simple handlers**

Implement `internal/tasks/service.go` with `NewService`, `Create`, and `List`. Insert tasks with defaults: `input_type='clash'`, `output_type='surge6'`, `enabled=1`, `merge_default_pinned_nodes=1`, `pinned_node_order_mode='after_remote'`.

Implement `internal/nodes/service.go` with `NewService`, `ImportURIs`, `Create`, and `List`. Use `ParseURI`, marshal node parameters and tags as JSON, and insert pinned nodes with `enabled=1`.

Implement `internal/tasks/handlers.go` and `internal/nodes/handlers.go` with JSON endpoints:

```text
GET /api/tasks
POST /api/tasks
GET /api/nodes
POST /api/nodes/import
```

For version 1, the handlers can use admin user ID `1` until auth middleware is completed in Task 10.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/tasks ./internal/nodes
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/tasks internal/nodes internal/app/routes.go
git commit -m "feat: add task and pinned node APIs"
```

## Task 8: Subscription Generation And Cache

**Files:**
- Create: `internal/subscription/service.go`
- Create: `internal/subscription/handlers.go`
- Create: `internal/subscription/service_test.go`
- Modify: `internal/app/routes.go`

- [ ] **Step 1: Write failing subscription service test**

Create `internal/subscription/service_test.go`:

```go
package subscription

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liulei/proxymorph/internal/storage"
)

func TestGenerateMergesPinnedNodes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\nrules:\n  - FINAL,Proxy\n"))
	}))
	defer upstream.Close()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskAndPinnedNode(t, db, upstream.URL)

	service := NewService(db, http.DefaultClient)
	output, err := service.GenerateByTaskID(1)
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	if !strings.Contains(output, "Remote = ss") || !strings.Contains(output, "Pinned = trojan") {
		t.Fatalf("expected remote and pinned nodes in output:\n%s", output)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/subscription
```

Expected: FAIL because subscription package does not exist.

- [ ] **Step 3: Implement generation service and handler**

Implement `internal/subscription/service.go`:

- Load task by ID.
- Fetch `source_url` using `http.Client`.
- Parse Clash with `convert.ParseClash`.
- Load effective pinned nodes: enabled default-included nodes when the task merges defaults.
- Merge remote and pinned nodes with `convert.MergeNodes`.
- Ensure generated default group contains final merged node names when upstream groups are absent.
- Render Surge 6 with `convert.RenderSurge6`.
- Upsert `output_cache`.
- Record a `conversion_runs` row.
- On failure, return cached output if it exists.

Implement `internal/subscription/handlers.go`:

```text
GET /sub/{token}
```

The handler resolves token to task ID, calls `GenerateByTaskID`, and returns `text/plain; charset=utf-8`.

- [ ] **Step 4: Run test**

Run:

```bash
go test ./internal/subscription
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/subscription internal/app/routes.go
git commit -m "feat: generate surge subscriptions"
```

## Task 9: Frontend Management UI

**Files:**
- Create: `web/package.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api.ts`
- Create: `web/src/styles.css`
- Modify: `internal/app/routes.go`
- Create: `internal/web/embed.go`

- [ ] **Step 1: Create frontend scaffold**

Create `web/package.json`:

```json
{
  "scripts": {
    "dev": "vite --host 0.0.0.0",
    "build": "vite build",
    "preview": "vite preview --host 0.0.0.0"
  },
  "dependencies": {
    "@vitejs/plugin-react": "latest",
    "vite": "latest",
    "typescript": "latest",
    "react": "latest",
    "react-dom": "latest",
    "lucide-react": "latest"
  },
  "devDependencies": {}
}
```

Create a Vite React app with:

- Login view.
- Dashboard summary.
- Task list and create form.
- Pinned node list and URI import form.
- Conversion preview area.
- Settings view with visible public base URL, cache policy text, and password-change form wired to the API added in Task 10.

Use `web/src/api.ts` for API calls and keep all fetch wrappers there.

- [ ] **Step 2: Implement app UI**

Create `web/src/App.tsx` with state for login, tasks, pinned nodes, and imports. Use lucide icons for copy, refresh, login, plus, server, key, and alert states. Keep the layout as a compact operations dashboard, with sidebar navigation and dense lists.

Create `web/src/styles.css` with a restrained palette, fixed toolbar heights, responsive grids, accessible focus states, and no nested cards.

- [ ] **Step 3: Build frontend**

Run:

```bash
cd web
npm install
npm run build
```

Expected: `web/dist` exists and the build exits with code 0.

- [ ] **Step 4: Embed frontend in Go**

Create `internal/web/embed.go`:

```go
package web

import "embed"

//go:embed dist
var Dist embed.FS
```

Modify `internal/app/routes.go` to serve embedded static assets from `internal/web.Dist` and fall back to `index.html` for frontend routes.

- [ ] **Step 5: Verify Go build**

Run:

```bash
go test ./...
go build ./cmd/proxymorph
```

Expected: both commands PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add web internal/web internal/app/routes.go
git commit -m "feat: add embedded management ui"
```

## Task 10: Auth Middleware And Protected APIs

**Files:**
- Modify: `internal/httpapi/middleware.go`
- Modify: `internal/app/routes.go`
- Modify: `internal/tasks/handlers.go`
- Modify: `internal/nodes/handlers.go`
- Create: `internal/httpapi/middleware_test.go`

- [ ] **Step 1: Write failing middleware test**

Create `internal/httpapi/middleware_test.go`:

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthRejectsMissingCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	res := httptest.NewRecorder()
	RequireAuth(func(token string) (int64, bool) { return 0, false }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})).ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
}

func TestRequireAuthAllowsValidCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.AddCookie(&http.Cookie{Name: "proxymorph_session", Value: "valid"})
	res := httptest.NewRecorder()
	called := false
	RequireAuth(func(token string) (int64, bool) { return 42, token == "valid" }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if UserID(r.Context()) != 42 {
			t.Fatalf("user id = %d, want 42", UserID(r.Context()))
		}
	})).ServeHTTP(res, req)
	if !called {
		t.Fatal("handler was not called")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/httpapi
```

Expected: FAIL because `RequireAuth` and `UserID` do not exist.

- [ ] **Step 3: Implement auth middleware**

Modify `internal/httpapi/middleware.go`:

```go
package httpapi

import (
	"context"
	"net/http"
)

type contextKey string

const userIDKey contextKey = "user_id"

func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			Error(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}

func RequireAuth(verify func(string) (int64, bool), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("proxymorph_session")
		if err != nil {
			Error(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID, ok := verify(cookie.Value)
		if !ok {
			Error(w, http.StatusUnauthorized, "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}
```

Modify routes so `/api/tasks`, `/api/nodes`, and other admin APIs use `RequireAuth(a.authSvc.VerifyToken, handler)`.

Modify task and node handlers to use `httpapi.UserID(r.Context())` instead of hard-coded user ID `1`.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/httpapi ./internal/app ./internal/tasks ./internal/nodes
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/httpapi internal/app internal/tasks internal/nodes
git commit -m "feat: protect admin APIs"
```

## Task 11: Docker, Compose, And README

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `README.md`
- Modify: `.gitignore`

- [ ] **Step 1: Create Dockerfile**

Create `Dockerfile`:

```dockerfile
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json ./
RUN npm install
COPY web ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -o /out/proxymorph ./cmd/proxymorph

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/proxymorph /app/proxymorph
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/proxymorph"]
```

- [ ] **Step 2: Create compose file**

Create `docker-compose.yml`:

```yaml
services:
  proxymorph:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - proxymorph-data:/data
    environment:
      PROXYMORPH_ADDR: ":8080"
      PROXYMORPH_PUBLIC_BASE_URL: "http://localhost:8080"
      PROXYMORPH_ADMIN_USERNAME: "admin"
      PROXYMORPH_ADMIN_PASSWORD: "change-me"
      PROXYMORPH_SESSION_SECRET: "change-this-32-byte-minimum-secret"

volumes:
  proxymorph-data:
```

- [ ] **Step 3: Create README**

Create `README.md` with:

- What ProxyMorph does.
- Current scope: Clash to Surge 6, pinned nodes, admin UI.
- Docker Compose quick start.
- Environment variable table.
- Security note recommending HTTPS behind a reverse proxy and changing default credentials.
- Development commands: `go test ./...`, `cd web && npm install && npm run dev`.

- [ ] **Step 4: Verify Docker build**

Run:

```bash
docker build -t proxymorph:dev .
```

Expected: image builds successfully.

- [ ] **Step 5: Commit**

Run:

```bash
git add Dockerfile docker-compose.yml README.md .gitignore
git commit -m "chore: add docker deployment docs"
```

## Task 12: End-To-End Verification And Polish

**Files:**
- Modify only files connected to failed verification output from Tasks 1-11.

- [ ] **Step 1: Run backend tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend build**

Run:

```bash
cd web
npm run build
```

Expected: PASS and `web/dist` exists.

- [ ] **Step 3: Run Go build**

Run:

```bash
go build ./cmd/proxymorph
```

Expected: PASS.

- [ ] **Step 4: Run local server smoke test**

Run:

```bash
PROXYMORPH_DATA_DIR=/tmp/proxymorph-dev PROXYMORPH_ADMIN_PASSWORD=password PROXYMORPH_SESSION_SECRET=local-development-secret-32-bytes go run ./cmd/proxymorph
```

In another terminal, run:

```bash
curl -i http://localhost:8080/healthz
```

Expected: HTTP 200 with body `ok`.

- [ ] **Step 5: Verify browser UI**

Open `http://localhost:8080` in the Codex in-app browser. Verify:

- Login form renders.
- Login succeeds with configured credentials.
- Dashboard renders.
- Pinned node import form is visible.
- Task creation form is visible.
- Text does not overlap at desktop or mobile widths.

- [ ] **Step 6: Commit verification fixes**

Run:

```bash
git add .
git commit -m "fix: polish mvp verification issues"
```

Only create this commit if verification required changes.

## Self-Review

Spec coverage:

- Go single binary: covered by Tasks 1, 6, 9, and 12.
- Docker deployment: covered by Task 11.
- SQLite storage: covered by Task 2.
- Admin username/password login: covered by Tasks 5 and 10.
- Clash to Surge 6 conversion: covered by Task 4.
- Pinned node library: covered by Tasks 3, 7, and 8.
- URI import and form editing: URI import is covered by Tasks 3 and 7; form editing is covered by Task 9 UI and Task 7 CRUD APIs.
- Task-level pinned-node merge controls: covered by Tasks 7 and 8.
- Cache and fallback behavior: covered by Task 8.
- Management UI: covered by Task 9.
- Error visibility: covered by Tasks 8 and 9.
- Docker smoke and end-to-end verification: covered by Tasks 11 and 12.

Red-flag scan:

- This plan defines exact files, required functions, routes, commands, and expected behavior for each implementation area.

Type consistency:

- Internal proxy nodes use `convert.Node`.
- Task merge mode uses `PinnedNodeOrderMode` in storage and `MergeOptions.Mode` in conversion.
- Admin user context uses `httpapi.UserID`.
