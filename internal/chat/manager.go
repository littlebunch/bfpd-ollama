package chat

import (
	"sync"
	"time"

	"github.com/gmoore/bfpd-ollama/pkg/models"
	"github.com/google/uuid"
)

// Manager manages chat conversations
type Manager struct {
	conversations map[string]*models.Conversation
	mu            sync.RWMutex
}

// NewManager creates a new chat manager
func NewManager() *Manager {
	return &Manager{
		conversations: make(map[string]*models.Conversation),
	}
}

// CreateConversation creates a new conversation
func (m *Manager) CreateConversation() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	now := time.Now().Unix()

	m.conversations[id] = &models.Conversation{
		ID:       id,
		Messages: []models.ChatMessage{},
		Created:  now,
		Updated:  now,
	}

	return id
}

// AddMessage adds a message to a conversation
func (m *Manager) AddMessage(conversationID, role, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return ErrConversationNotFound
	}

	conv.Messages = append(conv.Messages, models.ChatMessage{
		Role:    role,
		Content: content,
	})
	conv.Updated = time.Now().Unix()

	return nil
}

// GetConversation retrieves a conversation
func (m *Manager) GetConversation(conversationID string) (*models.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return nil, ErrConversationNotFound
	}

	return conv, nil
}

// GetRecentMessages gets the last N messages from a conversation
func (m *Manager) GetRecentMessages(conversationID string, limit int) ([]models.ChatMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return nil, ErrConversationNotFound
	}

	if len(conv.Messages) <= limit {
		return conv.Messages, nil
	}

	return conv.Messages[len(conv.Messages)-limit:], nil
}

// BuildContextWindow builds a context window from recent messages
func (m *Manager) BuildContextWindow(conversationID string, maxMessages int) (string, error) {
	messages, err := m.GetRecentMessages(conversationID, maxMessages)
	if err != nil {
		return "", err
	}

	var context string
	for _, msg := range messages {
		context += msg.Role + ": " + msg.Content + "\n"
	}

	return context, nil
}

// DeleteConversation deletes a conversation
func (m *Manager) DeleteConversation(conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.conversations, conversationID)
	return nil
}

// ListConversations lists all conversation IDs
func (m *Manager) ListConversations() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ids []string
	for id := range m.conversations {
		ids = append(ids, id)
	}

	return ids
}
