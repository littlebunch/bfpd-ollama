package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/text/encoding/charmap"
)

// Config holds database and import configuration
type Config struct {
	Host     string
	User     string
	Password string
	Database string
	DataDir  string
}

// ImportConfig defines how to import a specific CSV file
type ImportConfig struct {
	Table         string
	Columns       []string
	Preprocessing string
	ColumnMap     map[string]string
	BatchSize     int
}

// ImportJob represents a file to be imported
type ImportJob struct {
	Filename string
	Config   ImportConfig
}

// Row represents a single CSV row
type Row map[string]string

// ImportStats tracks import statistics
type ImportStats struct {
	Table    string
	Inserted int64
	Skipped  int64
	Errors   []string
	mu       sync.Mutex
}

// Importer manages the database import process
type Importer struct {
	config      Config
	db          *sql.DB
	stats       map[string]*ImportStats
	lookupMaps  map[string]map[string]int64
	lookupMutex sync.RWMutex
	maxWorkers  int
	logChan     chan string
	batchSize   int
}

// Define import order and configs
var importConfigs = map[string]ImportConfig{
	"food_nutrient_source.csv": {
		Table:     "nutrient_sources",
		Columns:   []string{"id", "code", "description"},
		BatchSize: 5000,
	},
	"food_nutrient_derivation.csv": {
		Table:     "nutrient_derivations",
		Columns:   []string{"id", "code", "description", "source_id"},
		BatchSize: 5000,
	},
	"measure_unit.csv": {
		Table:     "measure_units",
		Columns:   []string{"id", "name"},
		BatchSize: 5000,
	},
	"nutrient.csv": {
		Table:     "nutrients",
		Columns:   []string{"id", "name", "unit_name", "nutrient_nbr", "rank"},
		BatchSize: 5000,
	},
	"food_attribute_type.csv": {
		Table:     "food_attribute_types",
		Columns:   []string{"id", "name", "description"},
		BatchSize: 5000,
	},
	"food.csv": {
		Table:         "foods",
		Columns:       []string{"fdc_id", "data_type", "description", "food_category_id", "publication_date", "market_country", "trade_channel_id", "microbe_data"},
		Preprocessing: "populate_trade_channels",
		BatchSize:     5000,
	},
	"branded_food.csv": {
		Table:         "branded_foods",
		Columns:       []string{"fdc_id", "brand_owner", "brand_name", "subbrand_name", "gtin_upc", "ingredients", "not_a_significant_source_of", "serving_size", "serving_size_unit", "household_serving_fulltext", "branded_food_category_id", "data_source", "package_weight", "modified_date", "available_date", "market_country", "discontinued_date", "preparation_state_code", "trade_channel", "short_description", "material_code"},
		Preprocessing: "populate_branded_food_categories",
		ColumnMap:     map[string]string{"branded_food_category_id": "branded_food_category"},
		BatchSize:     2500,
	},
	"food_nutrient.csv": {
		Table:     "food_nutrients",
		Columns:   []string{"id", "fdc_id", "nutrient_id", "amount", "data_points", "derivation_id", "`min`", "`max`", "`median`", "footnote", "min_year_acquired"},
		BatchSize: 2000,
	},
	"food_attribute.csv": {
		Table:     "food_attributes",
		Columns:   []string{"id", "fdc_id", "seq_num", "food_attribute_type_id", "name", "value"},
		BatchSize: 10000,
	},
	"microbe.csv": {
		Table:     "microbes",
		Columns:   []string{"id", "fdc_id", "method", "microbe_code", "min_value", "max_value", "uom"},
		ColumnMap: map[string]string{"fdc_id": "foodId"},
		BatchSize: 10000,
	},
	"food_update_log_entry.csv": {
		Table:     "food_update_log_entries",
		Columns:   []string{"id", "description", "last_updated"},
		BatchSize: 5000,
	},
}

var importOrder = []string{
	"food_nutrient_source.csv",
	"food_nutrient_derivation.csv",
	"measure_unit.csv",
	"nutrient.csv",
	"food_attribute_type.csv",
	"food.csv",
	"branded_food.csv",
	"food_nutrient.csv",
	"food_attribute.csv",
	"microbe.csv",
	"food_update_log_entry.csv",
}

func NewImporter(config Config) *Importer {
	return &Importer{
		config:     config,
		stats:      make(map[string]*ImportStats),
		lookupMaps: make(map[string]map[string]int64),
		maxWorkers: 4, // Concurrent DB connections
		logChan:    make(chan string, 100),
		batchSize:  10000,
	}
}

// Connect to MySQL database
func (imp *Importer) Connect() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&collation=utf8mb4_unicode_ci",
		imp.config.User, imp.config.Password, imp.config.Host, imp.config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	imp.db = db
	imp.logf("✓ Connected to MySQL: %s", imp.config.Database)
	return nil
}

// CreateDatabaseAndSchema creates fresh database schema
func (imp *Importer) CreateDatabaseAndSchema() error {
	// Connect to default mysql database first
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/mysql?parseTime=true",
		imp.config.User, imp.config.Password, imp.config.Host)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to mysql database: %w", err)
	}
	defer db.Close()

	// Drop existing database
	imp.logf("\nDropping existing database if exists...")
	dropSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", imp.config.Database)
	if _, err := db.Exec(dropSQL); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}
	imp.logf("✓ Dropped %s", imp.config.Database)

	// Create database
	imp.logf("Creating database %s...", imp.config.Database)
	createSQL := fmt.Sprintf("CREATE DATABASE %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", imp.config.Database)
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	imp.logf("✓ Created %s", imp.config.Database)

	// Connect to new database and create schema
	if err := imp.Connect(); err != nil {
		return err
	}

	imp.logf("\nCreating tables...")
	if err := imp.executeSchema(); err != nil {
		return err
	}

	imp.logf("✓ Schema created successfully")
	return nil
}

// executeSchema executes all DDL statements
func (imp *Importer) executeSchema() error {
	statements := strings.Split(schemaDDL, ";")

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := imp.db.Exec(stmt); err != nil {
			imp.logf("⚠ Warning: %s", strings.Split(err.Error(), ":")[0][:80])
		}
	}

	return nil
}

// ReadCSV reads a CSV file with encoding detection
func (imp *Importer) ReadCSV(filepath string) ([]Row, []string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, nil, err
	}

	// Try different encodings
	encodings := []func([]byte) (string, error){
		func(b []byte) (string, error) { return string(b), nil }, // UTF-8
		func(b []byte) (string, error) { // Latin-1
			decoder := charmap.ISO8859_1.NewDecoder()
			result, err := decoder.String(string(b))
			return result, err
		},
		func(b []byte) (string, error) { // Windows-1252
			decoder := charmap.Windows1252.NewDecoder()
			result, err := decoder.String(string(b))
			return result, err
		},
	}

	var content string
	for _, enc := range encodings {
		if str, err := enc(data); err == nil && utf8.ValidString(str) {
			content = str
			break
		}
	}

	if content == "" {
		return nil, nil, fmt.Errorf("could not decode file with any encoding")
	}

	reader := csv.NewReader(strings.NewReader(content))

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}

	// Normalize headers (remove quotes)
	for i := range headers {
		headers[i] = strings.Trim(headers[i], `"`)
	}

	// Read all rows
	var rows []Row
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		row := make(Row)
		for i, header := range headers {
			if i < len(record) {
				row[strings.ToLower(header)] = record[i]
			}
		}
		rows = append(rows, row)
	}

	return rows, headers, nil
}

// PopulateBrandedFoodCategories extracts and populates lookup table
func (imp *Importer) PopulateBrandedFoodCategories(filename string) (map[string]int64, error) {
	filepath := filepath.Join(imp.config.DataDir, filename)

	imp.logf("\n  Preprocessing: Extracting unique categories from %s...", filename)

	rows, _, err := imp.ReadCSV(filepath)
	if err != nil {
		return nil, err
	}

	// Extract unique categories
	categories := make(map[string]bool)
	for _, row := range rows {
		if cat, ok := row["branded_food_category"]; ok && cat != "" {
			categories[strings.TrimSpace(cat)] = true
		}
	}

	imp.logf("    Found %d unique categories", len(categories))

	// Insert into lookup table
	categoryMap := make(map[string]int64)
	for cat := range categories {
		// Insert the category
		_, err := imp.db.Exec(
			"INSERT INTO branded_food_categories (category_name) VALUES (?) "+
				"ON DUPLICATE KEY UPDATE category_name=VALUES(category_name)",
			cat,
		)

		if err != nil {
			imp.logf("    ⚠ Error inserting category: %v", err)
			continue
		}

		// Get the ID
		var id int64
		err = imp.db.QueryRow("SELECT id FROM branded_food_categories WHERE category_name = ?", cat).Scan(&id)
		if err != nil {
			imp.logf("    ⚠ Error fetching category ID: %v", err)
			continue
		}

		categoryMap[cat] = id
	}

	imp.logf("    ✓ Populated %d categories", len(categoryMap))
	imp.lookupMutex.Lock()
	imp.lookupMaps["branded_food_categories"] = categoryMap
	imp.lookupMutex.Unlock()

	return categoryMap, nil
}

// PopulateTradeChannels extracts and populates lookup table
func (imp *Importer) PopulateTradeChannels(filename string) (map[string]int64, error) {
	filepath := filepath.Join(imp.config.DataDir, filename)

	imp.logf("\n  Preprocessing: Extracting unique trade channels from %s...", filename)

	rows, _, err := imp.ReadCSV(filepath)
	if err != nil {
		return nil, err
	}

	// Extract unique channels
	channels := make(map[string]bool)
	for _, row := range rows {
		if ch, ok := row["trade_channel"]; ok && ch != "" {
			channels[strings.TrimSpace(ch)] = true
		}
	}

	imp.logf("    Found %d unique trade channels", len(channels))

	// Insert into lookup table
	channelMap := make(map[string]int64)
	for ch := range channels {
		// Insert the channel
		_, err := imp.db.Exec(
			"INSERT INTO trade_channels (channel_name) VALUES (?) "+
				"ON DUPLICATE KEY UPDATE channel_name=VALUES(channel_name)",
			ch,
		)

		if err != nil {
			imp.logf("    ⚠ Error inserting channel: %v", err)
			continue
		}

		// Get the ID
		var id int64
		err = imp.db.QueryRow("SELECT id FROM trade_channels WHERE channel_name = ?", ch).Scan(&id)
		if err != nil {
			imp.logf("    ⚠ Error fetching channel ID: %v", err)
			continue
		}

		channelMap[ch] = id
	}

	imp.logf("    ✓ Populated %d trade channels", len(channelMap))
	imp.lookupMutex.Lock()
	imp.lookupMaps["trade_channels"] = channelMap
	imp.lookupMutex.Unlock()

	return channelMap, nil
}

// CleanValue converts CSV values to appropriate types
func (imp *Importer) CleanValue(value, colName string) interface{} {
	if value == "" || value == "NULL" {
		return nil
	}

	colLower := strings.ToLower(colName)

	// Handle numeric IDs
	if strings.Contains(colLower, "id") && colName != "microbe_code" {
		if v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
			return v
		}
		return nil
	}

	// Handle decimals/floats
	floatFields := []string{"amount", "min", "max", "median", "weight", "rank", "size"}
	for _, f := range floatFields {
		if strings.Contains(colLower, f) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				return v
			}
			return nil
		}
	}

	// Handle dates - just trim whitespace
	if strings.Contains(colLower, "date") {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
		return nil
	}

	// String fields
	return strings.TrimSpace(value)
}

// ImportCSV imports a single CSV file using batch processing
func (imp *Importer) ImportCSV(filename string, config ImportConfig) error {
	filepath := filepath.Join(imp.config.DataDir, filename)

	_, err := os.Stat(filepath)
	if err != nil {
		imp.logf("⚠ File not found: %s", filepath)
		return nil
	}

	imp.logf("\nImporting %s → %s...", filename, config.Table)

	// Run preprocessing if specified
	var lookupMap map[string]int64
	if config.Preprocessing != "" {
		if config.Preprocessing == "populate_branded_food_categories" {
			lookupMap, _ = imp.PopulateBrandedFoodCategories(filename)
		} else if config.Preprocessing == "populate_trade_channels" {
			lookupMap, _ = imp.PopulateTradeChannels(filename)
		}
	}

	rows, headers, err := imp.ReadCSV(filepath)
	if err != nil {
		imp.logf("  ✗ Error reading CSV: %v", err)
		return err
	}

	if len(rows) == 0 {
		imp.logf("  ⚠ No data rows found")
		return nil
	}

	// Build column header map
	csvCols := make(map[string]string)
	for _, header := range headers {
		csvCols[strings.ToLower(header)] = header
	}

	// Disable FK checks
	imp.db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer imp.db.Exec("SET FOREIGN_KEY_CHECKS=1")

	// Build column list for SQL
	colList := strings.Join(config.Columns, ", ")

	// Process rows in batches sequentially for reliability
	stats := &ImportStats{Table: config.Table}
	imp.stats[config.Table] = stats

	// Use a default batch size if not set
	actualBatchSize := config.BatchSize
	if actualBatchSize <= 0 {
		actualBatchSize = 10000
	}

	batchCapacity := actualBatchSize * len(config.Columns)
	batch := make([]interface{}, 0, batchCapacity)

	// Disable foreign key checks during import
	imp.db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer imp.db.Exec("SET FOREIGN_KEY_CHECKS=1")

	for rowIdx, row := range rows {
		for _, col := range config.Columns {
			colLower := strings.ToLower(strings.Trim(col, "`"))

			// Check column mapping
			csvColName := colLower
			if mapping, ok := config.ColumnMap[colLower]; ok {
				csvColName = strings.ToLower(mapping)
			}

			// Find value
			var value interface{}
			if _, ok := csvCols[csvColName]; ok {
				if rowVal, ok := row[csvColName]; ok {
					// Apply lookup mapping for FK columns
					if (colLower == "branded_food_category_id" || colLower == "trade_channel_id") && len(lookupMap) > 0 {
						if mapped, ok := lookupMap[strings.TrimSpace(rowVal)]; ok {
							value = mapped
						}
					} else {
						value = imp.CleanValue(rowVal, col)
					}
				}
			}

			batch = append(batch, value)
		}

		// Execute batch when it reaches capacity
		if len(batch) >= batchCapacity {
			if err := imp.executeBatch(config, colList, batch); err != nil {
				stats.Skipped += int64(len(batch) / len(config.Columns))
				errStr := fmt.Sprintf("Row %d: %v", rowIdx, err)
				if len(errStr) > 150 {
					stats.Errors = append(stats.Errors, errStr[:150])
				} else {
					stats.Errors = append(stats.Errors, errStr)
				}
			} else {
				stats.Inserted += int64(len(batch) / len(config.Columns))
			}
			batch = make([]interface{}, 0, batchCapacity)
		}

		if rowIdx%100000 == 0 && rowIdx > 0 {
			imp.logf("  ✓ Progress: %d rows processed...", rowIdx)
		}
	}

	// Execute final batch if any
	if len(batch) > 0 {
		completeRowCount := len(batch) / len(config.Columns)
		if completeRowCount > 0 {
			completeBatch := batch[:completeRowCount*len(config.Columns)]
			if err := imp.executeBatch(config, colList, completeBatch); err != nil {
				stats.Skipped += int64(len(completeBatch) / len(config.Columns))
				errStr := fmt.Sprintf("Final batch: %v", err)
				if len(errStr) > 150 {
					stats.Errors = append(stats.Errors, errStr[:150])
				} else {
					stats.Errors = append(stats.Errors, errStr)
				}
			} else {
				stats.Inserted += int64(len(completeBatch) / len(config.Columns))
			}
		}
	}

	imp.logf("  ✓ Inserted: %d rows", stats.Inserted)
	if stats.Skipped > 0 {
		imp.logf("  ⚠ Skipped: %d rows", stats.Skipped)
	}
	// For large skip counts, show more errors
	if len(stats.Errors) > 0 {
		if config.Table == "foods" || config.Table == "branded_foods" {
			// Show all errors for these critical tables
			maxErrors := len(stats.Errors)
			if maxErrors > 20 {
				maxErrors = 20
			}
			for i := 0; i < maxErrors; i++ {
				imp.logf("    %s", stats.Errors[i])
			}
			if len(stats.Errors) > 20 {
				imp.logf("    ... and %d more errors", len(stats.Errors)-20)
			}
		} else if len(stats.Errors) <= 5 {
			for _, err := range stats.Errors {
				imp.logf("    %s", err)
			}
		}
	}

	return nil
}

func (imp *Importer) buildUpdateClause(columns []string) string {
	if len(columns) <= 1 {
		return ""
	}
	updates := make([]string, 0, len(columns))
	for i := 1; i < len(columns); i++ {
		col := columns[i]
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", col, col))
	}
	return strings.Join(updates, ", ")
}

// executeBatch executes a batch insert
func (imp *Importer) executeBatch(config ImportConfig, colList string, batchValues []interface{}) error {
	if len(batchValues) == 0 || len(config.Columns) == 0 {
		return nil
	}

	numCols := len(config.Columns)
	rowCount := len(batchValues) / numCols

	if rowCount == 0 {
		return nil
	}

	// Build placeholders
	placeholderRows := make([]string, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		placeholders := make([]string, numCols)
		for j := 0; j < numCols; j++ {
			placeholders[j] = "?"
		}
		placeholderRows = append(placeholderRows, "("+strings.Join(placeholders, ", ")+")")
	}

	updateClause := imp.buildUpdateClause(config.Columns)

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s "+
		"ON DUPLICATE KEY UPDATE %s",
		config.Table, colList, strings.Join(placeholderRows, ", "),
		updateClause)

	_, err := imp.db.Exec(sql, batchValues...)
	return err
}

// RunImports orchestrates the entire import process
func (imp *Importer) RunImports() error {
	startTime := time.Now()

	if err := imp.CreateDatabaseAndSchema(); err != nil {
		return err
	}

	// Import with dependency ordering
	// First pass: prerequisites (sources, derivations, measures, nutrients, attribute types)
	// Second pass: main tables (foods, branded_foods)
	// Third pass: detail tables (nutrients, attributes, microbes)

	for _, filename := range importOrder {
		if config, ok := importConfigs[filename]; ok {
			if err := imp.ImportCSV(filename, config); err != nil {
				imp.logf("✗ Error importing %s: %v", filename, err)
			}
		}
	}

	imp.PrintSummary()
	imp.logf("\nTotal time: %.2fs", time.Since(startTime).Seconds())

	return nil
}

// PrintSummary prints import statistics
func (imp *Importer) PrintSummary() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("IMPORT SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	var totalInserted, totalSkipped int64

	for table, stats := range imp.stats {
		fmt.Printf("%-30s Inserted: %10d  Skipped: %10d\n", table, stats.Inserted, stats.Skipped)
		totalInserted += stats.Inserted
		totalSkipped += stats.Skipped
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-30s Inserted: %10d  Skipped: %10d\n", "TOTAL", totalInserted, totalSkipped)
	fmt.Println(strings.Repeat("=", 60))
}

func (imp *Importer) logf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (imp *Importer) Close() {
	if imp.db != nil {
		imp.db.Close()
	}
}

// Database schema DDL
const schemaDDL = `
CREATE TABLE IF NOT EXISTS nutrient_sources (
    id INT PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nutrient_derivations (
    id INT PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    description TEXT NOT NULL,
    source_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_id) REFERENCES nutrient_sources(id)
);

CREATE TABLE IF NOT EXISTS measure_units (
    id INT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nutrients (
    id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    unit_name VARCHAR(50),
    nutrient_nbr VARCHAR(50),
    rank DECIMAL(20,6),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    KEY idx_nutrient_nbr (nutrient_nbr),
    KEY idx_rank (rank)
);

CREATE TABLE IF NOT EXISTS food_attribute_types (
    id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS branded_food_categories (
    id INT AUTO_INCREMENT PRIMARY KEY,
    category_name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_category (category_name(500))
);

CREATE TABLE IF NOT EXISTS trade_channels (
    id INT AUTO_INCREMENT PRIMARY KEY,
    channel_name VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS foods (
    fdc_id INT PRIMARY KEY,
    data_type VARCHAR(50) NOT NULL,
    description VARCHAR(2000),
    food_category_id INT,
    publication_date DATE,
    market_country VARCHAR(255),
    trade_channel_id INT,
    microbe_data LONGTEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (trade_channel_id) REFERENCES trade_channels(id),
    KEY idx_data_type (data_type),
    KEY idx_publication_date (publication_date)
);

CREATE TABLE IF NOT EXISTS branded_foods (
    fdc_id INT PRIMARY KEY,
    brand_owner VARCHAR(500),
    brand_name VARCHAR(500),
    subbrand_name VARCHAR(500),
    gtin_upc VARCHAR(50),
    ingredients LONGTEXT,
    not_a_significant_source_of TEXT,
    serving_size DECIMAL(20,6),
    serving_size_unit VARCHAR(100),
    household_serving_fulltext VARCHAR(500),
    branded_food_category_id INT,
    data_source VARCHAR(100),
    package_weight DECIMAL(20,6),
    modified_date DATE,
    available_date DATE,
    market_country VARCHAR(255),
    discontinued_date DATE,
    preparation_state_code VARCHAR(100),
    trade_channel VARCHAR(500),
    short_description TEXT,
    material_code VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (fdc_id) REFERENCES foods(fdc_id) ON DELETE CASCADE,
    FOREIGN KEY (branded_food_category_id) REFERENCES branded_food_categories(id),
    KEY idx_brand_owner (brand_owner),
    KEY idx_brand_name (brand_name),
    KEY idx_gtin_upc (gtin_upc),
    KEY idx_modified_date (modified_date),
    FULLTEXT idx_ingredients (ingredients)
);

CREATE TABLE IF NOT EXISTS food_nutrients (
    id BIGINT PRIMARY KEY,
    fdc_id INT NOT NULL,
    nutrient_id INT NOT NULL,
    amount DECIMAL(30,10),
    data_points INT,
    derivation_id INT,
    min DECIMAL(30,10),
    max DECIMAL(30,10),
    median DECIMAL(30,10),
    footnote TEXT,
    min_year_acquired INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (fdc_id) REFERENCES foods(fdc_id) ON DELETE CASCADE,
    FOREIGN KEY (nutrient_id) REFERENCES nutrients(id),
    FOREIGN KEY (derivation_id) REFERENCES nutrient_derivations(id),
    KEY idx_fdc_id (fdc_id),
    KEY idx_nutrient_id (nutrient_id),
    KEY idx_composite (fdc_id, nutrient_id)
);

CREATE TABLE IF NOT EXISTS food_attributes (
    id INT PRIMARY KEY,
    fdc_id INT NOT NULL,
    seq_num INT,
    food_attribute_type_id INT,
    name VARCHAR(255),
    value VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (fdc_id) REFERENCES foods(fdc_id) ON DELETE CASCADE,
    FOREIGN KEY (food_attribute_type_id) REFERENCES food_attribute_types(id),
    KEY idx_fdc_id (fdc_id)
);

CREATE TABLE IF NOT EXISTS microbes (
    id INT PRIMARY KEY,
    fdc_id INT NOT NULL,
    method VARCHAR(100),
    microbe_code VARCHAR(100),
    min_value DECIMAL(30,10),
    max_value DECIMAL(30,10),
    uom VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (fdc_id) REFERENCES foods(fdc_id) ON DELETE CASCADE,
    KEY idx_fdc_id (fdc_id)
);

CREATE TABLE IF NOT EXISTS food_update_log_entries (
    id INT PRIMARY KEY,
    description TEXT,
    last_updated DATE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE OR REPLACE VIEW vw_branded_food_nutrients AS
SELECT 
    bf.fdc_id,
    bf.gtin_upc,
    bf.brand_owner,
    bf.brand_name,
    bf.short_description,
    bf.ingredients,
    bf.serving_size,
    bf.serving_size_unit,
    bfc.category_name as branded_food_category,
    n.name as nutrient_name,
    n.unit_name as nutrient_unit,
    fn.amount as nutrient_amount,
    fn.min as min_value,
    fn.max as max_value,
    fn.median as median_value,
    fn.data_points
FROM branded_foods bf
LEFT JOIN branded_food_categories bfc ON bf.branded_food_category_id = bfc.id
LEFT JOIN food_nutrients fn ON bf.fdc_id = fn.fdc_id
LEFT JOIN nutrients n ON fn.nutrient_id = n.id
ORDER BY bf.fdc_id, n.rank DESC;

CREATE OR REPLACE VIEW vw_branded_food_macros AS
SELECT 
    bf.fdc_id,
    bf.gtin_upc,
    bf.brand_name,
    bf.short_description,
    bfc.category_name,
    bf.serving_size,
    bf.serving_size_unit,
    MAX(CASE WHEN n.name LIKE '%Energy%' THEN fn.amount END) as energy_kcal,
    MAX(CASE WHEN n.name LIKE '%Protein%' THEN fn.amount END) as protein_g,
    MAX(CASE WHEN n.name LIKE '%lipid%fat%' THEN fn.amount END) as fat_g,
    MAX(CASE WHEN n.name LIKE '%Carbohydrate%' THEN fn.amount END) as carbs_g,
    MAX(CASE WHEN n.name LIKE '%Fiber%' THEN fn.amount END) as fiber_g
FROM branded_foods bf
LEFT JOIN branded_food_categories bfc ON bf.branded_food_category_id = bfc.id
LEFT JOIN food_nutrients fn ON bf.fdc_id = fn.fdc_id
LEFT JOIN nutrients n ON fn.nutrient_id = n.id
GROUP BY bf.fdc_id, bf.gtin_upc, bf.brand_name, bf.short_description, 
         bfc.category_name, bf.serving_size, bf.serving_size_unit;
`

func main() {
	var (
		host     = flag.String("host", "localhost", "MySQL host")
		user     = flag.String("user", "gmoore", "MySQL user")
		password = flag.String("password", "maggie2pie", "MySQL password")
		database = flag.String("database", "gbfpd", "Database name")
		dataDir  = flag.String("data-dir", "/Users/gmoore/bfpd/2025-12/FoodData_Central_branded_food_csv_2025-12-18", "Data directory")
	)

	flag.Parse()

	config := Config{
		Host:     *host,
		User:     *user,
		Password: *password,
		Database: *database,
		DataDir:  *dataDir,
	}

	importer := NewImporter(config)
	defer importer.Close()

	if err := importer.RunImports(); err != nil {
		log.Fatalf("Import failed: %v", err)
	}
}
