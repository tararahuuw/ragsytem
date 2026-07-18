package chat

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	chatmodel "github.com/tararahuuw/ragsytem/internal/model/chat"
)

// Repository is the data-access contract for conversations.
type Repository interface {
	GetSession(ctx context.Context, id string) (*chatmodel.Session, error)
	CountSessions(ctx context.Context, userID uint) (int64, error)
	OldestSessionID(ctx context.Context, userID uint) (string, error)
	CreateSession(ctx context.Context, s *chatmodel.Session) error
	TouchSession(ctx context.Context, id string) error
	ListSessions(ctx context.Context, userID uint) ([]chatmodel.Session, error)
	DeleteSession(ctx context.Context, id string) error

	AddMessage(ctx context.Context, m *chatmodel.Message) error
	ListMessages(ctx context.Context, sessionID string) ([]chatmodel.Message, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository wires a chat Repository over the given GORM connection.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

// GetSession returns nil (no error) when the session does not exist.
func (r *gormRepository) GetSession(ctx context.Context, id string) (*chatmodel.Session, error) {
	var s chatmodel.Session
	err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormRepository) CountSessions(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&chatmodel.Session{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// OldestSessionID returns the id of the user's oldest session (or "" if none).
func (r *gormRepository) OldestSessionID(ctx context.Context, userID uint) (string, error) {
	var s chatmodel.Session
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// CreateSession is idempotent on the primary key: a concurrent request that
// races on the same (client-generated) session id won't fail with a PK conflict.
func (r *gormRepository) CreateSession(ctx context.Context, s *chatmodel.Session) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(s).Error
}

func (r *gormRepository) TouchSession(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&chatmodel.Session{}).Where("id = ?", id).
		Update("updated_at", gorm.Expr("now()")).Error
}

func (r *gormRepository) ListSessions(ctx context.Context, userID uint) ([]chatmodel.Session, error) {
	var sessions []chatmodel.Session
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC").Find(&sessions).Error
	return sessions, err
}

// DeleteSession removes a session and all its messages in one transaction.
func (r *gormRepository) DeleteSession(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", id).Delete(&chatmodel.Message{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&chatmodel.Session{}).Error
	})
}

func (r *gormRepository) AddMessage(ctx context.Context, m *chatmodel.Message) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *gormRepository) ListMessages(ctx context.Context, sessionID string) ([]chatmodel.Message, error) {
	var msgs []chatmodel.Message
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at ASC, id ASC").Find(&msgs).Error
	return msgs, err
}
