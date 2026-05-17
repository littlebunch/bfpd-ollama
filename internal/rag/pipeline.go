package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/gmoore/bfpd-ollama/internal/ollama"
	"github.com/gmoore/bfpd-ollama/internal/vectordb"
	"github.com/gmoore/bfpd-ollama/pkg/models"
)

// Pipeline orchestrates the RAG (Retrieval Augmented Generation) workflow
type Pipeline struct {
	ollamaClient   *ollama.Client
	qdrantClient   *vectordb.QdrantClient
	config         models.RAGConfig
	promptTemplate string
}

// NewPipeline creates a new RAG pipeline
func NewPipeline(
	ollamaClient *ollama.Client,
	qdrantClient *vectordb.QdrantClient,
	config models.RAGConfig,
) *Pipeline {
	return &Pipeline{
		ollamaClient:   ollamaClient,
		qdrantClient:   qdrantClient,
		config:         config,
		promptTemplate: defaultPromptTemplate,
	}
}

// Query performs a full RAG query: retrieve, augment, generate
func (p *Pipeline) Query(ctx context.Context, userQuery string) (*models.ChatResponse, error) {
	// Step 1: Embed the user query
	embedding, err := p.ollamaClient.Embed(ctx, userQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Step 2: Retrieve relevant documents
	sources, err := p.qdrantClient.Search(ctx, embedding, p.config.TopK, p.config.ScoreThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to search vector db: %w", err)
	}

	// Step 3: Build context from retrieved documents
	context := p.buildContext(sources)

	// Step 4: Generate response with context
	systemPrompt := fmt.Sprintf(p.promptTemplate, context)

	response, err := p.ollamaClient.Generate(ctx, userQuery, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	return &models.ChatResponse{
		Content: response,
		Sources: sources,
	}, nil
}

// buildContext creates a formatted context string from retrieved documents
func (p *Pipeline) buildContext(sources []models.RAGSource) string {
	if len(sources) == 0 {
		return "No relevant information found in the knowledge base."
	}

	var contextParts []string
	for i, source := range sources {
		contextParts = append(contextParts, fmt.Sprintf("[%d] (Score: %.2f)\n%s", i+1, source.Score, source.Content))
	}

	return strings.Join(contextParts, "\n\n")
}

// EmbedAndStore embeds food documents and stores them in Qdrant
func (p *Pipeline) EmbedAndStore(ctx context.Context, docs []models.FoodDocument) error {
	if len(docs) == 0 {
		return nil
	}

	// Embed all documents
	for i := range docs {
		embedding, err := p.ollamaClient.Embed(ctx, docs[i].Content)
		if err != nil {
			return fmt.Errorf("failed to embed document %s: %w", docs[i].ID, err)
		}
		docs[i].Embedding = embedding
	}

	// Store in Qdrant
	if err := p.qdrantClient.UpsertPoints(ctx, docs); err != nil {
		return fmt.Errorf("failed to store embeddings: %w", err)
	}

	return nil
}

// SetPromptTemplate allows customizing the prompt template
func (p *Pipeline) SetPromptTemplate(template string) {
	p.promptTemplate = template
}

const defaultPromptTemplate = `You are a helpful food nutrition assistant. Answer questions based on the provided food database information.

FOOD DATA CONTEXT:
%s

INSTRUCTIONS:
- If the context doesn't contain relevant information, say so honestly.
- Provide specific nutritional information when available.
- Cite sources using the [n] references provided.
- Be conversational but factual.`
