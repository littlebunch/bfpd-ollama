package vectordb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gmoore/bfpd-ollama/pkg/models"
	"github.com/google/uuid"
)

// QdrantClient wraps Qdrant vector DB operations using HTTP API
type QdrantClient struct {
	baseURL        string
	collectionName string
	vectorSize     uint64
	httpClient     *http.Client
}

// NewQdrantClient creates a new Qdrant client
func NewQdrantClient(url, collectionName string, vectorSize int) (*QdrantClient, error) {
	return &QdrantClient{
		baseURL:        strings.TrimSuffix(url, "/"),
		collectionName: collectionName,
		vectorSize:     uint64(vectorSize),
		httpClient:     &http.Client{},
	}, nil
}

// VectorParams defines vector configuration for Qdrant collection
type VectorParams struct {
	Size     uint64 `json:"size"`
	Distance string `json:"distance"`
}

// CollectionConfig defines collection creation parameters
type CollectionConfig struct {
	Vectors VectorParams `json:"vectors"`
}

// CreateCollection creates a new collection in Qdrant
func (qc *QdrantClient) CreateCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections/%s", qc.baseURL, qc.collectionName)

	payload := CollectionConfig{
		Vectors: VectorParams{
			Size:     qc.vectorSize,
			Distance: "Cosine",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal collection config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Point represents a point to be upserted into Qdrant
type Point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

// UpsertRequest is the request payload for upserting points
type UpsertRequest struct {
	Points []Point `json:"points"`
}

// UpsertPoints adds or updates vector points in the collection
func (qc *QdrantClient) UpsertPoints(ctx context.Context, docs []models.FoodDocument) error {
	if len(docs) == 0 {
		return nil
	}

	// Convert documents to points
	points := make([]Point, len(docs))
	for i, doc := range docs {
		// Generate a UUID for each point (Qdrant requires UUID or uint64 format)
		pointID := uuid.New().String()

		points[i] = Point{
			ID:     pointID,
			Vector: doc.Embedding,
			Payload: map[string]interface{}{
				"content": doc.Content,
				"title":   doc.Title,
				"fdc_id":  doc.FdcID,
				"doc_id":  doc.ID, // Store original doc ID in payload for reference
			},
		}
		// Add any additional metadata
		if doc.Metadata != nil {
			for k, v := range doc.Metadata {
				points[i].Payload[k] = v
			}
		}
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", qc.baseURL, qc.collectionName)

	payload := UpsertRequest{Points: points}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal points: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upsert points: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SearchRequest is the request payload for searching points
type SearchRequest struct {
	Vector         []float32 `json:"vector"`
	Limit          int       `json:"limit"`
	ScoreThreshold float32   `json:"score_threshold,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// SearchResponse is the response from a search request
type SearchResponse struct {
	Result []SearchResult `json:"result"`
}

// Search performs a similarity search in the collection
func (qc *QdrantClient) Search(ctx context.Context, embedding []float32, topK int, scoreThreshold float32) ([]models.RAGSource, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", qc.baseURL, qc.collectionName)

	payload := SearchRequest{
		Vector:         embedding,
		Limit:          topK,
		ScoreThreshold: scoreThreshold,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	// Convert search results to RAGSource
	sources := make([]models.RAGSource, len(searchResp.Result))
	for i, result := range searchResp.Result {
		sources[i] = models.RAGSource{
			ID:       result.ID,
			Score:    result.Score,
			Metadata: result.Payload,
		}
		// Extract content from payload if available
		if content, ok := result.Payload["content"]; ok {
			if contentStr, ok := content.(string); ok {
				sources[i].Content = contentStr
			}
		}
	}

	return sources, nil
}

// DeleteCollection deletes the collection
func (qc *QdrantClient) DeleteCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections/%s", qc.baseURL, qc.collectionName)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content is also acceptable
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete collection: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// RecreateCollection drops and recreates the collection
func (qc *QdrantClient) RecreateCollection(ctx context.Context) error {
	// Delete collection (ignore errors if it doesn't exist)
	_ = qc.DeleteCollection(ctx)

	// Create new collection
	return qc.CreateCollection(ctx)
}

// VersionInfo represents the Qdrant version info
type VersionInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// Health checks if Qdrant is healthy by fetching version info
func (qc *QdrantClient) Health(ctx context.Context) error {
	url := qc.baseURL

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant health check failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("failed to decode version response: %w", err)
	}

	if info.Title == "" || info.Version == "" {
		return fmt.Errorf("qdrant returned invalid version info")
	}

	return nil
}

// Close closes the Qdrant client connection
func (qc *QdrantClient) Close() error {
	// HTTP client doesn't need explicit closing, but we can add cleanup if needed
	return nil
}
