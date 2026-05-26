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
	passwordConfigured := password != ""
	var count int
	if err := s.db.SQL().QueryRow(`SELECT count(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		if !passwordConfigured {
			return nil
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
			return nil
		}
	}
	if !passwordConfigured {
		password = "admin"
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
