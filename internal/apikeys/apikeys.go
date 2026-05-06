package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/johandavid77/btrfs-snapah-pow/internal/storage"
)

type Store struct {
	db *storage.DB
}

func NewStore(db *storage.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Generate(name, role string, expiresDays int) (plaintext string, rec *storage.APIKeyRecord, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generar bytes: %w", err)
	}

	plaintext = "spow_" + hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(plaintext))

	rec = &storage.APIKeyRecord{
		ID:        hex.EncodeToString(hash[:8]),
		Name:      name,
		KeyHash:   hex.EncodeToString(hash[:]),
		Prefix:    plaintext[:12],
		Role:      role,
		Active:    true,
		CreatedAt: time.Now(),
	}

	if expiresDays > 0 {
		exp := time.Now().AddDate(0, 0, expiresDays)
		rec.ExpiresAt = &exp
	}

	if err = s.db.SaveAPIKey(rec); err != nil {
		return "", nil, fmt.Errorf("guardar key: %w", err)
	}
	return plaintext, rec, nil
}

func (s *Store) Validate(plaintext string) (*storage.APIKeyRecord, bool) {
	hash := sha256.Sum256([]byte(plaintext))
	hashStr := hex.EncodeToString(hash[:])

	k, err := s.db.GetAPIKeyByHash(hashStr)
	if err != nil {
		return nil, false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return nil, false
	}
	s.db.UpdateAPIKeyLastUsed(k.ID, time.Now())
	return k, true
}

func (s *Store) List() ([]storage.APIKeyRecord, error) {
	return s.db.ListAPIKeys()
}

func (s *Store) Revoke(id string) bool {
	return s.db.RevokeAPIKey(id) == nil
}
