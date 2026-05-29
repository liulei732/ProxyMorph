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
var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

const adminPasswordSourceDigestSettingKey = "admin_password_source_digest"

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
	passwordConfigured := password != ""
	var count int
	if err := s.db.SQL().QueryRow(`SELECT count(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		if !passwordConfigured {
			return nil
		}
		digest := passwordSourceDigest(username, password)
		known, err := s.setting(adminPasswordSourceDigestSettingKey)
		if err != nil {
			return err
		}
		if known == digest {
			return nil
		}
		if known == "" {
			modified, err := s.userPasswordWasModified(username)
			if err != nil {
				return err
			}
			if modified {
				return s.setSetting(adminPasswordSourceDigestSettingKey, digest)
			}
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		res, err := s.db.SQL().Exec(`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE username = ?`, string(hash), username)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected > 0 {
			return s.setSetting(adminPasswordSourceDigestSettingKey, digest)
		}
	}
	if !passwordConfigured {
		password = "admin"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := s.db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, string(hash)); err != nil {
		return err
	}
	if passwordConfigured {
		return s.setSetting(adminPasswordSourceDigestSettingKey, passwordSourceDigest(username, password))
	}
	return nil
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

func (s *Service) ChangePassword(userID int64, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}
	user, err := s.UserByID(context.Background(), userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.SQL().Exec(`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(hash), userID)
	return err
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

func passwordSourceDigest(username, password string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + password))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *Service) setting(key string) (string, error) {
	var value string
	err := s.db.SQL().QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *Service) setSetting(key, value string) error {
	_, err := s.db.SQL().Exec(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

func (s *Service) userPasswordWasModified(username string) (bool, error) {
	var createdAt, updatedAt string
	err := s.db.SQL().QueryRow(`SELECT created_at, updated_at FROM users WHERE username = ?`, username).Scan(&createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(createdAt) != strings.TrimSpace(updatedAt), nil
}
