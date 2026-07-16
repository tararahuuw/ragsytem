package jwt_test

import (
	"testing"
	"time"

	appjwt "github.com/tararahuuw/ragsytem/internal/jwt"
)

func TestGenerateParse_Roundtrip(t *testing.T) {
	secret := "s3cr3t"
	tok, err := appjwt.Generate(secret, 42, "john@example.com", "pln", "admin", appjwt.TypeAccess, time.Minute)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := appjwt.Parse(secret, tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 || claims.Email != "john@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	// The custom organizationCode + role claims must survive the roundtrip.
	if claims.OrganizationCode != "pln" {
		t.Fatalf("expected organization_code=pln, got %q", claims.OrganizationCode)
	}
	if claims.Role != "admin" {
		t.Fatalf("expected role=admin, got %q", claims.Role)
	}
	if claims.TokenType != appjwt.TypeAccess {
		t.Fatalf("expected token_type=access, got %q", claims.TokenType)
	}
}

func TestParse_WrongSecret(t *testing.T) {
	tok, _ := appjwt.Generate("right", 1, "a@b.com", "pln", "user", appjwt.TypeAccess, time.Minute)
	if _, err := appjwt.Parse("wrong", tok); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestParse_Expired(t *testing.T) {
	tok, _ := appjwt.Generate("s", 1, "a@b.com", "pln", "user", appjwt.TypeAccess, -time.Minute)
	if _, err := appjwt.Parse("s", tok); err == nil {
		t.Fatal("expected error for expired token")
	}
}
