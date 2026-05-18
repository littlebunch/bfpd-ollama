package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/gmoore/bfpd-ollama/internal/embedding"
	"github.com/gmoore/bfpd-ollama/internal/ollama"
	"github.com/gmoore/bfpd-ollama/internal/vectordb"
	"github.com/gmoore/bfpd-ollama/pkg/models"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	var (
		dbHost       = flag.String("db-host", "localhost", "MySQL host")
		dbUser       = flag.String("db-user", "gmoore", "MySQL user")
		dbPassword   = flag.String("db-password", "maggie2pie", "MySQL password")
		dbName       = flag.String("db-name", "gbfpd", "Database name")
		ollamaURL    = flag.String("ollama-url", "http://localhost:11434", "Ollama URL")
		qdrantURL    = flag.String("qdrant-url", "http://localhost:6333", "Qdrant URL")
		chunkSize    = flag.Int("chunk-size", 512, "Chunk size in tokens")
		chunkOverlap = flag.Int("chunk-overlap", 128, "Chunk overlap in tokens")
	)
	flag.Parse()

	// Connect to MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true",
		*dbUser, *dbPassword, *dbHost, *dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	log.Println("✓ Connected to MySQL")

	// Initialize Ollama client
	ollamaClient := ollama.NewClient(*ollamaURL, "neural-chat", "nomic-embed-text")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := ollamaClient.Health(ctx); err != nil {
		cancel()
		log.Fatalf("Ollama service unavailable: %v", err)
	}
	cancel()
	log.Println("✓ Connected to Ollama")

	// Initialize Qdrant client
	qdrantClient, err := vectordb.NewQdrantClient(*qdrantURL, "food_vectors", 768)
	if err != nil {
		log.Fatalf("Failed to initialize Qdrant: %v", err)
	}
	defer qdrantClient.Close()
	log.Println("✓ Connected to Qdrant")

	// Recreate collection
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	if err := qdrantClient.RecreateCollection(ctx); err != nil {
		cancel()
		log.Fatalf("Failed to create collection: %v", err)
	}
	cancel()
	log.Println("✓ Collection created")

	// Load and process food data
	log.Println("\nLoading food data from MySQL...")
	foodDocs, err := loadFoodDocuments(db, *chunkSize, *chunkOverlap)
	if err != nil {
		log.Fatalf("Failed to load food documents: %v", err)
	}
	log.Printf("✓ Loaded %d documents\n", len(foodDocs))

	// Embed and store documents
	log.Println("\nEmbedding documents...")
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)

	batchSize := 100
	for i := 0; i < len(foodDocs); i += batchSize {
		end := i + batchSize
		if end > len(foodDocs) {
			end = len(foodDocs)
		}

		batch := foodDocs[i:end]

		// Embed batch
		for j := range batch {
			embedding, err := ollamaClient.Embed(ctx, batch[j].Content)
			if err != nil {
				cancel()
				log.Fatalf("Failed to embed document: %v", err)
			}
			batch[j].Embedding = embedding
		}

		// Store in Qdrant
		if err := qdrantClient.UpsertPoints(ctx, batch); err != nil {
			cancel()
			log.Fatalf("Failed to upsert points: %v", err)
		}

		log.Printf("  ✓ Processed %d/%d documents", end, len(foodDocs))
	}
	cancel()

	log.Println("\n✓ Vector database seeding complete!")
}

func loadFoodDocuments(db *sql.DB, chunkSize, chunkOverlap int) ([]models.FoodDocument, error) {
	var docs []models.FoodDocument
	chunker := embedding.NewChunker(chunkSize, chunkOverlap)

	// Query branded foods with nutrients
	query := `
		SELECT 
			bf.fdc_id,
			bf.gtin_upc,
			bf.brand_name,
			bf.short_description,
			bf.ingredients,
			bf.serving_size,
			bf.serving_size_unit,
			bfc.category_name,
			GROUP_CONCAT(CONCAT(n.name, ': ', fn.amount, ' ', n.unit_name) SEPARATOR ', ') as nutrients
		FROM branded_foods bf
		LEFT JOIN branded_food_categories bfc ON bf.branded_food_category_id = bfc.id
		LEFT JOIN food_nutrients fn ON bf.fdc_id = fn.fdc_id
		LEFT JOIN nutrients n ON fn.nutrient_id = n.id
		GROUP BY bf.fdc_id
		LIMIT 1000
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var fdcID int64
		var gtinUPC, brandName, description, ingredients, servingUnit, category, nutrients sql.NullString
		var servingSize sql.NullFloat64

		if err := rows.Scan(&fdcID, &gtinUPC, &brandName, &description, &ingredients,
			&servingSize, &servingUnit, &category, &nutrients); err != nil {
			return nil, err
		}

		// Build content
		var content string
		if brandName.Valid {
			content += "Brand: " + brandName.String + "\n"
		}
		if description.Valid {
			content += "Description: " + description.String + "\n"
		}
		if ingredients.Valid {
			content += "Ingredients: " + ingredients.String + "\n"
		}
		if nutrients.Valid {
			content += "Nutrients: " + nutrients.String + "\n"
		}

		if content == "" {
			continue
		}

		// Chunk content
		chunks := chunker.Split(content)
		for _, chunk := range chunks {
			doc := models.FoodDocument{
				ID:      fmt.Sprintf("food_%d_%d", fdcID, len(docs)),
				FdcID:   fdcID,
				Content: chunk,
				Title:   brandName.String,
				Metadata: map[string]interface{}{
					"gtin_upc": gtinUPC.String,
					"category": category.String,
					"type":     "branded_food",
				},
			}
			docs = append(docs, doc)
		}
	}

	return docs, rows.Err()
}
