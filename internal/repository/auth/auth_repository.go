package auth

import (
	"sync"
	"time"

	authmodel "github.com/tararahuuw/ragsytem/internal/model/auth"
)

// Repository is the data-access contract for users.
//
// This is a DUMMY in-memory implementation for the sample auth module. Swap
// NewRepository for a GORM-backed one (taking *gorm.DB) when persistence is
// needed — the interface stays the same, so nothing above this layer changes.
type Repository interface {
	Create(u *authmodel.User) (*authmodel.User, error)
	FindByEmail(email string) (*authmodel.User, error)
	ExistsByEmail(email string) bool
}

type memoryRepository struct {
	mu    sync.RWMutex
	users map[string]*authmodel.User
	seq   uint
}

// NewRepository returns an in-memory user repository.
func NewRepository() Repository {
	return &memoryRepository{users: make(map[string]*authmodel.User)}
}

func (r *memoryRepository) Create(u *authmodel.User) (*authmodel.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	u.ID = r.seq
	u.CreatedAt = time.Now()
	r.users[u.Email] = u
	return u, nil
}

func (r *memoryRepository) FindByEmail(email string) (*authmodel.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.users[email], nil // nil when not found
}

func (r *memoryRepository) ExistsByEmail(email string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.users[email]
	return ok
}
