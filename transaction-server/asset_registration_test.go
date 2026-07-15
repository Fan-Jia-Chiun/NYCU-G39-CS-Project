package main

import (
	"errors"
	"testing"
	"time"
)

func TestBuildRegisterAssetCredential(t *testing.T) {
	got := buildRegisterAssetCredential(
		"did:nycu-g39:identity:abc",
		"House",
		"Taipei",
		"",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"2026-07-15T08:30:00Z",
	)
	want := "REGISTER_ASSET|did:nycu-g39:identity:abc|House|Taipei||0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef|2026-07-15T08:30:00Z"
	if got != want {
		t.Fatalf("credential = %q, want %q", got, want)
	}
}

func TestValidateAssetRegistrationFieldsRejectsSeparator(t *testing.T) {
	err := validateAssetRegistrationFields(assetRegistrationForm{
		SessionToken:  "session",
		UserDID:       "did:nycu-g39:identity:abc",
		AssetName:     "House|Bad",
		AssetLocation: "Taipei",
		Timestamp:     "2026-07-15T08:30:00Z",
		PhotoHash:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Signature:     "sig",
	})
	if err == nil {
		t.Fatal("expected separator validation error")
	}
}

func TestValidateAssetRegistrationFieldsAllowsEmptyDescription(t *testing.T) {
	err := validateAssetRegistrationFields(assetRegistrationForm{
		SessionToken:  "session",
		UserDID:       "did:nycu-g39:identity:abc",
		AssetName:     "House",
		AssetLocation: "Taipei",
		Description:   "",
		Timestamp:     "2026-07-15T08:30:00Z",
		PhotoHash:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Signature:     "sig",
	})
	if err != nil {
		t.Fatalf("validateAssetRegistrationFields() error = %v", err)
	}
}

func TestSessionStoreValidateAndRevoke(t *testing.T) {
	store := newSessionStore(time.Minute)
	now := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	session, err := store.Create("did:nycu-g39:identity:abc", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := store.Validate(session.Token, "did:nycu-g39:identity:abc", now.Add(10*time.Second)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if _, err := store.Validate(session.Token, "did:nycu-g39:identity:other", now.Add(10*time.Second)); !errors.Is(err, errSessionMismatch) {
		t.Fatalf("Validate() error = %v, want %v", err, errSessionMismatch)
	}

	store.RevokeUser("did:nycu-g39:identity:abc")
	if _, err := store.Validate(session.Token, "did:nycu-g39:identity:abc", now.Add(20*time.Second)); !errors.Is(err, errSessionNotFound) {
		t.Fatalf("Validate() error = %v, want %v", err, errSessionNotFound)
	}
}

func TestSessionStoreExpires(t *testing.T) {
	store := newSessionStore(time.Second)
	now := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	session, err := store.Create("did:nycu-g39:identity:abc", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := store.Validate(session.Token, "did:nycu-g39:identity:abc", now.Add(2*time.Second)); !errors.Is(err, errSessionNotFound) {
		t.Fatalf("Validate() error = %v, want expired session cleanup to remove token", err)
	}
}
