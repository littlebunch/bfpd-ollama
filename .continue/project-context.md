# BFPD Importer - Local LLM Context

## Project Overview
**CSV Importer for USDA FoodData Central**
- Language: Go 1.21+
- Main file: `import_gbfpd.go` (~700 lines)
- Database: MySQL 8.0 (localhost)
- Imports: ~34.4M rows across 11 CSV files
- Performance: ~360 seconds total import time

## Architecture

### Core Components
1. **Importer struct** - Manages DB connections, batch processing, statistics
2. **ImportConfig map** - Defines CSV file import configurations
3. **Batch processing pipeline** - Sequential row processing with FK validation
4. **Preprocessing layer** - Extracts unique values for lookup tables

### Key Patterns

#### 1. Configuration-Driven Import
```go
importConfigs = map[string]ImportConfig{
    "filename.csv": {
        Table:         "table_name",
        Columns:       []string{"col1", "col2"},
        BatchSize:     5000,
        Preprocessing: "populate_lookup_table",
        ColumnMap:     map[string]string{"dbCol": "csvCol"},
    }
}
```

#### 2. Column Mapping (ColumnMap Pattern)
```go
// When CSV column name differs from DB column name
ColumnMap: map[string]string{
    "branded_food_category_id": "branded_food_category",
    "fdc_id": "foodId",  // microbes.csv example
}
```

#### 3. Multi-Encoding CSV Detection
```go
// Tries UTF-8 → Latin-1 → Windows-1252
// Ensures compatibility with mixed-encoding CSVs
ReadCSV(filepath) // Returns []Row, []string headers
```

#### 4. Batch Processing with FK Validation
```go
// Sequential batching (NOT concurrent) to avoid FK deadlocks
// Respects MySQL's 65,535 placeholder limit
batchCapacity := batchSize * len(columns)
```

#### 5. Lookup Table Preprocessing
```go
// Extract unique values before main import
PopulateBrandedFoodCategories() // Returns map[string]int64 (value → FK ID)
PopulateTradeChannels()          // Returns map[string]int64 (channel → FK ID)
```

#### 6. Type Conversion (CleanValue pattern)
```go
// Converts CSV strings to typed values
CleanValue(value, colName) interface{}
// Handles: IDs (int64), floats, dates, strings
```

#### 7. Error Handling
```go
// All errors wrapped with context
if err != nil {
    imp.logf("⚠ Error [context]: %v", err)
    return fmt.Errorf("operation failed: %w", err)
}
```

## Database Schema

### Lookup Tables (Auto-populated)
- `branded_food_categories` - 447 unique categories
- `trade_channels` - 4 unique channels
- `nutrient_sources` - 10 sources
- `nutrient_derivations` - 65 derivations

### Main Tables
- `foods` - ~2M rows (fdc_id PRIMARY KEY)
- `branded_foods` - ~2M rows (fdc_id PRIMARY KEY, branded_food_category_id FK)
- `food_nutrients` - ~26M rows (id BIGINT PRIMARY KEY)
- `food_attributes` - ~2.4M rows
- `microbes` - 17 rows

### Views
- `vw_branded_food_nutrients` - Nutrient details per branded food
- `vw_branded_food_macros` - Macro nutrients (energy, protein, fat, carbs, fiber)

## Configuration Details

### Current Import Order
```
1. food_nutrient_source.csv → nutrient_sources (10 rows)
2. food_nutrient_derivation.csv → nutrient_derivations (65 rows)
3. measure_unit.csv → measure_units (123 rows)
4. nutrient.csv → nutrients (477 rows)
5. food_attribute_type.csv → food_attribute_types (5 rows)
6. food.csv → foods (2M rows, with trade_channel FK lookup)
7. branded_food.csv → branded_foods (2M rows, with category FK lookup)
8. food_nutrient.csv → food_nutrients (26M rows)
9. food_attribute.csv → food_attributes (2.4M rows)
10. microbe.csv → microbes (17 rows)
11. food_update_log_entry.csv → food_update_log_entries (2M rows)
```

### Batch Sizes (Optimized)
- Lookup tables: 5000
- Main tables (foods, branded_foods): 2500-5000
- Nutrient/attribute tables: 2000-10000
- Constraint: 65,535 MySQL placeholders max

## Common Tasks

### Add New CSV File Import
1. Create new entry in `importConfigs` map
2. Add to `importOrder` slice in correct dependency position
3. If preprocessing needed, add `Preprocessing: "populate_something"`
4. If column name mismatch, add `ColumnMap` entry
5. Test with `go build && ./import_gbfpd`

### Add Preprocessing Function
Pattern to follow:
```go
func (imp *Importer) PopulateNewLookup(filename string) (map[string]int64, error) {
    imp.logf("\n  Preprocessing: Extracting unique values from %s...", filename)
    rows, _, err := imp.ReadCSV(filepath)
    
    // Extract unique values
    uniqueMap := make(map[string]bool)
    for _, row := range rows {
        if val, ok := row["csv_column_name"]; ok && val != "" {
            uniqueMap[strings.TrimSpace(val)] = true
        }
    }
    
    // Insert into DB and build FK map
    resultMap := make(map[string]int64)
    for val := range uniqueMap {
        _, err := imp.db.Exec("INSERT INTO table_name (...) VALUES (?)", val)
        var id int64
        imp.db.QueryRow("SELECT id FROM table_name WHERE col_name = ?", val).Scan(&id)
        resultMap[val] = id
    }
    
    imp.logf("    ✓ Populated %d entries", len(resultMap))
    return resultMap, nil
}
```

### Fix Column Name Mapping
Add to ColumnMap in ImportConfig:
```go
ColumnMap: map[string]string{
    "database_column": "csv_column_name",
}
```

### Performance Tuning
- Reduce `BatchSize` if hitting MySQL placeholder limit
- Increase `BatchSize` for small tables (< 100K rows)
- Monitor memory with `watch -n 1 'ps aux | grep import_gbfpd'`

## Recent Fixes Applied
1. **nutrient_derivations.description**: Changed VARCHAR(255) → TEXT for 263-char max
2. **Column mapping**: Fixed case sensitivity (microbes: "fdc_id" → "foodId")
3. **Filename mismatch**: "nutrient_source.csv" → "food_nutrient_source.csv"
4. **Branded food FK**: Added ColumnMap for branded_food_category_id → branded_food_category

## Testing Checklist
- [ ] Ollama running: `curl http://localhost:11434/api/tags`
- [ ] All 11 CSVs found in data directory
- [ ] MySQL running with correct credentials
- [ ] Build succeeds: `go build -o import_gbfpd import_gbfpd.go`
- [ ] Import completes: `./import_gbfpd 2>&1 | tail -20`
- [ ] All rows inserted (zero skipped)
- [ ] Views query correctly: `SELECT COUNT(*) FROM vw_branded_food_nutrients`
