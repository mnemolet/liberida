package db

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Manager struct {
	db *gorm.DB
}

// NewManager creates a new database manager
func NewManager(dbPath string) (*Manager, error) {
	// Ensure directory exists
	// dbPath should be like: /home/user/.liberida/chat.db

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Change to Info for debugging
	}

	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Auto-migrate the schemas
	if err := db.AutoMigrate(&ChatSession{}, &ChatMessage{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Manager{db: db}, nil
}

// CreateSession creates a new chat session
func (m *Manager) CreateSession(title string) (*ChatSession, error) {
	if title == "" {
		title = "New Chat"
	}

	session := &ChatSession{
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := m.db.Create(session)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to create session: %w", result.Error)
	}

	return session, nil
}

// GetSession retrieves a session by ID with its messages
func (m *Manager) GetSession(id uint) (*ChatSession, error) {
	var session ChatSession
	result := m.db.Preload("Messages").First(&session, id)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get session: %w", result.Error)
	}
	return &session, nil
}

// ListSessions returns all sessions, ordered by most recent
func (m *Manager) ListSessions(limit int) ([]ChatSession, error) {
	var sessions []ChatSession
	query := m.db.Order("updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	result := query.Find(&sessions)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", result.Error)
	}
	return sessions, nil
}

// DeleteSession removes a session and all its messages (cascade)
func (m *Manager) DeleteSession(id uint) error {
	result := m.db.Delete(&ChatSession{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

// AddMessage adds a message to a session
func (m *Manager) AddMessage(sessionID uint, role, message string) (*ChatMessage, error) {
	msg := &ChatMessage{
		SessionID: sessionID,
		Role:      role,
		Message:   message,
		CreatedAt: time.Now(),
	}

	result := m.db.Create(msg)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to add message: %w", result.Error)
	}

	// Update session's updated_at
	m.db.Model(&ChatSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now())

	return msg, nil
}

// GetMessages retrieves messages for a session, ordered by creation time
func (m *Manager) GetMessages(sessionID uint) ([]ChatMessage, error) {
	var messages []ChatMessage
	result := m.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&messages)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get messages: %w", result.Error)
	}
	return messages, nil
}

// UpdateSessionTitle updates the title of a session
func (m *Manager) UpdateSessionTitle(sessionID uint, title string) error {
	result := m.db.Model(&ChatSession{}).Where("id = ?", sessionID).Update("title", title)
	if result.Error != nil {
		return fmt.Errorf("failed to update session title: %w", result.Error)
	}
	return nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
