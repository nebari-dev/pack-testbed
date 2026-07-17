// Command app is a deliberately minimal HTTP service used as a fixture for
// exercising the Nebari reusable release/signing workflows and the NebariApp CRD
// API in ephemeral test clusters.
//
// SECURITY: this is a CI test fixture, not a product. By default it NEVER
// returns raw credential material (Authorization/Cookie values or raw JWTs) -
// only sanitized, derived facts sufficient for test assertions. A raw-echo debug
// mode exists ONLY when TESTBED_UNSAFE_ECHO=1 is set, which the ephemeral test
// harness may enable; it must never be set in a shipped/default deployment.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// whoami is the sanitized, derived view returned by /whoami. It deliberately
// contains no raw credential material.
type whoami struct {
	BearerPresent        bool                `json:"bearer_present"`
	SessionCookiePresent bool                `json:"session_cookie_present"`
	ForwardedByGateway   bool                `json:"forwarded_by_gateway"`
	Sub                  string              `json:"sub,omitempty"`
	PreferredUsername    string              `json:"preferred_username,omitempty"`
	Groups               []string            `json:"groups,omitempty"`
	// RawHeaders is populated ONLY when unsafe echo is explicitly enabled
	// (TESTBED_UNSAFE_ECHO=1). It is never set in the default/shipped mode.
	RawHeaders map[string][]string `json:"raw_headers,omitempty"`
}

// tokenClaims are the non-secret JWT claims surfaced for assertions.
type tokenClaims struct {
	Sub               string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

// bearerToken returns the token from an "Authorization: Bearer <t>" header.
func bearerToken(h http.Header) (string, bool) {
	const prefix = "Bearer "
	v := h.Get("Authorization")
	if len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
		return v[len(prefix):], true
	}
	return "", false
}

// sessionCookiePresent reports whether a session cookie is set. The gateway's
// OIDC filter sets one when auth is enforced. When name is empty, any cookie
// counts.
func sessionCookiePresent(r *http.Request, name string) bool {
	if name == "" {
		return len(r.Cookies()) > 0
	}
	_, err := r.Cookie(name)
	return err == nil
}

// decodeClaims base64url-decodes the payload segment of a JWT and unmarshals the
// non-secret claims. It does NOT verify the signature: the gateway already
// enforced auth; we only surface claims for assertions.
func decodeClaims(token string) (tokenClaims, error) {
	var c tokenClaims
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return c, errors.New("not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, err
	}
	return c, nil
}

// buildWhoami constructs the sanitized response. When unsafe is false (the
// default and the only shipped behavior) it never includes raw credential
// material.
func buildWhoami(r *http.Request, sessionCookieName string, unsafe bool) whoami {
	w := whoami{}
	if tok, ok := bearerToken(r.Header); ok {
		w.BearerPresent = true
		// The gateway injects the bearer only when forwardAccessToken is set,
		// so in the controlled test environment its presence means the gateway
		// forwarded it.
		w.ForwardedByGateway = true
		if claims, err := decodeClaims(tok); err == nil {
			w.Sub = claims.Sub
			w.PreferredUsername = claims.PreferredUsername
			w.Groups = claims.Groups
		}
	}
	w.SessionCookiePresent = sessionCookiePresent(r, sessionCookieName)
	if unsafe {
		w.RawHeaders = r.Header.Clone()
	}
	return w
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeOK(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func whoamiHandler(sessionCookie string, unsafe bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(buildWhoami(r, sessionCookie, unsafe))
	}
}

func main() {
	addr := ":" + getenv("PORT", "8080")
	unsafe := os.Getenv("TESTBED_UNSAFE_ECHO") == "1"
	sessionCookie := os.Getenv("TESTBED_SESSION_COOKIE") // optional exact cookie name

	if unsafe {
		log.Println("WARNING: TESTBED_UNSAFE_ECHO=1 - raw headers will be echoed. " +
			"This is a test-only debug mode; never enable it on a shared cluster.")
	}

	mux := http.NewServeMux()
	// Public, unauthenticated health/probe endpoints (targets for publicRoutes).
	mux.HandleFunc("/healthz", writeOK)
	mux.HandleFunc("/livez", writeOK)
	mux.HandleFunc("/readyz", writeOK)
	// Protected introspection endpoint (assert what the gateway forwarded).
	mux.HandleFunc("/whoami", whoamiHandler(sessionCookie, unsafe))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("pack-testbed fixture - see /whoami and /healthz\n"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("pack-testbed app listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
