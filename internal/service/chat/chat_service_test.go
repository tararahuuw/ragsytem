package chat

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	chatdto "github.com/tararahuuw/ragsytem/internal/dto/chat"
	"github.com/tararahuuw/ragsytem/internal/infra/ai"
	chatmodel "github.com/tararahuuw/ragsytem/internal/model/chat"
)

// fakeRepo is an in-memory chatrepo.Repository for testing service logic.
type fakeRepo struct {
	sessions map[string]*chatmodel.Session
	messages map[string][]chatmodel.Message
	seq      int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{sessions: map[string]*chatmodel.Session{}, messages: map[string][]chatmodel.Message{}}
}

func (f *fakeRepo) tick() time.Time { f.seq++; return time.Unix(f.seq, 0) }

func (f *fakeRepo) GetSession(_ context.Context, id string) (*chatmodel.Session, error) {
	return f.sessions[id], nil
}
func (f *fakeRepo) CountSessions(_ context.Context, userID uint) (int64, error) {
	var n int64
	for _, s := range f.sessions {
		if s.UserID == userID {
			n++
		}
	}
	return n, nil
}
func (f *fakeRepo) OldestSessionID(_ context.Context, userID uint) (string, error) {
	var oldest *chatmodel.Session
	for _, s := range f.sessions {
		if s.UserID != userID {
			continue
		}
		if oldest == nil || s.CreatedAt.Before(oldest.CreatedAt) {
			oldest = s
		}
	}
	if oldest == nil {
		return "", nil
	}
	return oldest.ID, nil
}
func (f *fakeRepo) CreateSession(_ context.Context, s *chatmodel.Session) error {
	if _, ok := f.sessions[s.ID]; ok {
		return nil // idempotent
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = f.tick()
	}
	s.UpdatedAt = s.CreatedAt
	cp := *s
	f.sessions[s.ID] = &cp
	return nil
}
func (f *fakeRepo) TouchSession(_ context.Context, id string) error {
	if s, ok := f.sessions[id]; ok {
		s.UpdatedAt = f.tick()
	}
	return nil
}
func (f *fakeRepo) ListSessions(_ context.Context, userID uint) ([]chatmodel.Session, error) {
	var out []chatmodel.Session
	for _, s := range f.sessions {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}
func (f *fakeRepo) DeleteSession(_ context.Context, id string) error {
	delete(f.sessions, id)
	delete(f.messages, id)
	return nil
}
func (f *fakeRepo) AddMessage(_ context.Context, m *chatmodel.Message) error {
	m.CreatedAt = f.tick()
	f.messages[m.SessionID] = append(f.messages[m.SessionID], *m)
	return nil
}
func (f *fakeRepo) ListMessages(_ context.Context, sessionID string) ([]chatmodel.Message, error) {
	return f.messages[sessionID], nil
}

func newSvc() (Service, *fakeRepo) {
	repo := newFakeRepo()
	return NewService(repo, ai.NewMockClient(), time.Second), repo
}

const sid = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

func TestAsk_CreatesSessionAndMessages(t *testing.T) {
	svc, repo := newSvc()
	res, err := svc.Ask(context.Background(), 1, "pln", chatdto.AskRequest{SessionID: sid, Question: "halo?"})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if res.Answer == "" || res.SessionID != sid {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, ok := repo.sessions[sid]; !ok {
		t.Fatal("session not created")
	}
	msgs := repo.messages[sid]
	if len(msgs) != 2 || msgs[0].Role != chatmodel.RoleUser || msgs[1].Role != chatmodel.RoleAssistant {
		t.Fatalf("expected user+assistant messages, got %+v", msgs)
	}
}

func TestAsk_InvalidSession(t *testing.T) {
	svc, _ := newSvc()
	_, err := svc.Ask(context.Background(), 1, "pln", chatdto.AskRequest{SessionID: "bad!id", Question: "x"})
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession, got %v", err)
	}
}

func TestAsk_OwnershipMismatch(t *testing.T) {
	svc, repo := newSvc()
	repo.sessions[sid] = &chatmodel.Session{ID: sid, UserID: 1, OrganizationCode: "pln", CreatedAt: time.Now()}
	// user 2 tries to use user 1's session id
	_, err := svc.Ask(context.Background(), 2, "icon", chatdto.AskRequest{SessionID: sid, Question: "hijack?"})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestAsk_SlidingWindowEvictsOldest(t *testing.T) {
	svc, repo := newSvc()
	// pre-create 20 sessions for user 1 with increasing createdAt
	first := ""
	for i := 0; i < maxSessionsPerUser; i++ {
		id := fmt.Sprintf("sess-0000-0000-0000-%012d", i)
		repo.sessions[id] = &chatmodel.Session{ID: id, UserID: 1, CreatedAt: time.Unix(int64(i+1), 0)}
		if i == 0 {
			first = id
		}
	}
	// 21st ask (new session) must evict the oldest, keeping count at 20
	if _, err := svc.Ask(context.Background(), 1, "pln", chatdto.AskRequest{SessionID: sid, Question: "q"}); err != nil {
		t.Fatalf("ask: %v", err)
	}
	if _, ok := repo.sessions[first]; ok {
		t.Fatal("oldest session was not evicted")
	}
	n, _ := repo.CountSessions(context.Background(), 1)
	if n != maxSessionsPerUser {
		t.Fatalf("expected %d sessions, got %d", maxSessionsPerUser, n)
	}
}

func TestGetAndDelete_Ownership(t *testing.T) {
	svc, repo := newSvc()
	repo.sessions[sid] = &chatmodel.Session{ID: sid, UserID: 1}
	if _, err := svc.GetSessionDetail(context.Background(), sid, 2); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("get by non-owner: expected ErrSessionNotFound, got %v", err)
	}
	if err := svc.DeleteSession(context.Background(), sid, 2); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("delete by non-owner: expected ErrSessionNotFound, got %v", err)
	}
	if err := svc.DeleteSession(context.Background(), sid, 1); err != nil {
		t.Fatalf("delete by owner: %v", err)
	}
}
