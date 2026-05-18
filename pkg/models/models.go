package models

// ChatMessage represents a single message in a conversation
type ChatMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Message text
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
	UseRAG         bool   `json:"use_rag,omitempty"`
}

// ChatResponse represents the API response for a chat message
type ChatResponse struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversation_id"`
	Role           string      `json:"role"`
	Content        string      `json:"content"`
	Sources        []RAGSource `json:"sources,omitempty"`
	Timestamp      int64       `json:"timestamp"`
}

// RAGSource represents a source document retrieved during RAG
type RAGSource struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Score    float32                `json:"score"`
}

// EmbeddingRequest represents a request to embed text
type EmbeddingRequest struct {
	Text string `json:"text"`
}

// EmbeddingResponse represents embedded vector data
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// FoodDocument represents a food item to be embedded and stored in vector DB
type FoodDocument struct {
	ID        string
	FdcID     int64
	Content   string // Combined description, ingredients, etc.
	Title     string
	Metadata  map[string]interface{}
	Embedding []float32
}

// Conversation represents a chat conversation
type Conversation struct {
	ID       string
	Messages []ChatMessage
	Created  int64
	Updated  int64
}

// RAGConfig holds RAG pipeline configuration
type RAGConfig struct {
	TopK           int     // Number of top results to retrieve
	ScoreThreshold float32 // Minimum similarity score
	ChunkSize      int     // Size of text chunks in tokens
	ChunkOverlap   int     // Overlap between chunks
	EmbeddingModel string  // Which embedding model to use
}
