package upload

import (
	"context"
	"errors"

	"gorm.io/gorm"

	uploadmodel "github.com/tararahuuw/ragsytem/internal/model/upload"
)

// Repository handles upload audit/dedup logs and per-role quota bookkeeping.
type Repository interface {
	// dedup / audit
	ExistsBySha256(ctx context.Context, sha256 string) (bool, error)
	SaveLog(ctx context.Context, l *uploadmodel.UploadLog) error

	// quota
	GetQuotaConfig(ctx context.Context, role string) (*uploadmodel.UploadQuotaConfig, error)
	GetMonthlyCount(ctx context.Context, userID uint, yearMonth string) (int, error)
	GetLifetimeCount(ctx context.Context, userID uint) (int, error)
	IncrementUsage(ctx context.Context, userID uint, yearMonth string) error
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository wires an upload Repository over the given GORM connection.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ExistsBySha256(ctx context.Context, sha256 string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&uploadmodel.UploadLog{}).
		Where("sha256 = ? AND status = ?", sha256, uploadmodel.StatusCompleted).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) SaveLog(ctx context.Context, l *uploadmodel.UploadLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

// GetQuotaConfig returns the enabled config for a role, or nil (no limit).
func (r *gormRepository) GetQuotaConfig(ctx context.Context, role string) (*uploadmodel.UploadQuotaConfig, error) {
	var cfg uploadmodel.UploadQuotaConfig
	err := r.db.WithContext(ctx).Where("role = ? AND enabled = ?", role, true).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *gormRepository) GetMonthlyCount(ctx context.Context, userID uint, yearMonth string) (int, error) {
	var usage uploadmodel.UploadQuotaUsage
	err := r.db.WithContext(ctx).Where("user_id = ? AND year_month = ?", userID, yearMonth).First(&usage).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return usage.MonthlyCount, nil
}

// GetLifetimeCount returns the highest lifetime counter recorded for a user.
func (r *gormRepository) GetLifetimeCount(ctx context.Context, userID uint) (int, error) {
	var max *int
	err := r.db.WithContext(ctx).Model(&uploadmodel.UploadQuotaUsage{}).
		Where("user_id = ?", userID).
		Select("MAX(lifetime_count)").Scan(&max).Error
	if err != nil {
		return 0, err
	}
	if max == nil {
		return 0, nil
	}
	return *max, nil
}

// IncrementUsage bumps the monthly counter for the current month and carries the
// lifetime counter forward (+1), creating the month row if needed.
func (r *gormRepository) IncrementUsage(ctx context.Context, userID uint, yearMonth string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lifetime, err := (&gormRepository{db: tx}).GetLifetimeCount(ctx, userID)
		if err != nil {
			return err
		}

		var usage uploadmodel.UploadQuotaUsage
		err = tx.WithContext(ctx).Where("user_id = ? AND year_month = ?", userID, yearMonth).First(&usage).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			usage = uploadmodel.UploadQuotaUsage{
				UserID:        userID,
				YearMonth:     yearMonth,
				MonthlyCount:  1,
				LifetimeCount: lifetime + 1,
			}
			return tx.WithContext(ctx).Create(&usage).Error
		}
		if err != nil {
			return err
		}
		usage.MonthlyCount++
		usage.LifetimeCount = lifetime + 1
		return tx.WithContext(ctx).Save(&usage).Error
	})
}
