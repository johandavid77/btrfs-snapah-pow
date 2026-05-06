package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type APIKey struct {
	ID          string
	Name        string
	KeyHash     string
	Prefix      string
	Role        string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	Active      bool
}

type Store struct {
	mu   sync.RWMutex
	keys map[string]*APIKey
}

func NewStore() *Store {
	return &Store{keys: make(map[string]*APIKey)}
}

// Generate crea una nueva API key, retorna el token en claro (solo una vez)
func (s *Store) Generate(name, role string, expiresDays int) (plaintext string, key *APIKey, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generar bytes: %w", err)
	}

	plaintext = "spow_" + hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(plaintext))
	prefix := plaintext[:12]

	key = &APIKey{
		ID:        hex.EncodeToString(hash[:8]),
		Name:      name,
		KeyHash:   hex.EncodeToString(hash[:]),
		Prefix:    prefix,
		Role:      role,
		CreatedAt: time.Now(),
		Active:    true,
	}

	if expiresDays > 0 {
		exp := time.Now().AddDate(0, 0, expiresDays)
		key.ExpiresAt = &exp
	}

	s.mu.Lock()
	s.keys[key.ID] = key
	s.mu.Unlock()

	return plaintext, key, nil
}

// Validate verifica una API key y retorna el rol si es valida
func (s *Store) Validate(plaintext string) (*APIKey, bool) {
	hash := sha256.Sum256([]byte(plaintext))
	hashStr := hex.EncodeToString(hash[:])

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, k := range s.keys {
		if k.KeyHash == hashStr && k.Active {
			if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
				return nil, false
			}
			now := time.Now()
			k.LastUsedAt = &now
			return k, true
		}
	}
	return nil, false
}

func (s *Store) List() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]*APIKey, 0, len(s.keys))
	for _, k := range s.keys {
		keys = append(keys, k)
	}
	return keys
}

func (s *Store) Revoke(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k, ok := s.keys[id]; ok {
		k.Active = false
		return true
	}
	return false
}
