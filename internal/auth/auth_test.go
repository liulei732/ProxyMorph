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

func TestEnsureAdminOnlyAppliesChangedConfiguredPasswordOnce(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "compose-password-a"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	user, err := service.Authenticate("admin", "compose-password-a")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if err := service.ChangePassword(user.ID, "compose-password-a", "user-password-b"); err != nil {
		t.Fatalf("ChangePassword returned error: %v", err)
	}

	if err := service.EnsureAdmin("admin", "compose-password-a"); err != nil {
		t.Fatalf("EnsureAdmin after unchanged compose password returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "user-password-b"); err != nil {
		t.Fatalf("unchanged compose password should preserve user password: %v", err)
	}
	if _, err := service.Authenticate("admin", "compose-password-a"); err == nil {
		t.Fatal("unchanged compose password should not be re-applied")
	}

	if err := service.EnsureAdmin("admin", "compose-password-c"); err != nil {
		t.Fatalf("EnsureAdmin after changed compose password returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "compose-password-c"); err != nil {
		t.Fatalf("changed compose password should be applied: %v", err)
	}
	if _, err := service.Authenticate("admin", "user-password-b"); err == nil {
		t.Fatal("expected previous user password to stop working after compose password changed")
	}

	if err := service.ChangePassword(user.ID, "compose-password-c", "user-password-d"); err != nil {
		t.Fatalf("second ChangePassword returned error: %v", err)
	}
	if err := service.EnsureAdmin("admin", "compose-password-c"); err != nil {
		t.Fatalf("EnsureAdmin after unchanged second compose password returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "user-password-d"); err != nil {
		t.Fatalf("unchanged second compose password should preserve user password: %v", err)
	}
}

func TestEnsureAdminPreservesModifiedPasswordWhenLegacyDigestMissing(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "compose-password-a"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	user, err := service.Authenticate("admin", "compose-password-a")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if err := service.ChangePassword(user.ID, "compose-password-a", "user-password-b"); err != nil {
		t.Fatalf("ChangePassword returned error: %v", err)
	}
	if _, err := db.SQL().Exec(`DELETE FROM app_settings WHERE key = ?`, adminPasswordSourceDigestSettingKey); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL().Exec(`UPDATE users SET created_at = '2026-01-01 00:00:00', updated_at = '2026-01-02 00:00:00' WHERE username = 'admin'`); err != nil {
		t.Fatal(err)
	}

	if err := service.EnsureAdmin("admin", "compose-password-a"); err != nil {
		t.Fatalf("EnsureAdmin with legacy missing digest returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "user-password-b"); err != nil {
		t.Fatalf("legacy missing digest should preserve modified user password: %v", err)
	}
	if _, err := service.Authenticate("admin", "compose-password-a"); err == nil {
		t.Fatal("legacy missing digest should not re-apply unchanged compose password")
	}
	if err := service.EnsureAdmin("admin", "compose-password-c"); err != nil {
		t.Fatalf("EnsureAdmin with changed compose password returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "compose-password-c"); err != nil {
		t.Fatalf("changed compose password should still be applied after legacy digest is recorded: %v", err)
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

func TestChangePasswordRequiresCurrentPasswordAndUpdatesHash(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "old-password"); err != nil {
		t.Fatalf("EnsureAdmin returned error: %v", err)
	}
	user, err := service.Authenticate("admin", "old-password")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if err := service.ChangePassword(user.ID, "wrong-password", "new-password"); err != ErrInvalidCredentials {
		t.Fatalf("ChangePassword wrong current error = %v, want ErrInvalidCredentials", err)
	}
	if err := service.ChangePassword(user.ID, "old-password", "short"); err != ErrPasswordTooShort {
		t.Fatalf("ChangePassword short password error = %v, want ErrPasswordTooShort", err)
	}
	if err := service.ChangePassword(user.ID, "old-password", "new-password"); err != nil {
		t.Fatalf("ChangePassword returned error: %v", err)
	}
	if _, err := service.Authenticate("admin", "old-password"); err == nil {
		t.Fatal("expected old password to stop working")
	}
	if _, err := service.Authenticate("admin", "new-password"); err != nil {
		t.Fatalf("Authenticate with new password returned error: %v", err)
	}
}
