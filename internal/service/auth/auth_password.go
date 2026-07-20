package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	authdto "github.com/tararahuuw/ragsytem/internal/dto/auth"
	"github.com/tararahuuw/ragsytem/internal/logger"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
)

// ChangePassword verifies the current password and sets a new one, revoking
// existing sessions (token_version bump).
func (s *service) ChangePassword(ctx context.Context, userID uint, req authdto.ChangePasswordRequest) error {
	log := logger.FromContext(ctx)

	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		log.Error("change password: lookup failed", "user_id", userID, "error", err)
		return err
	}
	if u == nil {
		return ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.OldPassword)) != nil {
		log.Warn("change password: wrong current password", "user_id", userID)
		return ErrInvalidOldPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error("change password: hash failed", "user_id", userID, "error", err)
		return err
	}
	if err := s.repo.SetPasswordAndBumpVersion(ctx, userID, string(hash)); err != nil {
		log.Error("change password: update failed", "user_id", userID, "error", err)
		return err
	}
	log.Info("change password: success", "user_id", userID)
	return nil
}

// Logout revokes the user's refresh tokens (bumps token_version). Access tokens
// expire naturally within their (short) TTL.
func (s *service) Logout(ctx context.Context, userID uint) error {
	log := logger.FromContext(ctx)
	if err := s.repo.BumpTokenVersion(ctx, userID); err != nil {
		log.Error("logout: failed", "user_id", userID, "error", err)
		return err
	}
	log.Info("logout: success", "user_id", userID)
	return nil
}

// ForgotPassword issues a reset token and emails it. Always returns nil so the
// caller can respond identically whether or not the email exists (anti user
// enumeration).
func (s *service) ForgotPassword(ctx context.Context, req authdto.ForgotPasswordRequest) error {
	log := logger.FromContext(ctx)
	email := strings.TrimSpace(strings.ToLower(req.Email))

	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		log.Error("forgot password: lookup failed", "email", email, "error", err)
		return nil // don't leak internal errors either
	}
	if u == nil {
		log.Info("forgot password: no such active user (silently ok)", "email", email)
		return nil
	}

	token, err := randomToken(32)
	if err != nil {
		log.Error("forgot password: token gen failed", "error", err)
		return nil
	}

	// Supersede any outstanding tokens, then store the new one hashed.
	_ = s.repo.InvalidateUserResetTokens(ctx, u.ID)
	if err := s.repo.CreateResetToken(ctx, &usermodel.PasswordResetToken{
		UserID:    u.ID,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().Add(s.cfg.PasswordResetTTL),
	}); err != nil {
		log.Error("forgot password: store token failed", "user_id", u.ID, "error", err)
		return nil
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(s.cfg.AppBaseURL, "/"), token)
	body := fmt.Sprintf(
		"Halo %s,\n\nAnda meminta reset password. Gunakan tautan berikut (berlaku %d menit):\n%s\n\nJika bukan Anda, abaikan email ini.",
		u.Name, int(s.cfg.PasswordResetTTL.Minutes()), resetLink,
	)
	if err := s.email.Send(ctx, u.Email, "Reset Password", body); err != nil {
		log.Error("forgot password: email send failed", "user_id", u.ID, "error", err)
		return nil
	}

	log.Info("forgot password: reset token issued", "user_id", u.ID)
	return nil
}

// ResetPassword validates a reset token and sets a new password (revoking
// sessions and consuming the token).
func (s *service) ResetPassword(ctx context.Context, req authdto.ResetPasswordRequest) error {
	log := logger.FromContext(ctx)

	rt, err := s.repo.FindValidResetToken(ctx, hashToken(req.Token))
	if err != nil {
		log.Error("reset password: token lookup failed", "error", err)
		return err
	}
	if rt == nil {
		log.Warn("reset password: invalid/expired token")
		return ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error("reset password: hash failed", "user_id", rt.UserID, "error", err)
		return err
	}
	if err := s.repo.SetPasswordAndBumpVersion(ctx, rt.UserID, string(hash)); err != nil {
		log.Error("reset password: update failed", "user_id", rt.UserID, "error", err)
		return err
	}
	if err := s.repo.MarkResetTokenUsed(ctx, rt.ID); err != nil {
		log.Error("reset password: mark used failed", "token_id", rt.ID, "error", err)
	}

	log.Info("reset password: success", "user_id", rt.UserID)
	return nil
}

// randomToken returns a URL-safe hex token of n random bytes.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken stores only the SHA-256 of a reset token (never the plaintext).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
