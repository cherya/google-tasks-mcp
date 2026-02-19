package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestSaveAndReadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	tok := &oauth2.Token{
		AccessToken:  "access-123",
		TokenType:    "Bearer",
		RefreshToken: "refresh-456",
		Expiry:       time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	if err := saveToken(path, tok); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Read back
	got, err := getTokenFromFile(path)
	if err != nil {
		t.Fatalf("getTokenFromFile failed: %v", err)
	}

	if got.AccessToken != tok.AccessToken {
		t.Errorf("access token: expected %q, got %q", tok.AccessToken, got.AccessToken)
	}
	if got.RefreshToken != tok.RefreshToken {
		t.Errorf("refresh token: expected %q, got %q", tok.RefreshToken, got.RefreshToken)
	}
	if got.TokenType != tok.TokenType {
		t.Errorf("token type: expected %q, got %q", tok.TokenType, got.TokenType)
	}
}

func TestGetTokenFromFile_NotFound(t *testing.T) {
	_, err := getTokenFromFile("/nonexistent/path/token.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
