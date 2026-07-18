// Package ai is the adapter to the (external) AI/RAG service provided by the
// PLN AI team. The real implementation will POST to their endpoint; a mock is
// used until the contract is finalized (see CLAUDE.md §8c). Swapping the mock
// for the real client is the only change needed once the API is known.
package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tararahuuw/ragsytem/internal/config"
)

// AskRequest is the input to the AI RAG service.
type AskRequest struct {
	Question         string
	OrganizationCode string // retrieval access filter (tenant isolation)
	ThreadID         string // conversation id (AI keeps context per thread)
}

// Source is an optional citation the RAG service may return.
type Source struct {
	DocumentID uint   `json:"document_id,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
}

// AskResponse is the AI's answer.
type AskResponse struct {
	Answer  string
	Sources []Source
}

// Client is the contract for the AI RAG service.
type Client interface {
	Ask(ctx context.Context, req AskRequest) (AskResponse, error)
}

// mockClient returns a canned, deterministic answer — no external call.
type mockClient struct{}

// NewMockClient returns an AI Client that fakes RAG answers (development only,
// until the AI team's API is wired).
func NewMockClient() Client { return &mockClient{} }

// NewClient picks the AI client based on config: the real HTTP client when
// AI_BASE_URL is set, otherwise the mock. This is the single swap point once the
// AI team's contract is finalized (see CLAUDE.md §8c).
func NewClient(cfg *config.Config) Client {
	if cfg.AIBaseURL == "" {
		slog.Warn("ai: AI_BASE_URL not set — using MOCK AI client (answers are fake)")
		return NewMockClient()
	}
	// TODO(ai-team): return newHTTPClient(cfg) once the endpoint/contract is known.
	slog.Warn("ai: real AI client not implemented yet — falling back to MOCK", "base_url", cfg.AIBaseURL)
	return NewMockClient()
}

func (m *mockClient) Ask(_ context.Context, req AskRequest) (AskResponse, error) {
	answer := fmt.Sprintf(
		"[MOCK AI] Pertanyaan \"%s\" diterima (organization=%s, thread=%s). "+
			"Ini jawaban tiruan — integrasi RAG tim AI belum terpasang. (%s)",
		req.Question, req.OrganizationCode, req.ThreadID, time.Now().Format(time.RFC3339),
	)
	return AskResponse{Answer: answer}, nil
}
