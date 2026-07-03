package usesmileid

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// tokenExpirySkew is subtracted from the JWT expiry so the cached token is
// refreshed slightly before it actually expires.
const tokenExpirySkew = 60 * time.Second

// tokenCache is a thread-safe cache for the internal JWT. The mutex is held
// for the duration of a fetch so concurrent callers never stampede the token
// endpoint: the first caller fetches, the rest read the cached value.
type tokenCache struct {
	mu        sync.Mutex
	jwt       string
	expiresAt time.Time
	fetch     func(ctx context.Context) (string, error)
}

// token returns a valid cached token, fetching a new one if the cache is empty
// or expired.
func (tc *tokenCache) token(ctx context.Context) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.jwt != "" && time.Now().Before(tc.expiresAt) {
		return tc.jwt, nil
	}

	jwt, err := tc.fetch(ctx)
	if err != nil {
		return "", err
	}
	tc.jwt = jwt
	if exp, ok := decodeJWTExp(jwt); ok {
		tc.expiresAt = exp.Add(-tokenExpirySkew)
	} else {
		// Undecodable expiry: treat as single-use, refresh on the next call.
		tc.expiresAt = time.Now()
	}
	return jwt, nil
}

// invalidate clears the cache so the next call fetches a fresh token. Used by
// the transport's refresh-on-401 path.
func (tc *tokenCache) invalidate() {
	tc.mu.Lock()
	tc.jwt = ""
	tc.expiresAt = time.Time{}
	tc.mu.Unlock()
}

// decodeJWTExp reads the exp claim from a JWT without verifying the signature.
func decodeJWTExp(jwt string) (time.Time, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if payload, err = base64.URLEncoding.DecodeString(parts[1]); err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(claims.Exp), 0), true
}
