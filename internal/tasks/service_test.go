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

func TestListTasksReturnsEmptySlice(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	tasks, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if tasks == nil {
		t.Fatal("List returned nil; want empty slice")
	}
	if len(tasks) != 0 {
		t.Fatalf("unexpected tasks: %#v", tasks)
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
