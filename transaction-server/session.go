package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

const defaultLoginSessionTTL = 30 * time.Minute

var (
	errSessionNotFound = errors.New("session not found")
	errSessionExpired  = errors.New("session expired")
	errSessionMismatch = errors.New("session user mismatch")

	loginSessions = newSessionStore(defaultLoginSessionTTL)
)

type LoginSession struct {
	Token        string    `json:"token"`
	UserDID      string    `json:"userDID"`
	ExpiresAt    time.Time `json:"expiresAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
}

type sessionStore struct {
	mu       sync.Mutex
	ttl      time.Duration
	sessions map[string]LoginSession
}

func newSessionStore(ttl time.Duration) *sessionStore {
	return &sessionStore{
		ttl:      ttl,
		sessions: map[string]LoginSession{},
	}
}

func (s *sessionStore) Create(userDID string, now time.Time) (LoginSession, error) {
	token, err := newSessionToken()
	if err != nil {
		return LoginSession{}, err
	}

	now = now.UTC()
	session := LoginSession{
		Token:        token,
		UserDID:      userDID,
		ExpiresAt:    now.Add(s.ttl),
		LastActiveAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)
	s.sessions[token] = session

	return session, nil
}

func (s *sessionStore) Validate(token string, userDID string, now time.Time) (LoginSession, error) {
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	session, ok := s.sessions[token]
	if !ok {
		return LoginSession{}, errSessionNotFound
	}
	if !now.Before(session.ExpiresAt) {
		delete(s.sessions, token)
		return LoginSession{}, errSessionExpired
	}
	if session.UserDID != userDID {
		return LoginSession{}, errSessionMismatch
	}

	session.LastActiveAt = now
	s.sessions[token] = session

	return session, nil
}

func (s *sessionStore) RevokeUser(userDID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for token, session := range s.sessions {
		if session.UserDID == userDID {
			delete(s.sessions, token)
		}
	}
}

func (s *sessionStore) cleanupExpiredLocked(now time.Time) {
	for token, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}

func newSessionToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
