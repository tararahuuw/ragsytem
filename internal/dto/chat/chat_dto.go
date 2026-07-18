// Package chat holds request/response DTOs for the conversation module.
package chat

import "time"

// AskRequest is the payload for POST /chat/ask. session_id is a client-generated
// UUID (same id = same conversation; new id = new conversation).
type AskRequest struct {
	SessionID string `json:"session_id" binding:"required" example:"3f2504e0-4f89-41d3-9a0c-0305e82c3301"`
	Question  string `json:"question" binding:"required" example:"Ringkas dokumen laporan tahunan"`
}

// AskResponse is returned from POST /chat/ask.
type AskResponse struct {
	SessionID string `json:"session_id"`
	Answer    string `json:"answer"`
}

// MessageResponse is one turn in a conversation.
type MessageResponse struct {
	ID        uint      `json:"id"`
	Role      string    `json:"role" example:"assistant"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionResponse is a conversation summary (for the list).
type SessionResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionDetailResponse is a conversation with its messages.
type SessionDetailResponse struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	OrganizationCode string            `json:"organization_code"`
	Messages         []MessageResponse `json:"messages"`
}
