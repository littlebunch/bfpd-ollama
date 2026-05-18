package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gmoore/bfpd-ollama/internal/chat"
	"github.com/gmoore/bfpd-ollama/internal/ollama"
	"github.com/gmoore/bfpd-ollama/internal/rag"
	"github.com/gmoore/bfpd-ollama/internal/vectordb"
	"github.com/gmoore/bfpd-ollama/pkg/config"
	"github.com/gmoore/bfpd-ollama/pkg/models"
	"github.com/spf13/cobra"
)

var (
	cfg          *config.Config
	ollamaClient *ollama.Client
	qdrantClient *vectordb.QdrantClient
	ragPipeline  *rag.Pipeline
	chatManager  *chat.Manager
	configPath   string
	useRAG       bool
)

var rootCmd = &cobra.Command{
	Use:   "food-chat",
	Short: "Food nutrition AI chat with RAG",
	Long:  "Interactive chat application for food nutrition queries powered by LLM and RAG",
	Run:   runChatCLI,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config.yaml", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&useRAG, "rag", true, "Use RAG for context")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func runChatCLI(cmd *cobra.Command, args []string) {
	// Load configuration
	var err error
	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize clients
	ollamaClient = ollama.NewClient(cfg.Ollama.URL, cfg.Ollama.Model, cfg.Ollama.EmbeddingModel)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := ollamaClient.Health(ctx); err != nil {
		cancel()
		log.Fatalf("Ollama service unavailable: %v", err)
	}
	cancel()

	qdrantClient, err = vectordb.NewQdrantClient(cfg.VectorDB.URL, cfg.VectorDB.CollectionName, cfg.VectorDB.VectorSize)
	if err != nil {
		log.Fatalf("Failed to initialize Qdrant: %v", err)
	}
	defer qdrantClient.Close()

	ragConfig := models.RAGConfig{
		TopK:           cfg.RAG.TopK,
		ScoreThreshold: cfg.RAG.ScoreThreshold,
	}
	ragPipeline = rag.NewPipeline(ollamaClient, qdrantClient, ragConfig)
	chatManager = chat.NewManager()

	// Create conversation
	conversationID := chatManager.CreateConversation()

	// Start interactive chat loop
	fmt.Println("🍽️  Food Nutrition AI Chat")
	fmt.Println("Type 'exit' to quit, 'clear' to start new conversation")
	fmt.Printf("Using RAG: %v\n\n", useRAG)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		if input == "clear" {
			conversationID = chatManager.CreateConversation()
			fmt.Println("New conversation started")
			continue
		}

		// Add user message
		chatManager.AddMessage(conversationID, "user", input)

		// Generate response
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

		if useRAG {
			resp, err := ragPipeline.Query(ctx, input)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				cancel()
				continue
			}

			fmt.Printf("\nAssistant: %s\n", resp.Content)

			if len(resp.Sources) > 0 {
				fmt.Println("\n📚 Sources:")
				for i, source := range resp.Sources {
					fmt.Printf("[%d] Score: %.2f\n", i+1, source.Score)
					if title, ok := source.Metadata["title"].(string); ok {
						fmt.Printf("    Title: %s\n", title)
					}
					preview := source.Content
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					fmt.Printf("    %s\n", preview)
				}
			}
		} else {
			response, err := ollamaClient.Generate(ctx, input, "You are a helpful food nutrition assistant.")
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				cancel()
				continue
			}

			fmt.Printf("\nAssistant: %s\n", response)
			chatManager.AddMessage(conversationID, "assistant", response)
		}

		cancel()
		fmt.Println()
	}
}
