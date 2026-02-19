package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/tasks/v1"
)

// OAuth2 configuration
func getOAuthConfig(credentialsFile string) (*oauth2.Config, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, tasks.TasksScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %v", err)
	}

	return config, nil
}

// getTokenFromFile reads token from file
func getTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves token to file
func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to save token: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// getClient returns an HTTP client with OAuth2 token
func getClient(config *oauth2.Config, tokenFile string) (*http.Client, error) {
	tok, err := getTokenFromFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("token not found, run with --auth first: %v", err)
	}

	// Token auto-refresh
	tokenSource := config.TokenSource(context.Background(), tok)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %v", err)
	}

	// Save refreshed token if it changed
	if newToken.AccessToken != tok.AccessToken {
		if err := saveToken(tokenFile, newToken); err != nil {
			// Non-fatal, just log
			fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token: %v\n", err)
		}
	}

	return config.Client(context.Background(), newToken), nil
}

// runAuthFlow performs OAuth2 authorization flow (manual code entry for headless servers)
func runAuthFlow(credentialsFile, tokenFile string) error {
	config, err := getOAuthConfig(credentialsFile)
	if err != nil {
		return err
	}

	// Use OOB redirect for manual code entry
	config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	// Generate auth URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("AUTH_URL:%s\n", authURL)

	return nil
}

// exchangeCode exchanges authorization code for token
func exchangeCode(credentialsFile, tokenFile, code string) error {
	config, err := getOAuthConfig(credentialsFile)
	if err != nil {
		return err
	}

	config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	// Exchange code for token
	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("unable to exchange code for token: %v", err)
	}

	// Save token
	if err := saveToken(tokenFile, tok); err != nil {
		return err
	}

	fmt.Printf("Token saved to: %s\n", tokenFile)
	return nil
}
