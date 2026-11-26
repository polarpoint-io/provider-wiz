package wizapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"golang.org/x/time/rate"
)

func TestConfigFromCredentials(t *testing.T) {
	_, err := ConfigFromCredentials(map[string]any{KeyURL: "https://x", KeyClientID: "id"})
	if err == nil {
		t.Fatal("want error for missing client secret")
	}
	c, err := ConfigFromCredentials(map[string]any{KeyURL: "https://x", KeyClientID: "id", KeyClientSecret: "s", KeyRetryMax: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthURL != DefaultAuthURL || c.Audience != DefaultAudience || c.RetryMax != 2 {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestClientAuthRetryAndNotFound(t *testing.T) {
	// Keep the shared limiter out of the way in tests.
	limiter = rate.NewLimiter(rate.Inf, 1)

	var tokens, calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		tokens.Add(1)
		body, _ := io.ReadAll(r.Body)
		v, _ := url.ParseQuery(string(body))
		if v.Get("grant_type") != "client_credentials" || v.Get("audience") != "wiz-api" ||
			v.Get("client_id") != "id" || v.Get("client_secret") != "secret" {
			http.Error(w, "bad token request: "+string(body), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// First call is throttled; the client must retry.
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		var req struct {
			OperationName string `json:"operationName"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.OperationName != "GetProject" {
			t.Errorf("unexpected operation %q", req.OperationName)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":null},"errors":[{"message":"Resource not found","extensions":{"code":"NOT_FOUND"}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(context.Background(), Config{
		URL: srv.URL + "/graphql", AuthURL: srv.URL + "/oauth/token",
		ClientID: "id", ClientSecret: "secret", Audience: "wiz-api", RetryMax: 3,
	})
	_, err := GetProject(context.Background(), c, "p1")
	if !IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("want 2 GraphQL calls (one retry), got %d", calls.Load())
	}
	if tokens.Load() != 1 {
		t.Errorf("want 1 token request, got %d", tokens.Load())
	}
}
