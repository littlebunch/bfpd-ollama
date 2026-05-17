# Quick Reference - Local LLM Setup

## 🚀 Quick Start (5 Commands)

```bash
# Terminal 1: Start Ollama server (keep running)
brew install ollama
ollama pull mistral
ollama serve

# Terminal 2: Build and test
cd /Users/gmoore/repos/bfpd-ollama
go build -o import_gbfpd import_gbfpd.go
./import_gbfpd 2>&1 | tail -20
```

## 📋 Verify Everything Works

```bash
# Check Ollama is running
curl http://localhost:11434/api/tags

# List downloaded models
ollama list

# Test model response time
time ollama generate mistral "Hello"
```

## 🎯 VS Code Integration

1. **Install Continue extension** (Cmd+Shift+X)
   - Search: "Continue"
   - Click Install

2. **Open import_gbfpd.go** and start typing
   - Autocomplete suggestions should appear
   - Wait 1-3 seconds first time

3. **Try slash commands**
   - Select code → Click Continue icon → Type `/comment`
   - Commands: `/doc`, `/explain`, `/edit`, `/test`

## 📊 Project Files

| File | Purpose |
|------|---------|
| `.continue/config.json` | Main configuration (Ollama + Mistral settings) |
| `.continue/project-context.md` | Project patterns and architecture |
| `.continue/rules.md` | Code generation guidelines |
| `Makefile` | Easy startup commands |
| `LOCAL_LLM_SETUP.md` | Detailed setup guide |

## 🔧 Useful Make Commands

```bash
make help                # Show all commands
make ollama-start        # Start Ollama server
make ollama-stop         # Stop Ollama server
make model-pull          # Download Mistral model
make build               # Build Go binary
make run                 # Full import
make test-ollama         # Test Ollama connection
make stats               # Show database row counts
```

## ⚡ Performance Tips

### Fast Completions
- Reduce context: Edit `.continue/config.json`, set `contextLength: 2048`
- Reduce tokens: Set `maxTokens: 512`

### Lower RAM Usage
```bash
OLLAMA_NUM_PARALLEL=1 ollama serve
```

### More RAM/Better Quality
```json
{
  "contextLength": 8192,
  "maxTokens": 2048,
  "temperature": 0.7
}
```

## 🆘 Troubleshooting

| Problem | Solution |
|---------|----------|
| "Connection refused" | Run `ollama serve` in another terminal |
| Slow completions | First load takes 1-3s; reduce context size |
| High RAM | Use `OLLAMA_NUM_PARALLEL=1` or smaller model |
| No autocomplete | Restart Continue: Cmd+Shift+P → "Continue: Restart" |
| Model not found | Run `ollama pull mistral` |

## 📌 Key Shortcuts

| Action | Shortcut |
|--------|----------|
| Restart Continue | Cmd+Shift+P → "Continue: Restart" |
| Open Continue chat | Cmd+Shift+C |
| Send message | Ctrl+Enter |
| Trigger autocomplete | Type (automatic) |
| Select code for slash command | Select → Click Continue icon |

## 💾 Config Examples

### Speed-Optimized (Latency < 200ms)
```json
{
  "contextLength": 2048,
  "maxTokens": 256,
  "temperature": 0.3
}
```

### Quality-Optimized (Better suggestions)
```json
{
  "contextLength": 8192,
  "maxTokens": 2048,
  "temperature": 0.7
}
```

### Balanced (Default)
```json
{
  "contextLength": 4096,
  "maxTokens": 1024,
  "temperature": 0.5
}
```

## 🔗 Resources

- **Project Context**: `.continue/project-context.md`
- **Code Rules**: `.continue/rules.md`
- **Setup Guide**: `LOCAL_LLM_SETUP.md`
- **Ollama Docs**: https://ollama.ai
- **Continue.dev**: https://continue.dev

## ✅ Verification Checklist

- [ ] Ollama installed: `which ollama`
- [ ] Model downloaded: `ollama list | grep mistral`
- [ ] Server running: `curl http://localhost:11434/api/tags`
- [ ] Continue extension installed
- [ ] Continue restarted
- [ ] Tab autocomplete working
- [ ] Slash commands working
- [ ] `.continue/config.json` present

## 📈 Expected Performance

| Operation | Expected Time |
|-----------|---|
| Tab autocomplete (first) | 1-3 seconds |
| Tab autocomplete (subsequent) | 100-300ms |
| Comment generation | 2-5 seconds |
| Refactor suggestion | 3-8 seconds |
| Full import (34.4M rows) | 360 seconds (~6 min) |

## 🎓 Next Steps

1. **Complete setup**: Run through commands in "Quick Start"
2. **Test functionality**: Verify "Verification Checklist" all ✅
3. **Configure for your needs**: Edit `.continue/config.json`
4. **Start coding**: Open `import_gbfpd.go` and let Mistral assist

---

**Questions?** Refer to:
- **Detailed setup**: `LOCAL_LLM_SETUP.md`
- **Code patterns**: `.continue/project-context.md`
- **Code rules**: `.continue/rules.md`
