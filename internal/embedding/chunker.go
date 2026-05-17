package embedding

import (
	"strings"
)

// Chunker splits text into overlapping chunks
type Chunker struct {
	chunkSize   int
	overlapSize int
}

// NewChunker creates a new text chunker
func NewChunker(chunkSize, overlapSize int) *Chunker {
	if overlapSize > chunkSize {
		overlapSize = chunkSize / 2
	}
	return &Chunker{
		chunkSize:   chunkSize,
		overlapSize: overlapSize,
	}
}

// Split splits text into chunks by word count (approximate tokens)
func (c *Chunker) Split(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var chunks []string
	stride := c.chunkSize - c.overlapSize

	for i := 0; i < len(words); i += stride {
		end := i + c.chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[i:end], " ")
		chunks = append(chunks, chunk)

		// Stop if we've reached the end
		if end == len(words) {
			break
		}
	}

	return chunks
}

// ChunkWithMetadata splits text and returns chunks with metadata
type ChunkWithMetadata struct {
	Content  string
	Index    int
	Metadata map[string]interface{}
}

// SplitWithMetadata splits text and includes metadata for each chunk
func (c *Chunker) SplitWithMetadata(text string, metadata map[string]interface{}) []ChunkWithMetadata {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []ChunkWithMetadata{}
	}

	var chunks []ChunkWithMetadata
	stride := c.chunkSize - c.overlapSize

	for i := 0; i < len(words); i += stride {
		end := i + c.chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[i:end], " ")
		meta := make(map[string]interface{})
		for k, v := range metadata {
			meta[k] = v
		}
		meta["chunk_index"] = len(chunks)

		chunks = append(chunks, ChunkWithMetadata{
			Content:  chunk,
			Index:    len(chunks),
			Metadata: meta,
		})

		if end == len(words) {
			break
		}
	}

	return chunks
}
