# USDA FoodData Central Normalized Database Schema

## Overview
This schema implements a proper normalization of the USDA FoodData Central branded food dataset, following the official API structure and database design principles.

## Database Tables

### Reference/Lookup Tables

#### `nutrient_sources`
Defines where nutrient data comes from (e.g., "Analytical or derived from analytical")
- `id`: Primary key
- `code`: Unique code identifier
- `description`: Full description

#### `nutrient_derivations`
Describes how nutrient values were calculated (e.g., "Analytical", "Derived")
- `id`: Primary key
- `code`: Code identifier  
- `description`: Description of derivation method
- `source_id`: FK to nutrient_sources

#### `measure_units`
Standard units of measurement (cup, gram, ml, oz, etc.)
- `id`: Primary key
- `name`: Unit name (unique)

#### `nutrients`
Master list of all nutrients tracked (Protein, Fat, Carbs, Fiber, etc.)
- `id`: Primary key (from USDA nutrient database)
- `name`: Full nutrient name
- `unit_name`: Default unit for this nutrient
- `nutrient_nbr`: USDA nutrient number
- `rank`: USDA ranking/priority

#### `food_attribute_types`
Types of food attributes (e.g., "Ingredients", "Allergens", "Update Log")
- `id`: Primary key
- `name`: Attribute type name
- `description`: What this attribute represents

### Main Food Tables

#### `foods`
Core table containing all food items (both branded and unbranded)
- `fdc_id`: Primary key (USDA FDC ID)
- `data_type`: Type of food ("branded_food", etc.)
- `description`: Full food description
- `food_category_id`: Food category
- `publication_date`: When this food was published
- `market_country`: Country where available
- `trade_channel`: Distribution channel

#### `branded_foods`
Extends foods table with brand-specific information
- `fdc_id`: PK/FK to foods table
- `brand_owner`: Company that owns the brand
- `brand_name`: Brand name
- `subbrand_name`: Sub-brand name (optional)
- `gtin_upc`: UPC code for the product
- `ingredients`: LONGTEXT of ingredients
- `serving_size`: Amount of one serving
- `serving_size_unit_id`: FK to measure_units
- `household_serving_fulltext`: Description like "1/2 cup"
- `branded_food_category`: USDA category
- `data_source`: Source of this data
- `modified_date`: Last modification date
- `available_date`: When product became available
- `discontinued_date`: If applicable
- Indexed for fast queries on brand_owner, brand_name, gtin_upc

### Nutrient Data Tables

#### `food_nutrients`
Nutrient content for each food item
- `id`: Primary key (unique ID for this measurement)
- `fdc_id`: FK to foods table
- `nutrient_id`: FK to nutrients table
- `amount`: Nutrient quantity per serving
- `data_points`: Number of data points in measurement
- `derivation_id`: FK to nutrient_derivations
- `min_value`, `max_value`, `median_value`: Statistical ranges
- `footnote`: Additional notes
- `min_year_acquired`: Year of data acquisition

**Unique Index**: (fdc_id, nutrient_id) prevents duplicate nutrient entries per food

### Food Attribute Tables

#### `food_attributes`
Flexible attribute storage for food metadata
- `id`: Primary key
- `fdc_id`: FK to foods table
- `food_attribute_type_id`: FK to food_attribute_types
- `seq_num`: Sequence number
- `name`: Attribute name
- `value`: Attribute value

### Microbe Data Tables

#### `microbes`
Microbiological test results
- `id`: Primary key
- `fdc_id`: FK to foods table
- `method`: Test method used
- `microbe_code`: Microbe identifier
- `min_value`, `max_value`: Detected ranges
- `uom`: Unit of measurement

### Audit Tables

#### `food_update_log_entries`
Change log for food items
- `id`: Primary key
- `description`: What was changed
- `last_updated`: Date of last update

## Pre-built Views

### `vw_branded_food_nutrients`
Join of branded foods with their nutrient data, ready for analysis
- Includes brand info, serving size, nutrient name/unit, amounts

### `vw_branded_food_macros`
Quick macro nutrient summary (Energy, Protein, Fat, Carbs, Fiber)
- Pivoted view for easy macro analysis per product

## Key Features

✅ **Proper Normalization**
- No data duplication
- Foreign keys enforce referential integrity
- Unique constraints prevent duplicates

✅ **Performance Optimizations**
- Indexes on frequently queried columns (brand, GTIN, nutrient ID)
- FULLTEXT indexes on description and ingredients for keyword search
- Unique index on (fdc_id, nutrient_id) prevents duplicate nutrients

✅ **UTF-8 Support**
- Full Unicode support for international product names and ingredients

✅ **Audit Trail**
- Created/updated timestamps on all main tables
- Tracks when data was inserted or modified

✅ **USDA Compliance**
- Follows FoodData Central API schema
- Supports all official USDA data types and relationships

## Usage

### Run the import:
```bash
DB_USER=root ./import_gbfpd_normalized.sh
```

### Query examples:

**All branded foods with macros:**
```sql
SELECT * FROM vw_branded_food_macros 
WHERE brand_name LIKE '%Coca%' 
LIMIT 10;
```

**Find products by GTIN:**
```sql
SELECT * FROM branded_foods 
WHERE gtin_upc = '00072940755050';
```

**Nutrient analysis:**
```sql
SELECT 
    n.name,
    COUNT(*) as products_with_nutrient,
    AVG(fn.amount) as avg_amount,
    MIN(fn.amount) as min_amount,
    MAX(fn.amount) as max_amount
FROM food_nutrients fn
JOIN nutrients n ON fn.nutrient_id = n.id
GROUP BY n.id, n.name
ORDER BY COUNT(*) DESC
LIMIT 20;
```

**Search by ingredient:**
```sql
SELECT 
    gtin_upc,
    brand_name,
    brand_owner,
    ingredients
FROM branded_foods
WHERE MATCH(ingredients) AGAINST('sugar' IN BOOLEAN MODE);
```

## Data Integrity Constraints

- Foreign keys prevent orphaned records
- Unique constraints prevent duplicate nutrients per food
- Check constraints on numeric fields
- NOT NULL on critical identifiers (fdc_id, nutrient_id)
