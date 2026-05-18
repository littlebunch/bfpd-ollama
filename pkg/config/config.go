package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Ollama   OllamaConfig
	VectorDB VectorDBConfig
	RAG      RAGConfig
}

// ServerConfig holds API server configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig holds MySQL configuration
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	MaxConns int    `mapstructure:"maxconns"`
}

// OllamaConfig holds Ollama service configuration
type OllamaConfig struct {
	URL            string `mapstructure:"url"`
	Model          string `mapstructure:"model"`
	EmbeddingModel string `mapstructure:"embedding_model"`
	Timeout        int    `mapstructure:"timeout"`
	ContextWindow  int    `mapstructure:"context_window"`
}

// VectorDBConfig holds Qdrant configuration
type VectorDBConfig struct {
	URL            string `mapstructure:"url"`
	CollectionName string `mapstructure:"collection_name"`
	VectorSize     int    `mapstructure:"vector_size"`
}

// RAGConfig holds RAG pipeline configuration
type RAGConfig struct {
	TopK           int     `mapstructure:"topk"`
	ScoreThreshold float32 `mapstructure:"score_threshold"`
	ChunkSize      int     `mapstructure:"chunk_size"`
	ChunkOverlap   int     `mapstructure:"chunk_overlap"`
}

// LoadConfig loads configuration from environment and config files
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.database", "gbfpd")
	viper.SetDefault("database.maxconns", 32)

	viper.SetDefault("ollama.url", "http://localhost:11434")
	viper.SetDefault("ollama.model", "neural-chat")
	viper.SetDefault("ollama.embedding_model", "nomic-embed-text")
	viper.SetDefault("ollama.timeout", 120)
	viper.SetDefault("ollama.context_window", 4096)

	viper.SetDefault("vectordb.url", "http://localhost:6333")
	viper.SetDefault("vectordb.collection_name", "food_vectors")
	viper.SetDefault("vectordb.vector_size", 768)

	viper.SetDefault("rag.topk", 5)
	viper.SetDefault("rag.score_threshold", 0.5)
	viper.SetDefault("rag.chunk_size", 512)
	viper.SetDefault("rag.chunk_overlap", 128)

	// Allow environment variables to override
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
		// Config file not found; using defaults and env vars
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}
