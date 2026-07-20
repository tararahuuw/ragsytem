package user

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	usermodel "github.com/tararahuuw/ragsytem/internal/model/user"
)

// Repository is the data-access contract for users (GORM-backed, soft delete).
type Repository interface {
	Create(ctx context.Context, u *usermodel.User) error
	FindByEmail(ctx context.Context, email string) (*usermodel.User, error)
	FindByID(ctx context.Context, id uint) (*usermodel.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Update(ctx context.Context, u *usermodel.User) error
	SoftDelete(ctx context.Context, id uint) error

	// Auth security
	BumpTokenVersion(ctx context.Context, userID uint) error
	SetPasswordAndBumpVersion(ctx context.Context, userID uint, passwordHash string) error
	CreateResetToken(ctx context.Context, t *usermodel.PasswordResetToken) error
	FindValidResetToken(ctx context.Context, tokenHash string) (*usermodel.PasswordResetToken, error)
	MarkResetTokenUsed(ctx context.Context, id uint) error
	InvalidateUserResetTokens(ctx context.Context, userID uint) error
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository wires a user Repository over the given GORM connection.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(ctx context.Context, u *usermodel.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// FindByEmail returns nil (no error) when no active user matches.
func (r *gormRepository) FindByEmail(ctx context.Context, email string) (*usermodel.User, error) {
	var u usermodel.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByID returns nil (no error) when no active user matches.
func (r *gormRepository) FindByID(ctx context.Context, id uint) (*usermodel.User, error) {
	var u usermodel.User
	err := r.db.WithContext(ctx).First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&usermodel.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) Update(ctx context.Context, u *usermodel.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

// SoftDelete sets deleted_at (GORM soft delete) for the given id.
func (r *gormRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&usermodel.User{}, id).Error
}

// BumpTokenVersion invalidates a user's existing refresh tokens (logout).
func (r *gormRepository) BumpTokenVersion(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Model(&usermodel.User{}).Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

// SetPasswordAndBumpVersion updates the password and bumps token_version in one
// statement (change / reset password → also revokes existing sessions).
func (r *gormRepository) SetPasswordAndBumpVersion(ctx context.Context, userID uint, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&usermodel.User{}).Where("id = ?", userID).
		Updates(map[string]any{
			"password":      passwordHash,
			"token_version": gorm.Expr("token_version + 1"),
		}).Error
}

func (r *gormRepository) CreateResetToken(ctx context.Context, t *usermodel.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

// FindValidResetToken returns an unused, unexpired token by its hash (nil if none).
func (r *gormRepository) FindValidResetToken(ctx context.Context, tokenHash string) (*usermodel.PasswordResetToken, error) {
	var t usermodel.PasswordResetToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *gormRepository) MarkResetTokenUsed(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&usermodel.PasswordResetToken{}).Where("id = ?", id).
		Update("used_at", now).Error
}

// InvalidateUserResetTokens marks all of a user's outstanding tokens used (so a
// new forgot-password request supersedes older links).
func (r *gormRepository) InvalidateUserResetTokens(ctx context.Context, userID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&usermodel.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", now).Error
}
