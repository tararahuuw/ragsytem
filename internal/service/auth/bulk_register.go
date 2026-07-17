package auth

import (
	"context"
	"crypto/rand"
	"math/big"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

	authdto "github.com/tararahuuw/ragsytem/internal/dto/auth"
	"github.com/tararahuuw/ragsytem/internal/logger"
	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
	"github.com/tararahuuw/ragsytem/internal/rbac"
)

// MaxBulkUsers caps how many users may be registered in a single bulk request.
const MaxBulkUsers = 100

const (
	tempPasswordLen = 14
	// charset excludes ambiguous chars (0/O, 1/l/I) for readability.
	passwordCharset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789@#%&*"
)

// BulkRegister registers many users at once using the partial-success model:
// each item is processed independently; failures are reported per-item and do
// not abort the batch. Role is always "user"; passwords are auto-generated and
// returned once in the result.
func (s *service) BulkRegister(ctx context.Context, items []authdto.BulkRegisterItem) authdto.BulkRegisterResponse {
	log := logger.FromContext(ctx)
	log.Info("bulk register: attempt", "count", len(items))

	resp := authdto.BulkRegisterResponse{
		Total:   len(items),
		Results: make([]authdto.BulkRegisterResultItem, 0, len(items)),
	}
	seen := make(map[string]bool, len(items))

	for i, item := range items {
		res := s.registerOne(ctx, i, item, seen)
		if res.Status == "created" {
			resp.SuccessCount++
		} else {
			resp.FailedCount++
		}
		resp.Results = append(resp.Results, res)
	}

	log.Info("bulk register: done", "total", resp.Total, "success", resp.SuccessCount, "failed", resp.FailedCount)
	return resp
}

// registerOne processes a single item and returns its outcome. Never returns an
// error — all failures are encoded in the result (partial-success).
func (s *service) registerOne(ctx context.Context, index int, item authdto.BulkRegisterItem, seen map[string]bool) authdto.BulkRegisterResultItem {
	log := logger.FromContext(ctx)
	res := authdto.BulkRegisterResultItem{Index: index, Email: item.Email, Status: "failed"}
	email := strings.TrimSpace(strings.ToLower(item.Email))

	if strings.TrimSpace(item.Name) == "" || email == "" || strings.TrimSpace(item.OrganizationCode) == "" {
		res.ErrorCode, res.Error = "VALIDATION_ERROR", "name, email, dan organization_code wajib diisi"
		return res
	}
	if _, err := mail.ParseAddress(email); err != nil {
		res.ErrorCode, res.Error = "VALIDATION_ERROR", "format email tidak valid"
		return res
	}
	if seen[email] {
		res.ErrorCode, res.Error = "DUPLICATE_IN_BATCH", "email duplikat dalam batch ini"
		return res
	}
	seen[email] = true

	exists, err := s.repo.ExistsByEmail(ctx, item.Email)
	if err != nil {
		log.Error("bulk register: exists check failed", "email", email, "error", err)
		res.ErrorCode, res.Error = "INTERNAL_ERROR", "gagal memeriksa email"
		return res
	}
	if exists {
		res.ErrorCode, res.Error = "EMAIL_TAKEN", "email sudah terdaftar"
		return res
	}

	pw, err := generatePassword(tempPasswordLen)
	if err != nil {
		log.Error("bulk register: password generation failed", "error", err)
		res.ErrorCode, res.Error = "INTERNAL_ERROR", "gagal membuat password"
		return res
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Error("bulk register: hash failed", "error", err)
		res.ErrorCode, res.Error = "INTERNAL_ERROR", "gagal memproses password"
		return res
	}

	u := &usermodel.User{
		Name:             item.Name,
		Email:            item.Email,
		Password:         string(hash),
		OrganizationCode: item.OrganizationCode,
		Role:             rbac.RoleUser,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		log.Error("bulk register: create failed", "email", email, "error", err)
		res.ErrorCode, res.Error = "INTERNAL_ERROR", "gagal membuat user"
		return res
	}

	log.Info("bulk register: user created", "index", index, "user_id", u.ID, "organization_code", u.OrganizationCode)
	res.Status = "created"
	res.ID = u.ID
	res.TempPassword = pw // returned once so admin can distribute it
	return res
}

// generatePassword returns a cryptographically-random password of length n.
func generatePassword(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(passwordCharset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = passwordCharset[idx.Int64()]
	}
	return string(b), nil
}
