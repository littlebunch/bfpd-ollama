# Quick Start Guide for LLM + RAG Food Chat Application

This guide walks you through setting up and running the food nutrition AI application.

## Step 1: Verify Prerequisites

```bash
# Check Go version (need 1.22+)
go version

# Check Docker
docker --version
docker-compose --version

# MySQL should already be running with GBFPD data
mysql -u gmoore -p -e "SELECT COUNT(*) FROM gbfpd.branded_foods;"
```

## Step 2: Install Dependencies

```bash
# Download Go modules
make deps
```

## Step 3: Start Services

```bash
# Start Ollama and Qdrant in Docker
make docker-up

# Wait 30 seconds for services to be healthy
sleep 30

# Verify services are running
curl http://localhost:11434/api/tags
curl http://localhost:6333/health
```

## Step 4: Download Models

This will download the LLM and embedding models (~2.5GB total). Takes 5-10 minutes:

```bash
make install-models
```

Monitor progress:
```bash
# In another terminal, watch model downloads
ollama ps
```

## Step 5: Seed Vector Database

This embeds food data from MySQL and populates Qdrant:

```bash
make seed-vectors
```

This will:
1. Connect to MySQL and fetch branded foods
2. Split into chunks (512 tokens)
3. Generate embeddings using Ollama
4. Store vectors in Qdrant

Expected output:
```
✓ Connected to MySQL
✓ Connected to Ollama
✓ Connected to Qdrant
✓ Collection created

Loading food data from MySQL...
✓ Loaded 1000 documents

Embedding documents...
  ✓ Processed 100/1000 documents
  ✓ Processed 200/1000 documents
  ...
✓ Vector database seeding complete!
```

## Step 6: Run Chat Application

### Option A: CLI Interface

```bash
make run-cli
```

Try these queries:
```
You: What are the top nutrients in eggs?
You: Compare protein content in chicken vs beef
You: Tell me about Coca-Cola nutrition
You: exit
```

### Option B: REST API Server

Terminal 1:
```bash
make run-api
```

Terminal 2 (test the API):
```bash
# Create new conversation
curl -X POST http://localhost:8080/conversations
# Response: {"id":"<conv_id>"}

# Send message with RAG
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What are the nutrients in apples?",
    "conversation_id": "<conv_id>",
    "use_rag": true
  }'

# Search without generating (just retrieval)
curl -X POST http://localhost:8080/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query": "high protein foods"}'
```

## Troubleshooting

### Models didn't download

```bash
# Check Ollama is running
docker-compose ps

# Restart and retry
make docker-down
make docker-up
sleep 30
ollama pull neural-chat
ollama pull nomic-embed-text
```

### Vector seeding failed

```bash
# Check MySQL connection
mysql -u gmoore -p -e "SELECT COUNT(*) FROM gbfpd.branded_foods;"

# Reinitialize vector DB
curl -X POST http://localhost:8080/admin/init-vectors

# Retry seeding
make seed-vectors
```

### Out of memory

Reduce context window in `config.yaml`:
```yaml
ollama:
  context_window: 2048  # was 4096
```

Or use smaller model:
```yaml
ollama:
  model: mistral  # instead of neural-chat
```

## Project Structure Reference

```
├── cmd/
│   ├── api/           REST API server
│   ├── chat-cli/      Interactive CLI
│   └── seed-vectors/  Vector DB population
├── internal/
│   ├── ollama/        LLM integration
│   ├── rag/           RAG pipeline
│   ├── vectordb/      Qdrant operations
│   ├── chat/          Conversation management
│   └── embedding/     Text chunking
├── pkg/
│   ├── models/        Data structures
│   └── config/        Configuration
└── config.yaml        Settings
```

## Next Steps

1. **Customize prompt template**: Edit [rag/pipeline.go](internal/rag/pipeline.go#L95)
2. **Add web UI**: Scaffold Next.js frontend
3. **Persistent storage**: Implement conversation database
4. **Performance**: Add response caching layer
5. **Multi-user**: Implement authentication

## Stop Services

When done:

```bash
# Stop Ollama and Qdrant
make docker-down

# Cleanup
make clean
```

## Getting Help

Check logs:
```bash
# API logs
tail -f nohup.out

# Docker service logs
docker-compose logs -f ollama
docker-compose logs -f qdrant

# Ollama specific
ollama list
```

Test endpoints:
```bash
# Health check
curl http://localhost:8080/health

# Check collection exists
curl http://localhost:6333/collections/food_vectors

# List Ollama models
curl http://localhost:11434/api/tags
```
