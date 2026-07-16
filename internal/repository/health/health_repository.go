package health

import (
	"context"

	"gorm.io/gorm"
)

// Repository checks the health of backing data stores.
type Repository interface {
	Ping(ctx context.Context) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository wires a health Repository over the given GORM connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Ping(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
