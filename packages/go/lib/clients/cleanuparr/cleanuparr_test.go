package cleanuparr_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"homelab/clients/cleanuparr"
	"homelab/httpx"
	"homelab/secret"
)

// fake is a stand-in Cleanuparr, recording what it was asked to do so tests can
// assert on the sequence rather than only the result.
type fake struct {
	setupCompleted bool
	accountCreated bool
	setupFinished  bool
	token          string
	apiKey         string
	// The wire shape, not cleanuparr.Credentials: a decoded secret.Value
	// would be empty, and cmp cannot reach its unexported field.
	seenCreds  wireCreds
	seenBearer string
}

func (f *fake) server(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/auth/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"setupCompleted": f.setupCompleted})
	})
	mux.HandleFunc("/api/auth/setup/account", func(w http.ResponseWriter, r *http.Request) {
		f.accountCreated = true
		decodeJSON(t, r, &f.seenCreds)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/auth/setup/complete", func(w http.ResponseWriter, _ *http.Request) {
		f.setupFinished = true
		f.setupCompleted = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		decodeJSON(t, r, &f.seenCreds)
		writeJSON(t, w, map[string]any{
			"tokens": map[string]any{"accessToken": f.token},
		})
	})
	mux.HandleFunc("/api/account/api-key", func(w http.ResponseWriter, r *http.Request) {
		f.seenBearer = r.Header.Get("Authorization")
		if f.seenBearer != "Bearer "+f.token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(t, w, map[string]any{"apiKey": f.apiKey})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, baseURL string) *cleanuparr.Client {
	t.Helper()

	c, err := cleanuparr.New(httpx.Config{BaseURL: baseURL, RetryMax: -1})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

var testCreds = cleanuparr.Credentials{Username: "admin", Password: secret.New("hunter2")}

func TestCredentialsValid(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		creds cleanuparr.Credentials
		want  bool
	}{
		"both set":    {creds: testCreds, want: true},
		"no username": {creds: cleanuparr.Credentials{Password: secret.New("x")}, want: false},
		"no password": {creds: cleanuparr.Credentials{Username: "x"}, want: false},
		"empty":       {creds: cleanuparr.Credentials{}, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tc.creds.Valid(); got != tc.want {
				t.Errorf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWaitReady(t *testing.T) {
	t.Parallel()

	f := &fake{}
	c := newClient(t, f.server(t).URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.WaitReady(ctx, time.Millisecond); err != nil {
		t.Fatalf("WaitReady() error = %v", err)
	}
}

func TestSetupCompleted(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{"fresh": false, "already set up": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := &fake{setupCompleted: want}
			c := newClient(t, f.server(t).URL)

			got, err := c.SetupCompleted(context.Background())
			if err != nil {
				t.Fatalf("SetupCompleted() error = %v", err)
			}
			if got != want {
				t.Errorf("SetupCompleted() = %v, want %v", got, want)
			}
		})
	}
}

func TestCompleteSetupCreatesAccountThenFinishes(t *testing.T) {
	t.Parallel()

	f := &fake{}
	c := newClient(t, f.server(t).URL)

	if err := c.CompleteSetup(context.Background(), testCreds); err != nil {
		t.Fatalf("CompleteSetup() error = %v", err)
	}
	if !f.accountCreated {
		t.Error("admin account was not created")
	}
	if !f.setupFinished {
		t.Error("setup was not completed")
	}
	if diff := cmp.Diff(wireCreds{Username: "admin", Password: "hunter2"}, f.seenCreds); diff != "" {
		t.Errorf("credentials mismatch (-want +got):\n%s", diff)
	}
}

func TestLoginReturnsToken(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123"}
	c := newClient(t, f.server(t).URL)

	got, err := c.Login(context.Background(), testCreds)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if got.Reveal() != "tok-123" {
		t.Errorf("Login() = %q, want tok-123", got)
	}
}

func TestAPIKeyAuthenticatesWithTheToken(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	c := newClient(t, f.server(t).URL)

	got, err := c.APIKey(context.Background(), secret.New("tok-123"))
	if err != nil {
		t.Fatalf("APIKey() error = %v", err)
	}
	if got.Reveal() != "key-abc" {
		t.Errorf("APIKey() = %q, want key-abc", got)
	}
	if f.seenBearer != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want Bearer tok-123", f.seenBearer)
	}
}

// An unauthenticated key fetch must fail rather than write an empty key file.
func TestAPIKeyRejectsABadToken(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	c := newClient(t, f.server(t).URL)

	if _, err := c.APIKey(context.Background(), secret.New("wrong")); err == nil {
		t.Fatal("APIKey() error = nil, want an authentication failure")
	}
}

func TestEmptyResponsesAreErrors(t *testing.T) {
	t.Parallel()

	t.Run("no access token", func(t *testing.T) {
		t.Parallel()

		f := &fake{token: ""}
		c := newClient(t, f.server(t).URL)

		if _, err := c.Login(context.Background(), testCreds); !errors.Is(err, cleanuparr.ErrNoToken) {
			t.Fatalf("Login() error = %v, want %v", err, cleanuparr.ErrNoToken)
		}
	})

	t.Run("no api key", func(t *testing.T) {
		t.Parallel()

		f := &fake{token: "tok-123", apiKey: ""}
		c := newClient(t, f.server(t).URL)

		_, err := c.APIKey(context.Background(), secret.New("tok-123"))
		if !errors.Is(err, cleanuparr.ErrNoAPIKey) {
			t.Fatalf("APIKey() error = %v, want %v", err, cleanuparr.ErrNoAPIKey)
		}
	})
}

func TestMissingCredentialsAreRejectedBeforeAnyRequest(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123"}
	c := newClient(t, f.server(t).URL)

	if err := c.CompleteSetup(context.Background(), cleanuparr.Credentials{}); !errors.Is(err, cleanuparr.ErrNoCredentials) {
		t.Errorf("CompleteSetup() error = %v, want %v", err, cleanuparr.ErrNoCredentials)
	}
	if f.accountCreated {
		t.Error("an account was created despite empty credentials")
	}
	if _, err := c.Login(context.Background(), cleanuparr.Credentials{}); !errors.Is(err, cleanuparr.ErrNoCredentials) {
		t.Errorf("Login() error = %v, want %v", err, cleanuparr.ErrNoCredentials)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func decodeJSON(t *testing.T, r *http.Request, dst any) {
	t.Helper()

	defer r.Body.Close() //nolint:errcheck // read-only body
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		t.Fatalf("decode request: %v", err)
	}
}

// wireCreds is the JSON body Credentials marshals to.
type wireCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
