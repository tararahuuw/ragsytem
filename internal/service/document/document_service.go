package document

import (
	"context"
	"errors"
	"time"

	documentdto "github.com/tararahuuw/ragsytem/internal/dto/document"
	minioinfra "github.com/tararahuuw/ragsytem/internal/infra/minio"
	"github.com/tararahuuw/ragsytem/internal/logger"
	uploadmodel "github.com/tararahuuw/ragsytem/internal/model/upload"
	"github.com/tararahuuw/ragsytem/internal/rbac"
	documentrepo "github.com/tararahuuw/ragsytem/internal/repository/document"
)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrForbiddenOrg     = errors.New("forbidden: document belongs to a different organization")
)

// Config carries document knobs.
type Config struct {
	PreviewExpiry time.Duration
}

// Service reads uploaded documents. Non-admin callers are scoped to their own
// organizationCode (tenant isolation); admins (super-admin) see all orgs.
type Service interface {
	List(ctx context.Context, actorOrg, actorRole string) ([]documentdto.DocumentResponse, error)
	GetByID(ctx context.Context, id uint, actorOrg, actorRole string) (documentdto.DocumentResponse, error)
}

type service struct {
	repo  documentrepo.Repository
	store *minioinfra.Client
	cfg   Config
}

// NewService wires a document Service.
func NewService(repo documentrepo.Repository, store *minioinfra.Client, cfg Config) Service {
	return &service{repo: repo, store: store, cfg: cfg}
}

func (s *service) List(ctx context.Context, actorOrg, actorRole string) ([]documentdto.DocumentResponse, error) {
	log := logger.FromContext(ctx)

	scope := actorOrg
	if actorRole == rbac.RoleAdmin {
		scope = "" // admin sees documents across all organizations
	}

	docs, err := s.repo.List(ctx, scope)
	if err != nil {
		log.Error("document: list failed", "actor_org", actorOrg, "error", err)
		return nil, err
	}

	res := make([]documentdto.DocumentResponse, 0, len(docs))
	for i := range docs {
		res = append(res, s.toResponse(ctx, &docs[i]))
	}
	log.Info("document: listed", "count", len(res), "actor_org", actorOrg, "actor_role", actorRole)
	return res, nil
}

func (s *service) GetByID(ctx context.Context, id uint, actorOrg, actorRole string) (documentdto.DocumentResponse, error) {
	log := logger.FromContext(ctx)

	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		log.Error("document: lookup failed", "id", id, "error", err)
		return documentdto.DocumentResponse{}, err
	}
	if doc == nil {
		log.Warn("document: not found", "id", id)
		return documentdto.DocumentResponse{}, ErrDocumentNotFound
	}
	if actorRole != rbac.RoleAdmin && doc.OrganizationCode != actorOrg {
		log.Warn("document: cross-organization access blocked",
			"id", id, "doc_org", doc.OrganizationCode, "actor_org", actorOrg)
		return documentdto.DocumentResponse{}, ErrForbiddenOrg
	}

	return s.toResponse(ctx, doc), nil
}

// toResponse maps a completed upload log to a document response, attaching a
// fresh presigned download URL.
func (s *service) toResponse(ctx context.Context, d *uploadmodel.UploadLog) documentdto.DocumentResponse {
	r := documentdto.DocumentResponse{
		ID:               d.ID,
		FileName:         d.FileName,
		FileSize:         d.FileSize,
		TotalChunks:      d.TotalChunks,
		Sha256:           d.Sha256,
		OrganizationCode: d.OrganizationCode,
		UploadedBy:       d.UserID,
		ObjectPath:       d.ObjectPath,
		CreatedAt:        d.CreatedAt,
	}
	if url, err := s.store.PresignedGetURL(ctx, d.ObjectPath, s.cfg.PreviewExpiry, d.FileName); err != nil {
		logger.FromContext(ctx).Warn("document: failed to presign url", "object", d.ObjectPath, "error", err)
	} else {
		r.PreviewURL = url
	}
	return r
}
