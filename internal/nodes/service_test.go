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
