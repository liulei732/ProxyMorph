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

func TestImportURIsAcceptsSurgeProxyLines(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	imported, err := service.ImportURIs(userID, []string{"香港 03 AnyTLS = anytls, at03-hlzp2o.fork2026.com, 18611, password=secret, sni=www.baidu.com, skip-cert-verify=true"})
	if err != nil {
		t.Fatalf("ImportURIs returned error: %v", err)
	}
	if len(imported) != 1 || imported[0].Name != "香港 03 AnyTLS" || imported[0].Protocol != "anytls" || imported[0].Server != "at03-hlzp2o.fork2026.com" || imported[0].Port != 18611 {
		t.Fatalf("unexpected import: %#v", imported)
	}
	if imported[0].ParametersJSON == "" {
		t.Fatalf("expected parameters json: %#v", imported[0])
	}
}

func TestDeleteNodeRemovesOnlyUsersNode(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	res, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES ('other', 'hash')`)
	if err != nil {
		t.Fatal(err)
	}
	otherUserID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	node, err := service.Create(userID, CreateInput{Name: "Edge", Protocol: "trojan", Server: "edge.example.com", Port: 443, Params: map[string]string{"password": "secret"}, Enabled: true})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := service.Delete(otherUserID, node.ID); err == nil {
		t.Fatal("Delete should reject nodes owned by another user")
	}
	if err := service.Delete(userID, node.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	nodes, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("node should be deleted: %#v", nodes)
	}
}

func TestBatchUpdateEnableDisableAndDeleteOnlyUsersNodes(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	res, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES ('other', 'hash')`)
	if err != nil {
		t.Fatal(err)
	}
	otherUserID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	first, err := service.Create(userID, CreateInput{Name: "A", Protocol: "trojan", Server: "a.example.com", Port: 443, Params: map[string]string{"password": "secret"}, Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(userID, CreateInput{Name: "B", Protocol: "trojan", Server: "b.example.com", Port: 443, Params: map[string]string{"password": "secret"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.Create(otherUserID, CreateInput{Name: "Other", Protocol: "trojan", Server: "other.example.com", Port: 443, Params: map[string]string{"password": "secret"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	affected, err := service.Batch(userID, BatchInput{Action: "enable", IDs: []int64{first.ID, other.ID}})
	if err != nil {
		t.Fatalf("Batch enable returned error: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	nodes, err := service.List(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !findNode(t, nodes, first.ID).Enabled {
		t.Fatalf("first node should be enabled: %#v", nodes)
	}

	affected, err = service.Batch(userID, BatchInput{Action: "disable", IDs: []int64{first.ID, second.ID}})
	if err != nil {
		t.Fatalf("Batch disable returned error: %v", err)
	}
	if affected != 2 {
		t.Fatalf("affected = %d, want 2", affected)
	}
	nodes, err = service.List(userID)
	if err != nil {
		t.Fatal(err)
	}
	if findNode(t, nodes, first.ID).Enabled || findNode(t, nodes, second.ID).Enabled {
		t.Fatalf("selected nodes should be disabled: %#v", nodes)
	}

	affected, err = service.Batch(userID, BatchInput{Action: "delete", IDs: []int64{first.ID, other.ID}})
	if err != nil {
		t.Fatalf("Batch delete returned error: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	nodes, err = service.List(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != second.ID {
		t.Fatalf("only second user's own node should remain: %#v", nodes)
	}
	otherNodes, err := service.List(otherUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherNodes) != 1 || otherNodes[0].ID != other.ID {
		t.Fatalf("other user's node should remain: %#v", otherNodes)
	}
}

func TestBatchRejectsInvalidInput(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	if _, err := service.Batch(userID, BatchInput{Action: "archive", IDs: []int64{1}}); err == nil {
		t.Fatal("Batch should reject unknown actions")
	}
	if _, err := service.Batch(userID, BatchInput{Action: "enable"}); err == nil {
		t.Fatal("Batch should reject empty ids")
	}
}

func TestListNodesReturnsEmptySlice(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	nodes, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if nodes == nil {
		t.Fatal("List returned nil; want empty slice")
	}
	if len(nodes) != 0 {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
}

func findNode(t *testing.T, nodes []storage.PinnedNode, id int64) storage.PinnedNode {
	t.Helper()
	for _, node := range nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("node %d not found in %#v", id, nodes)
	return storage.PinnedNode{}
}

func seedUser(t *testing.T, db *storage.DB) int64 {
	t.Helper()
	res, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash')`)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
