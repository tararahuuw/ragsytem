// Package chat holds persistence entities for the conversation module.
package chat

import "time"

// Message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Session is one conversation thread. ID is a client-generated UUID (also used
// as the AI thread_id). Scoped per user; organizationCode drives the AI's
// retrieval access filter.
type Session struct {
	ID               string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID           uint      `gorm:"index" json:"user_id"`
	OrganizationCode string    `gorm:"index" json:"organization_code"`
	Title            string    `json:"title"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Message is one turn in a session (role = user | assistant).
type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID string    `gorm:"index" json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Explicit table names (avoid the too-generic default "sessions"/"messages").
func (Session) TableName() string { return "chat_sessions" }
func (Message) TableName() string { return "chat_messages" }
