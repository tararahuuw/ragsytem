package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabriel-vasile/mimetype"

	uploaddto "github.com/tararahuuw/ragsytem/internal/dto/upload"
	"github.com/tararahuuw/ragsytem/internal/infra/cache"
	minioinfra "github.com/tararahuuw/ragsytem/internal/infra/minio"
	"github.com/tararahuuw/ragsytem/internal/logger"
	uploadmodel "github.com/tararahuuw/ragsytem/internal/model/upload"
	uploadrepo "github.com/tararahuuw/ragsytem/internal/repository/upload"
)

const (
	tempPrefix      = "temp_chunks"
	finalPrefix     = "uploads"
	maxChunksNoSize = 100
	mergeWaitLimit  = 30 * time.Second
	cleanupDelay    = 5 * time.Second
	// minPartSize is the S3/MinIO server-side compose minimum for every part
	// except the last. Clients MUST chunk at >= 5 MiB for multi-part uploads.
	minPartSize = 5 * 1024 * 1024
	// sessionIdleTTL bounds memory: an abandoned upload session is evicted after
	// this much idle time. janitorInterval is how often we sweep.
	sessionIdleTTL  = 30 * time.Minute
	janitorInterval = 5 * time.Minute
)

// Actor is the authenticated caller performing the upload.
type Actor struct {
	UserID  uint
	OrgCode string
	Role    string
}

// Config carries upload knobs (decoupled from the global config package).
type Config struct {
	MaxFileSize   int64
	PreviewExpiry time.Duration
}

// Service handles chunked, resumable, large-file uploads to object storage.
type Service interface {
	UploadChunk(ctx context.Context, req uploaddto.ChunkRequest, chunk io.Reader, chunkLen int64, actor Actor) (uploaddto.ChunkResult, error)
}

type service struct {
	repo     uploadrepo.Repository
	store    *minioinfra.Client
	cache    cache.Cache
	cfg      Config
	sessions sync.Map // sessionID -> *session
}

// NewService wires an upload Service and starts the idle-session janitor. The
// cache is used only to invalidate the document-list cache when a new document
// completes (so /documents reflects the new file immediately).
func NewService(repo uploadrepo.Repository, store *minioinfra.Client, c cache.Cache, cfg Config) Service {
	s := &service{repo: repo, store: store, cache: c, cfg: cfg}
	go s.janitor()
	return s
}

type session struct {
	totalChunks  int
	mu           sync.Mutex
	received     map[int]bool
	lastActivity time.Time
	mergeStarted atomic.Bool
	done         chan struct{}
	result       uploaddto.ChunkResult
}

func newSession(total int) *session {
	return &session{
		totalChunks:  total,
		received:     make(map[int]bool),
		lastActivity: time.Now(),
		done:         make(chan struct{}),
	}
}

// janitor periodically evicts sessions idle longer than sessionIdleTTL so
// abandoned uploads don't leak memory. Runs for the lifetime of the process.
func (s *service) janitor() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("upload janitor panic recovered", "panic", r)
		}
	}()
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-sessionIdleTTL)
		s.sessions.Range(func(k, v any) bool {
			st := v.(*session)
			st.mu.Lock()
			idle := st.lastActivity.Before(cutoff)
			st.mu.Unlock()
			if idle {
				s.sessions.Delete(k)
				slog.Info("upload: evicted idle session", "session", k)
			}
			return true
		})
	}
}

func (s *service) UploadChunk(ctx context.Context, req uploaddto.ChunkRequest, chunk io.Reader, chunkLen int64, actor Actor) (uploaddto.ChunkResult, error) {
	log := logger.FromContext(ctx)

	// Structural validation for EVERY chunk (session id + index range) — these
	// values shape the object key and merge, so they must be safe on every call.
	if err := validateSessionID(req.SessionID); err != nil {
		log.Warn("upload: invalid sessionId", "session", req.SessionID)
		return uploaddto.ChunkResult{}, err
	}
	if req.TotalChunks <= 0 || req.ChunkIndex < 0 || req.ChunkIndex >= req.TotalChunks {
		log.Warn("upload: chunk index/total out of range", "index", req.ChunkIndex, "total", req.TotalChunks)
		return uploaddto.ChunkResult{}, newErr("VALIDATION_ERROR", http.StatusBadRequest, "chunkIndex atau totalChunks tidak valid")
	}

	finalPath := s.finalObject(actor, req.SessionID)

	// chunk-0 gate: quota, dedup, size/name/type validation
	if req.ChunkIndex == 0 {
		if err := s.checkQuota(ctx, actor); err != nil {
			return uploaddto.ChunkResult{}, err
		}
		if !req.ForceUpload && req.Sha256 != "" {
			exists, err := s.repo.ExistsBySha256(ctx, req.Sha256)
			if err != nil {
				log.Error("upload: dedup check failed", "error", err)
				return uploaddto.ChunkResult{}, err
			}
			if exists {
				log.Warn("upload: rejected duplicate content", "sha256", req.Sha256)
				return uploaddto.ChunkResult{}, newErr("DUPLICATE_FILE", http.StatusConflict, "File dengan konten yang sama sudah pernah diunggah")
			}
		}
		if err := s.validateChunkZero(req); err != nil {
			// Security-relevant rejections (bad name/type/size) — log for tracing.
			log.Warn("upload: chunk-0 validation rejected", "file", req.FileName, "error", err.Error())
			return uploaddto.ChunkResult{}, err
		}
	}

	// Stream the chunk to object storage (chunk-0 also verifies real MIME).
	src := io.Reader(chunk)
	if req.ChunkIndex == 0 {
		br := bufio.NewReaderSize(chunk, 4096)
		header, _ := br.Peek(512)
		if mt := mimetype.Detect(header); !mt.Is("application/pdf") {
			log.Warn("upload: MIME mismatch", "detected", mt.String())
			return uploaddto.ChunkResult{}, newErr("INVALID_FILE_TYPE", http.StatusBadRequest,
				"File bukan PDF yang valid (terdeteksi "+mt.String()+")")
		}
		src = br
	}

	chunkPath := s.chunkObject(actor, req.SessionID, req.ChunkIndex)
	if err := s.store.Put(ctx, chunkPath, src, chunkLen, "application/octet-stream"); err != nil {
		log.Error("upload: failed to store chunk", "session", req.SessionID, "index", req.ChunkIndex, "error", err)
		return uploaddto.ChunkResult{}, err
	}

	// Track session state (chunks may arrive in parallel).
	sess, loaded := s.sessions.LoadOrStore(req.SessionID, newSession(req.TotalChunks))
	st := sess.(*session)
	if !loaded {
		// New session in this instance: guard against a retry after a prior merge.
		if ok, _ := s.store.Exists(ctx, finalPath); ok {
			s.sessions.Delete(req.SessionID)
			return s.completedResult(ctx, req, finalPath), nil
		}
	}

	st.mu.Lock()
	st.received[req.ChunkIndex] = true
	st.lastActivity = time.Now()
	count := len(st.received)
	st.mu.Unlock()

	if count < st.totalChunks {
		log.Info("upload: chunk stored", "session", req.SessionID, "index", req.ChunkIndex, "received", count, "total", st.totalChunks)
		return uploaddto.ChunkResult{
			SessionID:      req.SessionID,
			ChunkIndex:     req.ChunkIndex,
			TotalChunks:    req.TotalChunks,
			UploadComplete: false,
		}, nil
	}

	// All chunks present: exactly one goroutine merges; others wait on done.
	if st.mergeStarted.CompareAndSwap(false, true) {
		result, err := s.finalize(ctx, req, actor, finalPath, st.totalChunks)
		if err != nil {
			// Transient failure (compose/storage): un-poison the session so a
			// client retry can attempt the merge again instead of being stuck.
			st.mergeStarted.Store(false)
			return uploaddto.ChunkResult{}, err
		}
		st.result = result
		close(st.done)
		return result, nil
	}

	// Another request is merging: wait for it (bounded), else report "processing".
	select {
	case <-st.done:
		return st.result, nil
	case <-time.After(mergeWaitLimit):
		return uploaddto.ChunkResult{SessionID: req.SessionID, ChunkIndex: req.ChunkIndex, TotalChunks: req.TotalChunks, UploadComplete: false}, nil
	}
}

// finalize composes the chunks into the final object, records the log + quota,
// and schedules async cleanup.
func (s *service) finalize(ctx context.Context, req uploaddto.ChunkRequest, actor Actor, finalPath string, total int) (uploaddto.ChunkResult, error) {
	log := logger.FromContext(ctx)

	if ok, _ := s.store.Exists(ctx, finalPath); !ok {
		srcs := make([]string, total)
		for i := 0; i < total; i++ {
			srcs[i] = s.chunkObject(actor, req.SessionID, i)
		}
		if err := s.store.Compose(ctx, finalPath, srcs); err != nil {
			log.Error("upload: compose failed", "session", req.SessionID, "error", err)
			return uploaddto.ChunkResult{}, err
		}
	}

	// Audit log (also powers SHA-256 dedup). Non-fatal: the file is already
	// stored; a failure here only weakens dedup for this content.
	if err := s.repo.SaveLog(ctx, &uploadmodel.UploadLog{
		SessionID:        req.SessionID,
		FileName:         req.FileName,
		Sha256:           req.Sha256,
		FileSize:         req.FileSize,
		TotalChunks:      total,
		ObjectPath:       finalPath,
		Status:           uploadmodel.StatusCompleted,
		UserID:           actor.UserID,
		OrganizationCode: actor.OrgCode,
	}); err != nil {
		log.Error("upload: failed to save log", "session", req.SessionID, "error", err)
	}

	// A new document is now visible in this org — invalidate the document-list
	// cache (this org + admin all-orgs). Fail-open: a stale list self-heals at TTL.
	if err := s.cache.Delete(ctx, cache.DocListKeysForOrg(actor.OrgCode)...); err != nil {
		log.Warn("upload: doc cache invalidate failed (fail-open)", "org", actor.OrgCode, "error", err)
	}

	// Quota accounting (non-fatal: don't fail a stored upload over bookkeeping).
	if err := s.repo.IncrementUsage(ctx, actor.UserID, time.Now().Format("2006-01")); err != nil {
		log.Error("upload: failed to increment quota", "user_id", actor.UserID, "error", err)
	}

	// Async cleanup of temp chunks (own background context + panic guard).
	go s.cleanup(actor, req.SessionID, total)

	log.Info("upload: completed", "session", req.SessionID, "object", finalPath, "size", req.FileSize)
	return s.completedResult(ctx, req, finalPath), nil
}

func (s *service) completedResult(ctx context.Context, req uploaddto.ChunkRequest, finalPath string) uploaddto.ChunkResult {
	url, err := s.store.PresignedGetURL(ctx, finalPath, s.cfg.PreviewExpiry, req.FileName)
	if err != nil {
		logger.FromContext(ctx).Warn("upload: failed to presign url", "object", finalPath, "error", err)
	}
	return uploaddto.ChunkResult{
		SessionID:      req.SessionID,
		ChunkIndex:     req.ChunkIndex,
		TotalChunks:    req.TotalChunks,
		UploadComplete: true,
		FileName:       req.FileName,
		ObjectPath:     finalPath,
		PreviewURL:     url,
		Sha256:         req.Sha256,
	}
}

func (s *service) cleanup(actor Actor, sessionID string, total int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("upload cleanup panic recovered", "panic", r, "session", sessionID)
		}
	}()
	time.Sleep(cleanupDelay)

	names := make([]string, total)
	for i := 0; i < total; i++ {
		names[i] = s.chunkObject(actor, sessionID, i)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.store.Remove(ctx, names); err != nil {
		slog.Warn("upload: cleanup failed", "session", sessionID, "error", err)
	}
	s.sessions.Delete(sessionID)
}

func (s *service) checkQuota(ctx context.Context, actor Actor) error {
	log := logger.FromContext(ctx)
	cfg, err := s.repo.GetQuotaConfig(ctx, actor.Role)
	if err != nil {
		log.Error("upload: quota config lookup failed", "role", actor.Role, "error", err)
		return err
	}
	if cfg == nil {
		return nil // no limit configured for this role
	}

	ym := time.Now().Format("2006-01")
	monthly, err := s.repo.GetMonthlyCount(ctx, actor.UserID, ym)
	if err != nil {
		return err
	}
	if monthly >= cfg.MonthlyLimit {
		log.Warn("upload: monthly quota exceeded", "user_id", actor.UserID, "used", monthly, "limit", cfg.MonthlyLimit)
		return newErr("QUOTA_EXCEEDED", http.StatusTooManyRequests,
			fmt.Sprintf("Kuota upload bulanan Anda telah habis (maksimal %d per bulan)", cfg.MonthlyLimit))
	}

	lifetime, err := s.repo.GetLifetimeCount(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if lifetime >= cfg.LifetimeLimit {
		log.Warn("upload: lifetime quota exceeded", "user_id", actor.UserID, "used", lifetime, "limit", cfg.LifetimeLimit)
		return newErr("QUOTA_EXCEEDED", http.StatusTooManyRequests,
			fmt.Sprintf("Kuota upload lifetime Anda telah habis (maksimal %d file)", cfg.LifetimeLimit))
	}
	return nil
}

func (s *service) validateChunkZero(req uploaddto.ChunkRequest) error {
	if req.FileSize > s.cfg.MaxFileSize {
		return newErr("FILE_TOO_LARGE", http.StatusBadRequest,
			fmt.Sprintf("Ukuran file melebihi batas maksimal %d MB", s.cfg.MaxFileSize/(1024*1024)))
	}
	// Server-side compose needs every non-last part >= 5 MiB.
	if req.TotalChunks > 1 && req.ChunkSize > 0 && req.ChunkSize < minPartSize {
		return newErr("CHUNK_TOO_SMALL", http.StatusBadRequest,
			"Ukuran chunk minimal 5 MiB untuk upload multi-part (kecuali chunk terakhir)")
	}
	if req.ChunkSize > 0 {
		expected := int(math.Ceil(float64(req.FileSize) / float64(req.ChunkSize)))
		if req.TotalChunks > expected+1 {
			return newErr("VALIDATION_ERROR", http.StatusBadRequest,
				fmt.Sprintf("Jumlah potongan melebihi ekspektasi maksimum (%d)", expected))
		}
	} else if req.TotalChunks > maxChunksNoSize {
		return newErr("VALIDATION_ERROR", http.StatusBadRequest,
			fmt.Sprintf("Jumlah potongan melebihi batas maksimal %d", maxChunksNoSize))
	}
	if err := validateFileName(req.FileName); err != nil {
		return err
	}
	return validatePDFExtension(req.FileName)
}

// Object keys are scoped per organization AND per user so a session cannot be
// hijacked by another user in the same org guessing the sessionId.
func (s *service) chunkObject(actor Actor, sessionID string, index int) string {
	return fmt.Sprintf("%s/%s/%d/%s/%d", tempPrefix, actor.OrgCode, actor.UserID, sessionID, index)
}

func (s *service) finalObject(actor Actor, sessionID string) string {
	return fmt.Sprintf("%s/%s/%d/%s.pdf", finalPrefix, actor.OrgCode, actor.UserID, sessionID)
}
