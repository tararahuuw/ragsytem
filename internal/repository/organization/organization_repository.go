package organization

import (
	"context"
	"errors"

	"gorm.io/gorm"

	orgmodel "github.com/tararahuuw/ragsytem/internal/model/organization"
)

// Repository is the data-access contract for organizations.
type Repository interface {
	Create(ctx context.Context, o *orgmodel.Organization) error
	GetByCode(ctx context.Context, code string) (*orgmodel.Organization, error)
	List(ctx context.Context) ([]orgmodel.Organization, error)
	Update(ctx context.Context, o *orgmodel.Organization) error
	SoftDelete(ctx context.Context, code string) error
	// ExistsActive reports whether an active (non-deleted) org with this code
	// exists — used to validate organization_code at register time.
	ExistsActive(ctx context.Context, code string) (bool, error)
	// CountUsers returns the number of active users bound to this org.
	CountUsers(ctx context.Context, code string) (int64, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository wires an organization Repository over the given GORM connection.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(ctx context.Context, o *orgmodel.Organization) error {
	return r.db.WithContext(ctx).Create(o).Error
}

// GetByCode returns nil (no error) when the org does not exist.
func (r *gormRepository) GetByCode(ctx context.Context, code string) (*orgmodel.Organization, error) {
	var o orgmodel.Organization
	err := r.db.WithContext(ctx).First(&o, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *gormRepository) List(ctx context.Context) ([]orgmodel.Organization, error) {
	var orgs []orgmodel.Organization
	err := r.db.WithContext(ctx).Order("code ASC").Find(&orgs).Error
	return orgs, err
}

func (r *gormRepository) Update(ctx context.Context, o *orgmodel.Organization) error {
	return r.db.WithContext(ctx).Save(o).Error
}

func (r *gormRepository) SoftDelete(ctx context.Context, code string) error {
	return r.db.WithContext(ctx).Where("code = ?", code).Delete(&orgmodel.Organization{}).Error
}

func (r *gormRepository) ExistsActive(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&orgmodel.Organization{}).
		Where("code = ? AND active = ?", code, true).Count(&count).Error
	return count > 0, err
}

// CountUsers counts active (non-soft-deleted) users in an org. Reads the users
// table directly to avoid a module dependency on the user model.
func (r *gormRepository) CountUsers(ctx context.Context, code string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("users").
		Where("organization_code = ? AND deleted_at IS NULL", code).Count(&count).Error
	return count, err
}
