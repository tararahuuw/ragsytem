package document

import (
	"context"
	"errors"

	"gorm.io/gorm"

	uploadmodel "github.com/tararahuuw/ragsytem/internal/model/upload"
)

// Repository reads uploaded documents from the completed upload logs.
type Repository interface {
	// List returns completed documents, newest first. If orgCode is non-empty it
	// filters by organization; empty means all organizations (admin scope).
	List(ctx context.Context, orgCode string) ([]uploadmodel.UploadLog, error)
	// FindByID returns a completed document by id, or nil if not found.
	FindByID(ctx context.Context, id uint) (*uploadmodel.UploadLog, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository wires a document Repository over the given GORM connection.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) List(ctx context.Context, orgCode string) ([]uploadmodel.UploadLog, error) {
	var docs []uploadmodel.UploadLog
	q := r.db.WithContext(ctx).Where("status = ?", uploadmodel.StatusCompleted)
	if orgCode != "" {
		q = q.Where("organization_code = ?", orgCode)
	}
	err := q.Order("created_at DESC").Find(&docs).Error
	return docs, err
}

func (r *gormRepository) FindByID(ctx context.Context, id uint) (*uploadmodel.UploadLog, error) {
	var doc uploadmodel.UploadLog
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, uploadmodel.StatusCompleted).
		First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}
