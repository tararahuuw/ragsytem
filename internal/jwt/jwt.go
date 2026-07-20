// Package jwt wraps token generation/parsing with the app's custom claims
// (including organizationCode for multi-tenant identification).
package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Token types embedded in claims so an access token can't be used where a
// refresh token is expected (and vice versa).
const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

// ErrInvalidToken is returned when a token is malformed, expired, or has an
// unexpected signing method.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims are the custom JWT claims carried by every token.
type Claims struct {
	UserID           uint   `json:"user_id"`
	Email            string `json:"email"`
	OrganizationCode string `json:"organization_code"`
	Role             string `json:"role"`
	TokenVersion     int    `json:"ver"`
	TokenType        string `json:"token_type"`
	jwtlib.RegisteredClaims
}

// Generate signs a token of the given type with the provided TTL.
func Generate(secret string, userID uint, email, orgCode, role string, tokenVersion int, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:           userID,
		Email:            email,
		OrganizationCode: orgCode,
		Role:             role,
		TokenVersion:     tokenVersion,
		TokenType:        tokenType,
		RegisteredClaims: jwtlib.RegisteredClaims{
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse validates a token's signature/expiry and returns its claims.
func Parse(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwtlib.ParseWithClaims(tokenString, claims, func(t *jwtlib.Token) (any, error) {
		// Reject any signing method other than HMAC (prevents alg-swap attacks).
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
