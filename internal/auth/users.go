package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
}

// UserStore simple en memoria — en v0.3 mover a DB
type UserStore struct {
	users map[string]*User
}

func NewUserStore() *UserStore {
	return &UserStore{users: make(map[string]*User)}
}

func (s *UserStore) Add(id, username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s.users[username] = &User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
	}
	return nil
}

func (s *UserStore) Authenticate(username, password string) (*User, error) {
	user, ok := s.users[username]
	if !ok {
		return nil, errors.New("usuario no encontrado")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("contraseña incorrecta")
	}
	return user, nil
}

func (s *UserStore) Get(username string) (*User, bool) {
	u, ok := s.users[username]
	return u, ok
}
