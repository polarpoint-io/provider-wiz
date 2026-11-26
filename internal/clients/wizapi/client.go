package wizapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
)

// Credential keys read from a ProviderConfig secret. They match the keys the
// Upjet (Terraform) controllers use, so one ProviderConfig serves both.
const (
	KeyURL          = "wiz_url"
	KeyClientID     = "wiz_auth_client_id"
	KeyClientSecret = "wiz_auth_client_secret"
	KeyAuthURL      = "wiz_auth_url"
	KeyAudience     = "wiz_auth_audience"
	KeyRetryMax     = "http_client_retry_max"

	DefaultAuthURL  = "https://auth.app.wiz.io/oauth/token"
	DefaultAudience = "wiz-api"

	defaultRetryMax = 5
	userAgent       = "provider-wiz"
)

// Config holds what is needed to talk to one Wiz tenant.
type Config struct {
	// URL is the tenant's GraphQL endpoint, https://api.<dc>.app.wiz.io/graphql.
	URL          string
	ClientID     string
	ClientSecret string
	AuthURL      string
	Audience     string
	RetryMax     int
}

// ConfigFromCredentials builds a Config from the decoded ProviderConfig
// credentials secret.
func ConfigFromCredentials(creds map[string]any) (Config, error) {
	str := func(k string) string {
		if v, ok := creds[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	c := Config{
		URL:          str(KeyURL),
		ClientID:     str(KeyClientID),
		ClientSecret: str(KeyClientSecret),
		AuthURL:      str(KeyAuthURL),
		Audience:     str(KeyAudience),
		RetryMax:     defaultRetryMax,
	}
	for k, v := range map[string]string{KeyURL: c.URL, KeyClientID: c.ClientID, KeyClientSecret: c.ClientSecret} {
		if v == "" {
			return Config{}, fmt.Errorf("credentials are missing required key %q", k)
		}
	}
	if c.AuthURL == "" {
		c.AuthURL = DefaultAuthURL
	}
	if c.Audience == "" {
		c.Audience = DefaultAudience
	}
	if s := str(KeyRetryMax); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("%s must be a non-negative integer", KeyRetryMax)
		}
		c.RetryMax = n
	}
	return c, nil
}

// limiter is shared by every client in the process so that all controllers
// together stay under Wiz's per-service-account rate limit.
// TODO(needs real schema/docs): confirm Wiz's documented limit.
var limiter = rate.NewLimiter(rate.Limit(3), 5)

var (
	cacheMu sync.Mutex
	cache   = map[Config]graphql.Client{}
)

// Cached returns a client for cfg, reusing one built earlier for the same
// configuration so the OAuth token is fetched once and refreshed on expiry,
// not on every reconcile.
func Cached(cfg Config) graphql.Client {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if c, ok := cache[cfg]; ok {
		return c
	}
	c := New(context.Background(), cfg)
	cache[cfg] = c
	return c
}

// New returns a GraphQL client that authenticates with OAuth 2.0 client
// credentials, caches the token until it expires, and retries throttled and
// server-side failures with exponential backoff.
func New(ctx context.Context, cfg Config) graphql.Client {
	cc := clientcredentials.Config{
		ClientID:       cfg.ClientID,
		ClientSecret:   cfg.ClientSecret,
		TokenURL:       cfg.AuthURL,
		EndpointParams: map[string][]string{"audience": {cfg.Audience}},
		AuthStyle:      oauth2.AuthStyleInParams,
	}
	// Bound token requests too; the token source keeps this context.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
	httpClient := cc.Client(ctx)
	httpClient.Transport = &retryTransport{next: httpClient.Transport, max: cfg.RetryMax, limiter: limiter}
	httpClient.Timeout = 60 * time.Second
	return graphql.NewClient(cfg.URL, httpClient)
}

type retryTransport struct {
	next    http.RoundTripper
	max     int
	limiter *rate.Limiter
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	for attempt := 0; ; attempt++ {
		if err := t.prepare(req, attempt); err != nil {
			return nil, err
		}
		resp, err := t.next.RoundTrip(req)
		if attempt >= t.max || !retryable(resp, err, replayable(req)) {
			return resp, err
		}
		d := backoff(attempt, resp)
		if resp != nil {
			_ = resp.Body.Close()
		}
		if err := sleep(req.Context(), d); err != nil {
			return nil, err
		}
	}
}

// prepare waits for the rate limiter and, on a retry, rewinds the body.
func (t *retryTransport) prepare(req *http.Request, attempt int) error {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return err
	}
	if attempt == 0 || req.GetBody == nil {
		return nil
	}
	body, err := req.GetBody()
	if err != nil {
		return err
	}
	req.Body = body
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// retryable reports whether to try again. A 429 means Wiz did not process the
// request, so it is always safe. A transport error or 5xx is ambiguous: Wiz may
// have applied the request, so only replayable operations are retried.
func retryable(resp *http.Response, err error, replayable bool) bool {
	if err == nil && resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if !replayable {
		return false
	}
	return err != nil || resp.StatusCode >= http.StatusInternalServerError
}

// replayable reports whether sending the request twice is harmless. Queries
// and updates are; a create could make a second, untracked object.
func replayable(req *http.Request) bool {
	if req.GetBody == nil {
		return false
	}
	body, err := req.GetBody()
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // read-only copy of the request body
	var op struct {
		OperationName string `json:"operationName"`
	}
	if err := json.NewDecoder(body).Decode(&op); err != nil {
		return false
	}
	return !strings.HasPrefix(op.OperationName, "Create")
}

func backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			return time.Duration(s) * time.Second
		}
	}
	d := time.Duration(math.Pow(2, float64(attempt))) * 500 * time.Millisecond
	return min(d, 30*time.Second)
}

// IsNotFound reports whether err is Wiz telling us the object does not exist.
// TODO(needs real schema/docs): confirm the error code Wiz uses; this accepts
// the conventional NOT_FOUND extension code and a "not found" message.
func IsNotFound(err error) bool {
	var list gqlerror.List
	if errors.As(err, &list) {
		for _, e := range list {
			if code, _ := e.Extensions["code"].(string); strings.EqualFold(code, "NOT_FOUND") {
				return true
			}
			if strings.Contains(strings.ToLower(e.Message), "not found") {
				return true
			}
		}
	}
	return false
}
