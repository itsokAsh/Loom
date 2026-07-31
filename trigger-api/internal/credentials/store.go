package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
	gcm  cipher.AEAD
}

type Credential struct {
	ID        pgtype.UUID        `json:"id"`
	Name      string             `json:"name"`
	Type      string             `json:"type"`
	CreatedAt pgtype.Timestamptz `json:"createdAt"`
	UpdatedAt pgtype.Timestamptz `json:"updatedAt"`
}

type sendgridPayload struct {
	APIKey string `json:"apiKey"`
}

func NewStore(pool *pgxpool.Pool) (*Store, error) {
	key := os.Getenv("CREDENTIALS_ENCRYPTION_KEY")
	if key == "" {
		key = "loom-dev-credentials-key-change-me"
	}
	hash := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool, gcm: gcm}, nil
}

func (s *Store) encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.gcm.Seal(nonce, nonce, plain, nil), nil
}

func (s *Store) decrypt(blob []byte) ([]byte, error) {
	ns := s.gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return s.gcm.Open(nil, blob[:ns], blob[ns:], nil)
}

func (s *Store) List(ctx context.Context) ([]Credential, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, type, created_at, updated_at FROM credentials ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Credential
	for rows.Next() {
		var c Credential
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []Credential{}
	}
	return out, rows.Err()
}

func (s *Store) CreateSendGrid(ctx context.Context, name, apiKey string) (Credential, error) {
	if name == "" {
		name = "SendGrid"
	}
	if apiKey == "" {
		return Credential{}, errors.New("apiKey is required")
	}
	plain, err := json.Marshal(sendgridPayload{APIKey: apiKey})
	if err != nil {
		return Credential{}, err
	}
	enc, err := s.encrypt(plain)
	if err != nil {
		return Credential{}, err
	}
	var c Credential
	err = s.pool.QueryRow(ctx, `
		INSERT INTO credentials (name, type, encrypted_payload)
		VALUES ($1, 'sendgrid', $2)
		RETURNING id, name, type, created_at, updated_at
	`, name, enc).Scan(&c.ID, &c.Name, &c.Type, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) Delete(ctx context.Context, id pgtype.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM credentials WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) GetSendGridAPIKey(ctx context.Context, id pgtype.UUID) (string, error) {
	var blob []byte
	var typ string
	err := s.pool.QueryRow(ctx, `SELECT type, encrypted_payload FROM credentials WHERE id = $1`, id).Scan(&typ, &blob)
	if err != nil {
		return "", err
	}
	if typ != "sendgrid" {
		return "", fmt.Errorf("credential type is %s, not sendgrid", typ)
	}
	plain, err := s.decrypt(blob)
	if err != nil {
		return "", err
	}
	var p sendgridPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return "", err
	}
	if p.APIKey == "" {
		return "", errors.New("empty api key")
	}
	return p.APIKey, nil
}

func UUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	return fmt.Sprintf(
		"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15],
	)
}

func ParseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(s)
	return id, err
}

// FormatTime helper for JSON responses if needed
func FormatTime(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}
