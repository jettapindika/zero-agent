package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UserInfo mirrors the relevant subset of Google's OIDC userinfo response.
type UserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// FetchUserInfo calls Google's OIDC userinfo endpoint with the bearer access
// token. Email must be present and verified or the call is rejected — Google
// returns an unverified email for some Workspace edge cases and we refuse to
// trust those.
func FetchUserInfo(ctx context.Context, client *http.Client, accessToken string) (*UserInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if accessToken == "" {
		return nil, errors.New("access token required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	if info.Sub == "" {
		return nil, errors.New("userinfo missing sub")
	}
	if info.Email == "" {
		return nil, errors.New("userinfo missing email")
	}
	if !info.EmailVerified {
		return nil, errors.New("email is not verified by Google")
	}
	return &info, nil
}
