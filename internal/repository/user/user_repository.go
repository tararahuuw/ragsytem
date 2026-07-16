package user

import (
	"context"
	"errors"

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
