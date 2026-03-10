package db

import (
	"time"
)

// ChatSession matches MnemoLet Python ChatSession model
type ChatSession struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"size:255;not null;default:'New Chat'" json:"title"`
	CreatedAt time.Time `gorm:"not null;index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationship to messages
	Messages []ChatMessage `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
}

// ChatMessage matches MnemoLet Python ChatMessage model
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID uint      `gorm:"not null;index" json:"session_id"`
	Role      string    `gorm:"size:50;not null" json:"role"` // 'user' or 'assistant'
	Message   string    `gorm:"type:text;not null" json:"message"`
	CreatedAt time.Time `gorm:"not null;index" json:"created_at"`

	// Relationship to session
	Session ChatSession `gorm:"foreignKey:SessionID" json:"-"`
}

func (ChatSession) TableName() string {
	return "chat_sessions"
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}
