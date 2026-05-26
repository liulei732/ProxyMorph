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

func TestEnsureAdminUpdatesConfiguredAdminPassword(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "old-password"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	if err := service.EnsureAdmin("admin", "new-password"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "new-password"); err != nil {
		t.Fatalf("Authenticate with new password returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "old-password"); err == nil {
		t.Fatal("expected old password to stop working")
	}
}

func TestEnsureAdminDoesNotResetExistingPasswordWhenPasswordUnset(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "custom-password"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	if err := service.EnsureAdmin("admin", ""); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "custom-password"); err != nil {
		t.Fatalf("Authenticate with existing password returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "admin"); err == nil {
		t.Fatal("expected default password to remain invalid")
	}
}
