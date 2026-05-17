# Local LLM Setup Guide - BFPD Importer

Complete guide to replace Copilot with a local Mistral 7B model using Ollama + Continue.dev.

**Total Setup Time: ~60 minutes**
- Download: ~10 min (depends on internet)
- Installation: ~15 min
- Configuration: ~10 min
- Testing: ~5 min
- Tuning: ~20 min (optional)

---

## Prerequisites

- **macOS** with M1/M2/Intel CPU
- **8GB RAM minimum** (16GB+ recommended)
- **VS Code** installed
- **MySQL 8.0** running (already have)
- **Go 1.21+** installed (already have)

---

## Phase 1: Install Ollama (10 minutes)

### Step 1: Install via Homebrew
```bash
brew install ollama
```

Or download directly: https://ollama.ai/download/mac

### Step 2: Verify Installation
```bash
ollama --version
# Should output: ollama version 0.1.x (or similar)
```

---

## Phase 2: Download Model (10 minutes)

### Step 3: Pull Mistral 7B
```bash
ollama pull mistral
```

**Expected output:**
```
pulling manifest...
pulling 2dd940fee1ee... 100% ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 4.1 GB
pulling 2c46bc33b798... 100% ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 6.6 KB
pulling 2c7fb3762003... 100% ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 11 B
Digest: sha256:61e8...
```

**Time:** 5-10 minutes (depending on internet speed)
**Size:** ~4.1GB download, ~7.4GB disk space

### Step 4: Verify Model Installed
```bash
ollama list
# Should show:
# NAME            ID              SIZE      MODIFIED
# mistral:latest  2dd940fee1ee    4.1 GB    2 minutes ago
```

---

## Phase 3: Start Ollama Server (Persistent)

### Step 5: Start Ollama in Background (Option A - Recommended)

**Terminal 1 - Keep Ollama running:**
```bash
ollama serve
```

**Output should show:**
```
2024/04/28 14:32:10 "GET /api/tags HTTP/1.1" 200 123
Listening on 127.0.0.1:11434
```

**Keep this terminal window open.** Ollama will continue serving requests.

### Step 6: Verify Server is Running (Another Terminal)
```bash
# Test connection
curl http://localhost:11434/api/tags
# Should return JSON with model info
```

**Alternative: Start as macOS Service**
```bash
# If you want Ollama to auto-start (advanced)
brew services start ollama
brew services list | grep ollama
```

---

## Phase 4: Install VS Code Extension (5 minutes)

### Step 7: Install Continue Extension
1. Open VS Code
2. Go to Extensions (Cmd+Shift+X)
3. Search: `Continue`
4. Click "Install" on **Continue - Code Autocomplete**
5. Reload VS Code when prompted

### Step 8: Verify Installation
- You should see a "Continue" icon in the left sidebar
- Click it to open the Continue panel

---

## Phase 5: Configure Continue.dev (10 minutes)

### Step 9: Auto-Configuration (Easiest)

The configuration files are already created in your project:
```
.continue/
  ├── config.json           ← Main configuration
  ├── project-context.md    ← Project-specific info
  └── rules.md              ← Code generation rules
```

Continue.dev automatically uses these when editing files in the project.

### Step 10: Manual Configuration (If Needed)

Edit/create `~/.continue/config.json`:
```json
{
  "models": [
    {
      "title": "Mistral Local",
      "provider": "ollama",
      "model": "mistral",
      "apiBase": "http://localhost:11434",
      "contextLength": 4096,
      "maxTokens": 1024,
      "temperature": 0.5
    }
  ],
  "tabAutocompleteModel": {
    "title": "Mistral Local",
    "provider": "ollama",
    "model": "mistral",
    "apiBase": "http://localhost:11434",
    "contextLength": 2048,
    "maxTokens": 512,
    "temperature": 0.3
  }
}
```

### Step 11: Restart Continue.dev
1. In VS Code, open Command Palette (Cmd+Shift+P)
2. Type: `Continue: Restart`
3. Press Enter

---

## Phase 6: Test Setup (5 minutes)

### Step 12: Test Tab Autocomplete

1. Open `import_gbfpd.go`
2. Go to any function (e.g., `CleanValue`)
3. Start typing a new line
4. You should see autocomplete suggestions from Mistral within 1-3 seconds

**Example:**
```go
func (imp *Importer) NewFunction() {
    // Type here - should get suggestions from Mistral
```

### Step 13: Test Slash Commands

1. Select some code in `import_gbfpd.go`
2. Click the Continue icon in left sidebar
3. In chat box, type: `/comment`
4. Press Ctrl+Enter or click Send
5. Continue should generate a detailed comment

**Slash commands:**
- `/edit` - Suggest code edits
- `/comment` - Explain with comments
- `/doc` - Generate documentation
- `/explain` - Detailed explanation

### Step 14: Verify Everything

```bash
# Check Ollama is responding
curl -X POST http://localhost:11434/api/generate \
  -d '{"model":"mistral","prompt":"test","stream":false}' | jq .

# Check latency (should be < 500ms for short prompts)
time curl -X POST http://localhost:11434/api/generate \
  -d '{"model":"mistral","prompt":"Hello","stream":false}' > /dev/null
```

---

## Usage Guide

### Tab Autocomplete (Real-time)

**Enabled automatically when:**
- You're editing a `.go` file
- You pause typing for ~500ms
- Ollama is running

**Speed expectations:**
- First completion: 1-3 seconds (model warming up)
- Subsequent: 100-300ms per completion

### Chat Mode

1. Click Continue icon (left sidebar)
2. Type your question or command
3. Use `/` for slash commands
4. Press Ctrl+Enter to send

**Examples:**
```
/doc       → Generate JSDoc/Go doc comments
/explain   → Explain the selected code
/edit      → Suggest edits
/test      → Write unit tests
/refactor  → Refactor for readability
```

### Custom Commands

Edit `.continue/config.json` to add custom commands:
```json
"customCommands": [
  {
    "name": "test",
    "prompt": "Write Go unit tests for this function"
  }
]
```

---

## Troubleshooting

### Issue: "Connection refused" or Timeout

**Solution:**
1. Check Ollama is running: `curl http://localhost:11434/api/tags`
2. If not running, start it: `ollama serve`
3. In VS Code, restart Continue: Cmd+Shift+P → `Continue: Restart`

### Issue: Slow Completions (> 5 seconds)

**Causes:**
- Model still loading (first time)
- Low RAM available
- High CPU load

**Solutions:**
- Wait 30 seconds for model to fully load
- Check available RAM: `top` or Activity Monitor
- Reduce context: Edit `.continue/config.json`, set `contextLength: 2048`

### Issue: High RAM Usage (> 12GB)

**Solutions:**
1. Reduce max tokens in config:
```json
{
  "contextLength": 2048,
  "maxTokens": 512,
  "temperature": 0.5
}
```

2. Reduce parallel requests:
```bash
OLLAMA_NUM_PARALLEL=1 ollama serve
```

3. Use smaller model (CodeLlama 7B instead):
```bash
ollama pull codellama:7b
# Update config.json model to "codellama:7b"
```

### Issue: "Model not found" in Continue

**Solution:**
```bash
# Verify model is downloaded
ollama list

# If not there, pull it
ollama pull mistral

# Restart Ollama and Continue
```

### Issue: VS Code autocomplete not working

**Checklist:**
- [ ] Ollama running? `curl http://localhost:11434/api/tags`
- [ ] Model exists? `ollama list | grep mistral`
- [ ] Continue extension installed? Extensions panel
- [ ] Continue restarted? Cmd+Shift+P → `Continue: Restart`
- [ ] File is in project directory? (uses `.continue/config.json`)

---

## Performance Tuning

### For Maximum Speed

Edit `.continue/config.json`:
```json
{
  "models": [
    {
      "title": "Mistral Fast",
      "provider": "ollama",
      "model": "mistral",
      "apiBase": "http://localhost:11434",
      "contextLength": 2048,      // Reduced from 4096
      "maxTokens": 256,           // Reduced from 1024
      "temperature": 0.3,         // Lower = faster (more deterministic)
      "requestOptions": {
        "timeout": 3000          // 3 second timeout
      }
    }
  ]
}
```

### For Better Code Quality

```json
{
  "contextLength": 8192,         // Larger context
  "maxTokens": 2048,             // Longer responses
  "temperature": 0.7             // More creative
}
```

### For Lower RAM Usage

```bash
# Start with reduced parallelism
OLLAMA_NUM_PARALLEL=1 OLLAMA_NUM_GPU=1 ollama serve
```

Or use smaller model:
```bash
ollama pull neural-chat:7b
# Update config model: "neural-chat:7b"
```

---

## Easy Startup Shortcuts

Use the provided Makefile:

```bash
# First time setup
make init

# Start Ollama (in new terminal)
make ollama-start

# Quick test
make test

# Full import
make run

# Check status
make ollama-status

# Stop Ollama
make ollama-stop
```

---

## Migrating from Copilot

### Step 1: Disable Copilot (Optional)
```json
// VS Code settings.json
{
  "github.copilot.enable": {
    "*": false
  }
}
```

### Step 2: Test Continue on Simple File
- Open a small `.go` file
- Type some code and verify autocomplete works
- Try a slash command

### Step 3: Migrate to Main File
- Open `import_gbfpd.go`
- Use Continue for:
  - Adding new CSV import functions
  - Refactoring batch processing
  - Generating documentation
  - Writing tests

### Step 4: Fine-tune Settings

Adjust in `.continue/config.json` based on:
- **Speed**: Reduce `contextLength` and `maxTokens`
- **Quality**: Increase `contextLength` and `maxTokens`
- **Cost**: No cost! All local and private

---

## Privacy & Security

✅ **Completely Private**
- No data sent to cloud
- All processing on your machine
- No Copilot license tracking
- No Microsoft/OpenAI access

✅ **Offline Operation**
- Works without internet after initial download
- Perfect for sensitive code
- No telemetry

✅ **Full Control**
- Own all generated code
- No licensing restrictions
- Can modify model if needed

---

## Next Steps

1. **Start using it immediately:**
   - Open a Go file
   - Start typing (autocomplete)
   - Select code and use `/comment`

2. **Fine-tune configuration:**
   - `.continue/project-context.md` - Already configured with BFPD patterns
   - `.continue/rules.md` - Code generation guidelines

3. **Optional enhancements:**
   - Install additional models
   - Create custom slash commands
   - Set up keyboard shortcuts

---

## Useful Commands

```bash
# Check Ollama status
ollama list

# Show running models
ollama ps

# Stop a model
ollama stop mistral

# Show model info
ollama show mistral

# Delete model (frees 4.1GB)
ollama rm mistral

# Test model speed
time ollama generate mistral "Write a hello world in Go"
```

---

## Support & Resources

- **Ollama**: https://ollama.ai
- **Continue.dev**: https://continue.dev
- **Mistral**: https://mistral.ai
- **Project context**: `.continue/project-context.md`
- **Code rules**: `.continue/rules.md`

---

## Comparison: Copilot vs Local Mistral

| Feature | Copilot | Mistral (Local) |
|---------|---------|-----------------|
| **Cost** | $10/month | Free |
| **Privacy** | Cloud (tracked) | 100% Local |
| **Latency** | 200-500ms (network) | 100-300ms (local) |
| **Code Quality** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Go Support** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Offline** | ❌ | ✅ |
| **Data Ownership** | Microsoft/OpenAI | You |
| **Customization** | Limited | Full |
| **Setup Time** | 2 min (account) | 60 min (download) |
| **Ongoing Cost** | $120/year | $0 |

---

## Troubleshooting Checklist

Before reporting issues, verify:

- [ ] macOS updated to latest
- [ ] Ollama installed: `which ollama`
- [ ] Model downloaded: `ollama list`
- [ ] Server running: `curl http://localhost:11434/api/tags`
- [ ] VS Code latest version
- [ ] Continue extension installed
- [ ] `.continue/config.json` exists and valid JSON
- [ ] MySQL running: `mysql -u gmoore -pmaggie2pie -e "SELECT 1;"`
- [ ] Go toolchain working: `go version`

If all above pass and still having issues, check:
```bash
# Check logs
log stream --predicate 'eventMessage contains "ollama"'

# Memory available
vm_stat | grep "Pages free"

# Network connectivity
ping ollama.ai
```

