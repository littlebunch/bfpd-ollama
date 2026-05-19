# Food Nutrition AI with LLM + RAG

A Go-based application for querying food nutrition data using local LLMs (Ollama) and Retrieval-Augmented Generation (RAG).

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Chat Interfaces                       │
│          (CLI, REST API, or Next.js Web UI)             │
└────────────┬────────────────────────────────────────────┘
             │
┌────────────▼─────────────────────────────────────────────┐
│              Go Backend Services                         │
│  ┌──────────────┬──────────────┬──────────────────────┐ │
│  │ Chat Manager │ RAG Pipeline │ Vector Operations   │ │
│  └──────────────┴──────────────┴──────────────────────┘ │
└────────────┬───────────────────┬──────────────────────────┘
             │                   │
    ┌────────▼────┐    ┌────────▼──────────┐
    │   Ollama    │    │  Qdrant (Vector)  │
    │ - LLM Infer │    │  + Food Data      │
    │ - Embeddings│    │  Semantic Search  │
    └─────────────┘    └───────────────────┘
          │                      │
    ┌─────▴──────────┬───────────┘
    │                │
    ▼                ▼
 MySQL Database  (PostgreSQL alt)
 Food Data
```

## Project Structure

```
.
├── cmd/
│   ├── api/              # REST API server
│   ├── chat-cli/         # CLI chat interface
│   └── seed-vectors/     # Vector DB population tool
├── internal/
│   ├── ollama/           # Ollama client wrapper
│   ├── vectordb/         # Qdrant operations
│   ├── rag/              # RAG pipeline
│   ├── chat/             # Chat history management
│   └── embedding/        # Text chunking utilities
├── pkg/
│   ├── models/           # Data structures
│   └── config/           # Configuration
├── web/                  # Next.js web frontend
│   ├── app/              # App Router pages & layout
│   ├── components/       # React components
│   ├── lib/              # TypeScript types & API client
│   └── next.config.js    # Proxies /api/* → localhost:8080
├── config.yaml           # Configuration file
├── docker-compose.yml    # Ollama + Qdrant services
├── Makefile             # Build targets
└── go.mod               # Dependencies
```

## Prerequisites

- Go 1.22+
- Docker & Docker Compose
- MySQL 8.0+ (with GBFPD database populated via `import_gbfpd.go`)
- At least 4GB RAM

## Quick Start

### 1. Start Dependencies

```bash
# Start Ollama and Qdrant
make docker-up

# Pull LLM models (takes a few minutes)
make install-models
```

### 2. Seed Vector Database

```bash
# Embed food data and populate Qdrant
make seed-vectors
```

### 3. Run Chat Interface

Choose one:

```bash
# REST API server
make run-api

# Interactive CLI
make run-cli
```

## Usage

### CLI Chat

```bash
$ make run-cli

🍽️  Food Nutrition AI Chat
Using RAG: true

You: What are the nutrients in Coca-Cola?
Assistant: Based on the food database, Coca-Cola contains...
📚 Sources:
[1] Score: 0.92
    Title: Coca-Cola Classic
    Calories: 140 kcal, Carbs: 39g, ...
```

### Web UI

```bash
# Start the Go API server first (port 8080)
make run-api

# In a separate terminal, start the Next.js frontend (port 3000)
make run-web
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

**Features:**
- 💬 Conversational chat interface
- 🔍 RAG toggle — enable/disable food database search per message
- 📚 Source citations displayed below assistant responses (click to expand)
- 🟢 Live backend health indicator
- ✨ Suggested prompts on an empty chat

### REST API

```bash
# Create conversation
curl -X POST http://localhost:8080/conversations

# Send message with RAG
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Tell me about eggs",
    "conversation_id": "conv_...",
    "use_rag": true
  }'

# Search vector database
curl -X POST http://localhost:8080/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query": "high protein foods"}'
```

## Configuration

Edit `config.yaml`:

```yaml
server:
  host: localhost
  port: 8080

database:
  host: localhost
  user: ${USER}
  password: ${PASSWORD}
  database: gbfpd

ollama:
  url: http://localhost:11434
  model: neural-chat              # LLM model
  embedding_model: nomic-embed-text  # Embedding model

vectordb:
  url: http://localhost:6333
  collection_name: food_vectors
  vector_size: 768                # Must match embedding model

rag:
  topk: 5                         # Results to retrieve
  score_threshold: 0.5            # Similarity cutoff
  chunk_size: 512                 # Tokens per chunk
  chunk_overlap: 128              # Overlap tokens
```

## Development

### Build

```bash
# Build all
make build

# Build specific target
make build-api
make build-cli
make build-seed
```

### Testing

```bash
make test
```

### Dependencies

```bash
make deps
```

## API Endpoints

### Chat Operations

- `POST /chat` - Send message with optional RAG
- `POST /conversations` - Create new conversation
- `GET /conversations/:id` - Get conversation history

### RAG Operations

- `POST /rag/search` - Search vector database

### Admin

- `POST /admin/init-vectors` - Initialize vector database
- `GET /health` - Health check

## How RAG Works

1. **Query Embedding**: User query is embedded using Ollama
2. **Semantic Search**: Query embedding searched against food vectors in Qdrant
3. **Context Injection**: Top-K results combined into system prompt
4. **Generation**: LLM generates response with context awareness
5. **Response**: Answer returned with source citations

## Supported Models

### LLM Models
- `neural-chat` (default, ~5GB)
- `llama2` (~4GB)
- `mistral` (~5GB)

### Embedding Models
- `nomic-embed-text` (default, 768d)
- `all-minilm` (384d)
- `mxbai-embed-large` (1024d)

Pull via: `ollama pull <model>`

## Performance Tips

- Increase `topk` in RAG for more context
- Use smaller `chunk_size` for finer-grained retrieval
- Enable batching in seed-vectors for faster embedding
- Use `neural-chat` for balance of speed/quality

## Troubleshooting

### Ollama not responding
```bash
# Check if service is running
curl http://localhost:11434/api/tags

# Restart
docker-compose restart ollama
```

### Qdrant connection fails
```bash
# Check vector DB
curl http://localhost:6333/health

# Reseed vectors
make seed-vectors
```

### Out of memory
- Reduce `ollama.context_window` in config
- Use smaller LLM model (e.g., `mistral` instead of `llama2`)
- Increase Docker memory limits

## Next Steps

1. ~~Implement web UI with Next.js frontend~~ ✅
2. Add streaming responses for better UX
3. Implement persistent conversation storage
4. Add fine-tuning pipeline for domain-specific LLM
5. Multi-language support
6. Caching layer for repeated queries

## License

MIT
