package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// errNotHuman means Cloudflare looked at the token and rejected it. Any other
// error from Verify means the check could not be completed at all.
var errNotHuman = errors.New("turnstile rejected the token")

// humanVerifier checks a bot-check token before a code is created.
type humanVerifier interface {
	Verify(ctx context.Context, token string) error
}

// turnstileVerifier validates Cloudflare Turnstile tokens server-side. Tokens
// are single-use and expire after five minutes; Cloudflare enforces both.
type turnstileVerifier struct {
	secret string
	// hostname, when set, must match the site the token was issued on, so a
	// token solved on someone else's page cannot be replayed here. Empty skips
	// the check, which Cloudflare's test keys need: they report example.com.
	hostname  string
	verifyURL string
	client    *http.Client
}

func newTurnstileVerifier(secret, hostname string) *turnstileVerifier {
	return &turnstileVerifier{
		secret:    secret,
		hostname:  hostname,
		verifyURL: turnstileVerifyURL,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (v *turnstileVerifier) Verify(ctx context.Context, token string) error {
	if token == "" {
		return errNotHuman
	}
	form := url.Values{"secret": {v.secret}, "response": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile verify: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile verify: status %d", resp.StatusCode)
	}

	var body struct {
		Success  bool   `json:"success"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("turnstile verify: decode: %w", err)
	}
	if !body.Success || (v.hostname != "" && body.Hostname != v.hostname) {
		return errNotHuman
	}
	return nil
}
