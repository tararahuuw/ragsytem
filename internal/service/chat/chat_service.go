package chat

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	chatdto "github.com/tararahuuw/ragsytem/internal/dto/chat"
	"github.com/tararahuuw/ragsytem/internal/infra/ai"
	"github.com/tararahuuw/ragsytem/internal/logger"
	chatmodel "github.com/tararahuuw/ragsytem/internal/model/chat"
	chatrepo "github.com/tararahuuw/ragsytem/internal/repository/chat"
)

const (
	maxSessionsPerUser = 20 // sliding window: oldest evicted beyond this
	titleMaxLen        = 80
	fallbackAnswer     = "Mohon maaf, terjadi kendala saat memproses pertanyaan Anda. Silakan coba lagi."
)

// sessionIDRe validates the client-generated session id (UUID-like) before it
// is used as a DB key / AI thread id.
var sessionIDRe = regexp.MustCompile(`^[a-zA-Z0-9-]{8,64}$`)

// Domain errors surfaced to the controller for HTTP mapping.
var (
	ErrInvalidSession  = errors.New("invalid session id")
	ErrSessionNotFound = errors.New("session not found")
)

// Service holds conversation business logic.
type Service interface {
	Ask(ctx context.Context, userID uint, orgCode string, req chatdto.AskRequest) (chatdto.AskResponse, error)
	ListSessions(ctx context.Context, userID uint) ([]chatdto.SessionResponse, error)
	GetSessionDetail(ctx context.Context, id string, userID uint) (chatdto.SessionDetailResponse, error)
	DeleteSession(ctx context.Context, id string, userID uint) error
}

type service struct {
	repo      chatrepo.Repository
	ai        ai.Client
	aiTimeout time.Duration
}

// NewService wires a chat Service over the repository and AI client. aiTimeout
// bounds every call to the AI service (0 = a safe default).
func NewService(repo chatrepo.Repository, aiClient ai.Client, aiTimeout time.Duration) Service {
	if aiTimeout <= 0 {
		aiTimeout = 30 * time.Second
	}
	return &service{repo: repo, ai: aiClient, aiTimeout: aiTimeout}
}

func (s *service) Ask(ctx context.Context, userID uint, orgCode string, req chatdto.AskRequest) (chatdto.AskResponse, error) {
	log := logger.FromContext(ctx)

	if !sessionIDRe.MatchString(req.SessionID) {
		log.Warn("chat: invalid session id", "session", req.SessionID)
		return chatdto.AskResponse{}, ErrInvalidSession
	}

	sess, err := s.repo.GetSession(ctx, req.SessionID)
	if err != nil {
		log.Error("chat: failed to load session", "session", req.SessionID, "error", err)
		return chatdto.AskResponse{}, err
	}
	// A session id belonging to another user is treated as not found (no leak).
	if sess != nil && sess.UserID != userID {
		log.Warn("chat: session ownership mismatch", "session", req.SessionID, "owner", sess.UserID, "actor", userID)
		return chatdto.AskResponse{}, ErrSessionNotFound
	}

	if sess == nil {
		if err := s.enforceSlidingWindow(ctx, userID); err != nil {
			log.Error("chat: sliding window failed", "user_id", userID, "error", err)
			return chatdto.AskResponse{}, err
		}
		sess = &chatmodel.Session{
			ID:               req.SessionID,
			UserID:           userID,
			OrganizationCode: orgCode,
			Title:            makeTitle(req.Question),
		}
		if err := s.repo.CreateSession(ctx, sess); err != nil {
			log.Error("chat: failed to create session", "session", req.SessionID, "error", err)
			return chatdto.AskResponse{}, err
		}
		log.Info("chat: new session", "session", sess.ID, "user_id", userID, "organization_code", orgCode)
	}

	if err := s.repo.AddMessage(ctx, &chatmodel.Message{SessionID: sess.ID, Role: chatmodel.RoleUser, Content: req.Question}); err != nil {
		log.Error("chat: failed to save user message", "session", sess.ID, "error", err)
		return chatdto.AskResponse{}, err
	}

	// Call the AI RAG service (mock until the AI team's API is wired). A failure
	// is degraded to a friendly fallback answer (still 200), and logged.
	log.Info("chat: ask", "session", sess.ID, "organization_code", sess.OrganizationCode)
	answer := fallbackAnswer
	aiCtx, cancel := context.WithTimeout(ctx, s.aiTimeout)
	aiResp, aiErr := s.ai.Ask(aiCtx, ai.AskRequest{
		Question:         req.Question,
		OrganizationCode: sess.OrganizationCode,
		ThreadID:         sess.ID,
	})
	cancel()
	if aiErr != nil {
		log.Error("chat: AI call failed", "session", sess.ID, "error", aiErr)
	} else {
		answer = aiResp.Answer
	}

	if err := s.repo.AddMessage(ctx, &chatmodel.Message{SessionID: sess.ID, Role: chatmodel.RoleAssistant, Content: answer}); err != nil {
		log.Error("chat: failed to save assistant message", "session", sess.ID, "error", err)
		return chatdto.AskResponse{}, err
	}
	if err := s.repo.TouchSession(ctx, sess.ID); err != nil {
		log.Warn("chat: failed to touch session", "session", sess.ID, "error", err) // non-fatal
	}

	log.Info("chat: answered", "session", sess.ID)
	return chatdto.AskResponse{SessionID: sess.ID, Answer: answer}, nil
}

func (s *service) enforceSlidingWindow(ctx context.Context, userID uint) error {
	count, err := s.repo.CountSessions(ctx, userID)
	if err != nil {
		return err
	}
	if count < maxSessionsPerUser {
		return nil
	}
	oldest, err := s.repo.OldestSessionID(ctx, userID)
	if err != nil {
		return err
	}
	if oldest == "" {
		return nil
	}
	if err := s.repo.DeleteSession(ctx, oldest); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("chat: evicted oldest session (sliding window)", "session", oldest, "user_id", userID)
	return nil
}

func (s *service) ListSessions(ctx context.Context, userID uint) ([]chatdto.SessionResponse, error) {
	sessions, err := s.repo.ListSessions(ctx, userID)
	if err != nil {
		logger.FromContext(ctx).Error("chat: failed to list sessions", "user_id", userID, "error", err)
		return nil, err
	}
	res := make([]chatdto.SessionResponse, 0, len(sessions))
	for _, ss := range sessions {
		res = append(res, chatdto.SessionResponse{ID: ss.ID, Title: ss.Title, CreatedAt: ss.CreatedAt, UpdatedAt: ss.UpdatedAt})
	}
	return res, nil
}

func (s *service) GetSessionDetail(ctx context.Context, id string, userID uint) (chatdto.SessionDetailResponse, error) {
	log := logger.FromContext(ctx)
	sess, err := s.repo.GetSession(ctx, id)
	if err != nil {
		log.Error("chat: failed to load session", "session", id, "error", err)
		return chatdto.SessionDetailResponse{}, err
	}
	if sess == nil || sess.UserID != userID {
		log.Warn("chat: session not found / not owned", "session", id, "actor", userID)
		return chatdto.SessionDetailResponse{}, ErrSessionNotFound
	}

	msgs, err := s.repo.ListMessages(ctx, id)
	if err != nil {
		log.Error("chat: failed to list messages", "session", id, "error", err)
		return chatdto.SessionDetailResponse{}, err
	}
	out := chatdto.SessionDetailResponse{
		ID:               sess.ID,
		Title:            sess.Title,
		OrganizationCode: sess.OrganizationCode,
		Messages:         make([]chatdto.MessageResponse, 0, len(msgs)),
	}
	for _, m := range msgs {
		out.Messages = append(out.Messages, chatdto.MessageResponse{ID: m.ID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt})
	}
	return out, nil
}

func (s *service) DeleteSession(ctx context.Context, id string, userID uint) error {
	log := logger.FromContext(ctx)
	sess, err := s.repo.GetSession(ctx, id)
	if err != nil {
		log.Error("chat: failed to load session", "session", id, "error", err)
		return err
	}
	if sess == nil || sess.UserID != userID {
		log.Warn("chat: delete on missing / not-owned session", "session", id, "actor", userID)
		return ErrSessionNotFound
	}
	if err := s.repo.DeleteSession(ctx, id); err != nil {
		log.Error("chat: failed to delete session", "session", id, "error", err)
		return err
	}
	log.Info("chat: session deleted", "session", id, "user_id", userID)
	return nil
}

func makeTitle(q string) string {
	q = strings.TrimSpace(q)
	r := []rune(q)
	if len(r) > titleMaxLen {
		return string(r[:titleMaxLen])
	}
	return q
}
