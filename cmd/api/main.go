package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gmoore/bfpd-ollama/internal/chat"
	"github.com/gmoore/bfpd-ollama/internal/ollama"
	"github.com/gmoore/bfpd-ollama/internal/rag"
	"github.com/gmoore/bfpd-ollama/internal/vectordb"
	"github.com/gmoore/bfpd-ollama/pkg/config"
	"github.com/gmoore/bfpd-ollama/pkg/models"
)

var (
	cfg          *config.Config
	ollamaClient *ollama.Client
	qdrantClient *vectordb.QdrantClient
	ragPipeline  *rag.Pipeline
	chatManager  *chat.Manager
)

func main() {
	// Load configuration
	var err error
	cfg, err = config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Ollama client
	ollamaClient = ollama.NewClient(cfg.Ollama.URL, cfg.Ollama.Model, cfg.Ollama.EmbeddingModel)

	// Check Ollama health
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := ollamaClient.Health(ctx); err != nil {
		cancel()
		log.Fatalf("Ollama service unavailable: %v", err)
	}
	cancel()

	// Initialize Qdrant client
	qdrantClient, err = vectordb.NewQdrantClient(cfg.VectorDB.URL, cfg.VectorDB.CollectionName, cfg.VectorDB.VectorSize)
	if err != nil {
		log.Fatalf("Failed to initialize Qdrant: %v", err)
	}
	defer qdrantClient.Close()

	// Initialize RAG pipeline
	ragConfig := models.RAGConfig{
		TopK:           cfg.RAG.TopK,
		ScoreThreshold: cfg.RAG.ScoreThreshold,
		ChunkSize:      cfg.RAG.ChunkSize,
		ChunkOverlap:   cfg.RAG.ChunkOverlap,
	}
	ragPipeline = rag.NewPipeline(ollamaClient, qdrantClient, ragConfig)

	// Initialize chat manager
	chatManager = chat.NewManager()

	// Setup Gin router
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	setupRoutes(router)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting API server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func setupRoutes(router *gin.Engine) {
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Chat endpoints
	router.POST("/chat", handleChat)
	router.POST("/conversations", handleCreateConversation)
	router.GET("/conversations/:id", handleGetConversation)

	// RAG endpoints
	router.POST("/rag/search", handleRAGSearch)

	// Admin endpoints
	router.POST("/admin/init-vectors", handleInitVectors)
}

// handleChat handles a chat message
func handleChat(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = chatManager.CreateConversation()
	}

	// Add user message to conversation
	if err := chatManager.AddMessage(conversationID, "user", req.Message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate response using RAG if requested
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var resp *models.ChatResponse
	var err error

	if req.UseRAG {
		resp, err = ragPipeline.Query(ctx, req.Message)
	} else {
		response, err := ollamaClient.Generate(ctx, req.Message, "You are a helpful assistant.")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp = &models.ChatResponse{
			Content: response,
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp.ID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	resp.ConversationID = conversationID
	resp.Role = "assistant"
	resp.Timestamp = time.Now().Unix()

	// Add assistant response to conversation
	chatManager.AddMessage(conversationID, "assistant", resp.Content)

	c.JSON(http.StatusOK, resp)
}

// handleCreateConversation creates a new conversation
func handleCreateConversation(c *gin.Context) {
	conversationID := chatManager.CreateConversation()
	c.JSON(http.StatusOK, gin.H{
		"id": conversationID,
	})
}

// handleGetConversation retrieves a conversation
func handleGetConversation(c *gin.Context) {
	conversationID := c.Param("id")
	conv, err := chatManager.GetConversation(conversationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

// handleRAGSearch performs a RAG search without generating a response
func handleRAGSearch(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Embed query
	embedding, err := ollamaClient.Embed(ctx, req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Search Qdrant
	sources, err := qdrantClient.Search(ctx, embedding, 5, 0.5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
	})
}

// handleInitVectors initializes the vector database (placeholder)
func handleInitVectors(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := qdrantClient.RecreateCollection(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Vector database initialized",
	})
}
