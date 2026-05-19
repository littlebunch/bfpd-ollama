# Code Generation Rules for BFPD Importer

## DO's ✓

### Code Style
- Use error wrapping with `fmt.Errorf(..., %w, err)` for all error handling
- Add context to error messages: `"failed to [action]: %w"`
- Use the `Importer` receiver pattern for all methods
- Follow existing logging style: `imp.logf("⚠ Message: %v", err)`
- Add defensive programming (check length before string operations)

### Go Idioms
- Use comma-ok pattern for map/channel operations: `if val, ok := m[key]; ok`
- Prefer `strings.TrimSpace()` for user input
- Use `strings.ToLower()` for case-insensitive comparisons
- Check for empty strings: `if value == "" { return nil }`
- Use `int64` for database IDs and large row counts

### Database Patterns
- Disable FK checks during batch import: `SET FOREIGN_KEY_CHECKS=0`
- Use `ON DUPLICATE KEY UPDATE` for idempotent operations
- Respect MySQL's 65,535 placeholder limit per statement
- Pass `nil` for NULL database values
- Use prepared statements with `?` placeholders (not string concatenation)

### CSV Processing
- Normalize column names to lowercase: `strings.ToLower(header)`
- Trim whitespace: `strings.TrimSpace(value)`
- Handle missing values gracefully: `if _, ok := row[col]; ok`
- Use ColumnMap for CSV column name → DB column name translation
- Support multi-encoding: UTF-8 → Latin-1 → Windows-1252

### Batch Processing
- Use sequential batching (NOT concurrent) for FK consistency
- Calculate batch capacity: `batchCapacity := batchSize * len(columns)`
- Pre-allocate slices: `batch := make([]interface{}, 0, batchCapacity)`
- Execute final partial batch separately (don't skip)
- Track rows with `rowIdx` not in-memory map

### Type Conversion
- Return `nil` for empty/invalid values (database NULL)
- Use `strconv.ParseInt()` for IDs: `strconv.ParseInt(val, 10, 64)`
- Use `strconv.ParseFloat()` for decimals: `strconv.ParseFloat(val, 64)`
- Use string for dates: `if strings.Contains(col, "date")`
- Check column name patterns, not exact matches

### Function Signatures
Follow existing patterns:
```go
// Preprocessing functions
func (imp *Importer) PopulateSomething(filename string) (map[string]int64, error)

// Import functions
func (imp *Importer) ImportCSV(filename string, config ImportConfig) error

// Batch operations
func (imp *Importer) executeBatch(config ImportConfig, colList string, batchValues []interface{}) error

// Utility functions
func (imp *Importer) logf(format string, args ...interface{})
```

---

## DON'Ts ✗

### Concurrency
- ❌ Do NOT use goroutines (causes FK deadlocks)
- ❌ Do NOT use channels (unnecessary complexity)
- ❌ Do NOT use mutexes for row processing (use sequential loop)

### Error Handling
- ❌ Do NOT ignore errors (use `if err != nil { ... }`)
- ❌ Do NOT panic in production code
- ❌ Do NOT use `log.Fatal` except in main()
- ❌ Do NOT wrap errors twice

### Placeholders & Performance
- ❌ Do NOT exceed 65,535 MySQL placeholders per statement
- ❌ Do NOT use string concatenation for SQL (always use `?` placeholders)
- ❌ Do NOT load entire CSV into memory if > 1GB (already handled by ReadCSV)

### CSV Processing
- ❌ Do NOT assume column exists (always check with `ok` pattern)
- ❌ Do NOT assume CSV column name matches DB column name (use ColumnMap)
- ❌ Do NOT forget to lowercase all CSV column keys
- ❌ Do NOT use raw string values in DB (always convert types with CleanValue)

### Type Handling
- ❌ Do NOT treat empty string as zero (return nil instead)
- ❌ Do NOT trust CSV data types (always convert)
- ❌ Do NOT use pointer types for scalar values (use value types)
- ❌ Do NOT force conversion (check error first, return nil on failure)

### String Operations
- ❌ Do NOT truncate strings without length check: ❌ `s[:10]` (risky)
- ✓ Do check first: `if len(s) > 10 { s = s[:10] }`
- ❌ Do NOT modify error messages beyond 150 characters (impacts logging)

### Struct Usage
- ❌ Do NOT create new ImportConfig at runtime (define in map at package level)
- ❌ Do NOT modify stats outside of ImportStats (use proper locking)
- ❌ Do NOT expose internal DB connection (always use methods)

---

## Common Patterns to Use

### Error Message Template
```go
if err != nil {
    imp.logf("⚠ Error [context]: %v", err)
    return fmt.Errorf("operation failed: %w", err)
}
```

### Safe String Slicing
```go
errMsg := fmt.Sprintf("Error: %v", err)
if len(errMsg) > 150 {
    errMsg = errMsg[:150]
}
```

### Column Mapping Lookup
```go
csvColName := colLower
if mapping, ok := config.ColumnMap[colLower]; ok {
    csvColName = strings.ToLower(mapping)
}
if rowVal, ok := row[csvColName]; ok {
    // Use rowVal
}
```

### Batch Execution
```go
if len(batch) >= batchCapacity {
    if err := imp.executeBatch(config, colList, batch); err != nil {
        stats.Skipped += int64(len(batch) / len(config.Columns))
    } else {
        stats.Inserted += int64(len(batch) / len(config.Columns))
    }
    batch = make([]interface{}, 0, batchCapacity)
}
```

### FK Lookup Mapping
```go
if (colLower == "branded_food_category_id" || colLower == "trade_channel_id") && len(lookupMap) > 0 {
    if mapped, ok := lookupMap[strings.TrimSpace(rowVal)]; ok {
        value = mapped
    }
} else {
    value = imp.CleanValue(rowVal, col)
}
```

---

## Performance Considerations

### Batch Size Tuning
- Calculate: `placeholders = batchSize × columnCount`
- Limit: Must be ≤ 65,535
- For foods (8 cols): max ~8,000 rows per batch (64K placeholders)
- For branded_foods (21 cols): max ~3,000 rows per batch (63K placeholders)
- For nutrient data (11 cols): max ~5,000 rows per batch (55K placeholders)

### Memory Management
- Pre-allocate slices with known capacity
- Reuse batch slice: `batch = make([]interface{}, 0, batchCapacity)`
- Don't keep entire CSV in memory (handled by ReadCSV streaming)

### Database Performance
- Disable FK checks during bulk import
- Use ON DUPLICATE KEY UPDATE for idempotency
- One import per function call (sequential execution)
- Progress logging every 100K rows

---

## Testing Before Committing

1. **Syntax Check**
   ```bash
   go build -o import_gbfpd import_gbfpd.go
   ```

2. **Quick Test on Small Table**
   - Edit `importOrder` to import only one small file
   - Run and verify success
   - Check row counts in MySQL

3. **Full Import Test**
   ```bash
   ./import_gbfpd 2>&1 | tail -20
   ```
   - Verify all tables have "Inserted" count > 0
   - Verify "Skipped" count = 0
   - Check total time (should be ~360 seconds)

4. **Data Integrity Check**
   ```sql
   SELECT COUNT(*) FROM branded_foods WHERE branded_food_category_id IS NULL;
   SELECT * FROM vw_branded_food_nutrients LIMIT 5;
   ```

