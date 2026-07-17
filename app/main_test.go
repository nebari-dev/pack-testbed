package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// makeJWT builds an unsigned JWT-shaped string whose payload encodes claims.
// Only the payload segment matters for decodeClaims (it does not verify).
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantTok   string
		wantOK    bool
	}{
		{"absent", "", "", false},
		{"bearer", "Bearer abc.def.ghi", "abc.def.ghi", true},
		{"case-insensitive scheme", "bearer xyz", "xyz", true},
		{"wrong scheme", "Basic abc", "", false},
		{"prefix only", "Bearer ", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.header != "" {
				h.Set("Authorization", tc.header)
			}
			gotTok, gotOK := bearerToken(h)
			if gotTok != tc.wantTok || gotOK != tc.wantOK {
				t.Fatalf("bearerToken() = (%q, %v), want (%q, %v)", gotTok, gotOK, tc.wantTok, tc.wantOK)
			}
		})
	}
}

func TestDecodeClaims(t *testing.T) {
	valid := makeJWT(t, map[string]any{
		"sub":                "user-123",
		"preferred_username": "alice",
		"groups":             []string{"admin", "dev"},
	})
	tests := []struct {
		name    string
		token   string
		want    tokenClaims
		wantErr bool
	}{
		{
			name:  "valid",
			token: valid,
			want:  tokenClaims{Sub: "user-123", PreferredUsername: "alice", Groups: []string{"admin", "dev"}},
		},
		{"not a jwt", "notajwt", tokenClaims{}, true},
		{"bad base64 payload", "h.@@@.s", tokenClaims{}, true},
		{"bad json payload", "h." + base64.RawURLEncoding.EncodeToString([]byte("{not json")) + ".s", tokenClaims{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeClaims(tc.token)
			if (err != nil) != tc.wantErr {
				t.Fatalf("decodeClaims() err = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("decodeClaims() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestBuildWhoami_NeverLeaksRawSecrets is the security-critical test: in the
// default (unsafe=false) mode the response must contain no raw credential
// material, only sanitized derived facts.
func TestBuildWhoami_NeverLeaksRawSecrets(t *testing.T) {
	const secretCookie = "SUPERSECRETSESSIONVALUE"
	tok := makeJWT(t, map[string]any{
		"sub":                "user-123",
		"preferred_username": "alice",
		"groups":             []string{"admin"},
	})

	r := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	r.AddCookie(&http.Cookie{Name: "session", Value: secretCookie})

	w := buildWhoami(r, "", false)

	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(b)

	// No raw secret material may appear in the response.
	for _, forbidden := range []string{tok, secretCookie, "Bearer", "Authorization", "Cookie"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, out)
		}
	}
	// Derived facts must be present and correct.
	if !w.BearerPresent || !w.SessionCookiePresent || !w.ForwardedByGateway {
		t.Fatalf("expected presence flags true, got %+v", w)
	}
	if w.Sub != "user-123" || w.PreferredUsername != "alice" || !reflect.DeepEqual(w.Groups, []string{"admin"}) {
		t.Fatalf("unexpected derived claims: %+v", w)
	}
	if w.RawHeaders != nil {
		t.Fatalf("RawHeaders must be nil in default mode, got %v", w.RawHeaders)
	}
}

func TestBuildWhoami_UnsafeEchoIsOptIn(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	r.Header.Set("Authorization", "Bearer h.e.s")

	if got := buildWhoami(r, "", false); got.RawHeaders != nil {
		t.Fatalf("default mode must not populate RawHeaders")
	}
	if got := buildWhoami(r, "", true); got.RawHeaders == nil {
		t.Fatalf("unsafe mode must populate RawHeaders")
	}
}

func TestHealthzIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	writeOK(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
}
