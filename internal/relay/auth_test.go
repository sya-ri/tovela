package relay

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func TestAuthenticatedClientReusesValidToken(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	config := DefaultConfig("google-client", "")
	_, key, err := config.oauth("google")
	if err != nil {
		t.Fatal(err)
	}
	token := oauth2.Token{AccessToken: "saved", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}
	if err := writeCredential(key, token); err != nil {
		t.Fatal(err)
	}
	client, err := authenticatedClient(t.Context(), config, "google")
	if err != nil {
		t.Fatal(err)
	}
	if client.CheckRedirect == nil || !errors.Is(client.CheckRedirect(nil, nil), http.ErrUseLastResponse) {
		t.Fatal("authenticated requests must not follow redirects")
	}
	transport := client.Transport.(*oauth2.Transport)
	transport.Base = transportFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer saved" {
			t.Errorf("Authorization = %q, want Bearer saved", got)
		}
		return reply(http.StatusOK, "{}")
	})

	// The credential store is only needed again when the cached token expires.
	keyring.MockInit()
	for range 2 {
		var result struct{}
		if err := request(t.Context(), client, http.MethodGet, "https://example.test", nil, &result); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSavedTokensReuseRefreshAcrossClients(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	const key = "test-provider"
	stale := oauth2.Token{AccessToken: "expired", RefreshToken: "old-refresh", Expiry: time.Now().Add(-time.Minute)}
	if err := writeCredential(key, stale); err != nil {
		t.Fatal(err)
	}
	refreshes := 0
	client := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		refreshes++
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("grant_type") != "refresh_token" || req.Form.Get("refresh_token") != stale.RefreshToken {
			t.Fatalf("unexpected token refresh request")
		}
		res, err := reply(http.StatusOK, `{"access_token":"current","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
		res.Header = http.Header{"Content-Type": {"application/json"}}
		return res, err
	})}
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, client)
	config := &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{TokenURL: "https://example.test/token", AuthStyle: oauth2.AuthStyleInParams},
	}
	for range 2 {
		client := oauth2.ReuseTokenSource(&stale, &savedTokens{ctx: ctx, cfg: config, key: key})
		token, err := client.Token()
		if err != nil || token.AccessToken != "current" {
			t.Fatalf("refreshed token = %v, %v", token, err)
		}
	}
	if refreshes != 1 {
		t.Fatalf("made %d refresh requests, want 1", refreshes)
	}
	var saved oauth2.Token
	if err := readCredential(key, &saved); err != nil || saved.RefreshToken != "new-refresh" {
		t.Fatalf("did not persist the rotated refresh token: %v", err)
	}
}
