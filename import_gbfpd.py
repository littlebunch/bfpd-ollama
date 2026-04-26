#!/usr/bin/env python3

"""
USDA FoodData Central Database Importer
Robust CSV import with error handling and data validation
"""

import csv
import sys
import mysql.connector
from mysql.connector import Error
from pathlib import Path
import json
from datetime import datetime
import argparse

# Configuration
CONFIG = {
    "host": "localhost",
    "user": "gmoore",
    "password": "maggie2pie",
    "database": "gbfpd",
    "data_dir": "/Users/gmoore/bfpd/2025-12/FoodData_Central_branded_food_csv_2025-12-18"
}

# CSV file to table mapping with optional column mapping
IMPORTS = {
    "nutrient_source.csv": {
        "table": "nutrient_sources",
        "columns": ["id", "code", "description"]
    },
    "food_nutrient_derivation.csv": {
        "table": "nutrient_derivations",
        "columns": ["id", "code", "description", "source_id"]
    },
    "measure_unit.csv": {
        "table": "measure_units",
        "columns": ["id", "name"]
    },
    "nutrient.csv": {
        "table": "nutrients",
        "columns": ["id", "name", "unit_name", "nutrient_nbr", "rank"]
    },
    "food_attribute_type.csv": {
        "table": "food_attribute_types",
        "columns": ["id", "name", "description"]
    },
    "food.csv": {
        "table": "foods",
        "columns": ["fdc_id", "data_type", "description", "food_category_id", 
                   "publication_date", "market_country", "trade_channel_id", "microbe_data"],
        "preprocessing": "populate_trade_channels"
    },
    "branded_food.csv": {
        "table": "branded_foods",
        "columns": ["fdc_id", "brand_owner", "brand_name", "subbrand_name", "gtin_upc",
                   "ingredients", "not_a_significant_source_of", "serving_size", "serving_size_unit",
                   "household_serving_fulltext", "branded_food_category_id", "data_source", "package_weight",
                   "modified_date", "available_date", "market_country", "discontinued_date",
                   "preparation_state_code", "trade_channel", "short_description", "material_code"],
        "preprocessing": "populate_branded_food_categories"
    },
    "food_nutrient.csv": {
        "table": "food_nutrients",
        "columns": ["id", "fdc_id", "nutrient_id", "amount", "data_points", "derivation_id",
                   "`min`", "`max`", "`median`", "footnote", "min_year_acquired"]
    },
    "food_attribute.csv": {
        "table": "food_attributes",
        "columns": ["id", "fdc_id", "seq_num", "food_attribute_type_id", "name", "value"]
    },
    "microbe.csv": {
        "table": "microbes",
        "columns": ["id", "fdc_id", "method", "microbe_code", "min_value", "max_value", "uom"],
        "column_map": {"fdc_id": "foodId"}  # CSV column name mapping
    },
    "food_update_log_entry.csv": {
        "table": "food_update_log_entries",
        "columns": ["id", "description", "last_updated"]
    }
}


class FoodDataImporter:
    def __init__(self, config):
        self.config = config
        self.connection = None
        self.cursor = None
        self.stats = {}
        
    def connect(self, database=None):
        """Connect to MySQL database"""
        try:
            config = {k: v for k, v in self.config.items() if k != 'data_dir'}
            if database:
                config['database'] = database
            self.connection = mysql.connector.connect(**config)
            self.cursor = self.connection.cursor()
            db_name = database if database else self.config['database']
            print(f"✓ Connected to MySQL: {db_name}")
        except Error as e:
            print(f"✗ Connection failed: {e}")
            sys.exit(1)
    
    def disconnect(self):
        """Close database connection"""
        if self.cursor:
            self.cursor.close()
        if self.connection:
            self.connection.close()
        print("✓ Disconnected from database")
    
    def create_database_and_schema(self):
        """Drop existing database and create fresh schema"""
        # Connect to default database first
        self.connect(database='mysql')
        
        db_name = self.config['database']
        print(f"\n{'='*60}")
        print("Creating Database and Schema")
        print(f"{'='*60}")
        
        # Drop existing database
        print(f"Dropping existing database if exists...")
        self.cursor.execute(f"DROP DATABASE IF EXISTS {db_name}")
        self.connection.commit()
        print(f"✓ Dropped {db_name}")
        
        # Create database
        print(f"Creating database {db_name}...")
        self.cursor.execute(f"CREATE DATABASE {db_name} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
        self.connection.commit()
        print(f"✓ Created {db_name}")
        
        # Disconnect from mysql, reconnect to gbfpd
        self.disconnect()
        self.connect()
        
        # Create all tables
        print("\nCreating tables...")
        
        schema_sql = """
        -- Reference Tables
        CREATE TABLE IF NOT EXISTS nutrient_sources (
            id INT PRIMARY KEY,
            code VARCHAR(50) NOT NULL UNIQUE,
            description VARCHAR(255) NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS nutrient_derivations (
            id INT PRIMARY KEY,
            code VARCHAR(50) NOT NULL UNIQUE,
            description VARCHAR(255) NOT NULL,
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

        -- Branded Food Categories
        CREATE TABLE IF NOT EXISTS branded_food_categories (
            id INT AUTO_INCREMENT PRIMARY KEY,
            category_name TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            UNIQUE KEY unique_category (category_name(500))
        );

        -- Trade Channels
        CREATE TABLE IF NOT EXISTS trade_channels (
            id INT AUTO_INCREMENT PRIMARY KEY,
            channel_name VARCHAR(255) NOT NULL UNIQUE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );

        -- Main Food Tables
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

        -- Nutrient Data Tables
        CREATE TABLE IF NOT EXISTS food_nutrients (
            id BIGINT PRIMARY KEY,
            fdc_id INT NOT NULL,
            nutrient_id INT NOT NULL,
            amount DECIMAL(30,10),
            data_points INT,
            derivation_id INT,
            `min` DECIMAL(30,10),
            `max` DECIMAL(30,10),
            `median` DECIMAL(30,10),
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

        -- Food Attributes
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

        -- Microbes
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

        -- Update Log
        CREATE TABLE IF NOT EXISTS food_update_log_entries (
            id INT PRIMARY KEY,
            description TEXT,
            last_updated DATE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );

        -- Views
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
            fn.`min` as min_value,
            fn.`max` as max_value,
            fn.`median` as median_value,
            fn.data_points
        FROM branded_foods bf
        LEFT JOIN branded_food_categories bfc ON bf.branded_food_category_id = bfc.id
        LEFT JOIN food_nutrients fn ON bf.fdc_id = fn.fdc_id
        LEFT JOIN nutrients n ON fn.nutrient_id = n.id
        ORDER BY bf.fdc_id, n.rank DESC;

        -- View: Branded foods with key macronutrients
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
        """
        
        # Execute each statement
        for statement in schema_sql.split(';'):
            if statement.strip():
                try:
                    self.cursor.execute(statement)
                    self.connection.commit()
                except Exception as e:
                    print(f"  ⚠ Warning: {str(e)[:80]}")
        
        print("✓ Schema created successfully")
    
    def clean_value(self, value, col_name=None):
        """Clean and convert values for insertion"""
        if value is None or value == '' or value == 'NULL':
            return None
        
        # Handle numeric fields
        if 'id' in col_name.lower() and col_name not in ['microbe_code']:
            try:
                return int(float(value))
            except (ValueError, TypeError):
                return None
        
        # Handle decimal/float fields
        if any(x in col_name.lower() for x in ['amount', 'min', 'max', 'median', 'weight', 'rank', 'size']):
            try:
                return float(value)
            except (ValueError, TypeError):
                return None
        
        # Handle date fields
        if 'date' in col_name.lower():
            if value and value.strip():
                return value.strip()
            return None
        
        # String fields - strip whitespace
        if isinstance(value, str):
            return value.strip() if value.strip() else None
        
        return value
    
    def read_csv(self, filepath):
        """Read CSV file with proper encoding detection"""
        encodings = ['utf-8', 'utf-8-sig', 'latin-1', 'cp1252']
        
        for encoding in encodings:
            try:
                with open(filepath, 'r', encoding=encoding) as f:
                    # Try to read first few lines
                    reader = csv.DictReader(f)
                    rows = list(reader)
                    return rows, reader.fieldnames
            except (UnicodeDecodeError, Exception) as e:
                continue
        
        raise Exception(f"Could not read {filepath} with any encoding")
    
    def populate_branded_food_categories(self, csv_filename):
        """Extract unique branded_food_category values and populate lookup table"""
        filepath = Path(self.config['data_dir']) / csv_filename
        
        print(f"\n  Preprocessing: Extracting unique categories from {csv_filename}...")
        
        try:
            rows, fieldnames = self.read_csv(filepath)
            
            if not rows:
                return {}
            
            # Find the category column
            category_col = None
            for field in fieldnames:
                if field.strip('"').lower() == 'branded_food_category':
                    category_col = field
                    break
            
            if not category_col:
                print("    ⚠ Column 'branded_food_category' not found")
                return {}
            
            # Extract unique categories
            categories = set()
            for row in rows:
                cat = row.get(category_col, '').strip() if row.get(category_col) else None
                if cat:
                    categories.add(cat)
            
            print(f"    Found {len(categories)} unique categories")
            
            # Insert categories into lookup table
            category_map = {}
            for cat in sorted(categories):
                try:
                    self.cursor.execute(
                        "INSERT INTO branded_food_categories (category_name) VALUES (%s) "
                        "ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)",
                        (cat,)
                    )
                    self.connection.commit()
                    
                    # Get the ID
                    self.cursor.execute("SELECT LAST_INSERT_ID()")
                    category_id = self.cursor.fetchone()[0]
                    category_map[cat] = category_id
                except Exception as e:
                    print(f"    ⚠ Error inserting category '{cat}': {str(e)[:80]}")
            
            print(f"    ✓ Populated {len(category_map)} categories")
            return category_map
            
        except Exception as e:
            print(f"    ✗ Error: {e}")
            return {}
    
    def populate_trade_channels(self, csv_filename):
        """Extract unique trade_channel values and populate lookup table"""
        filepath = Path(self.config['data_dir']) / csv_filename
        
        print(f"\n  Preprocessing: Extracting unique trade channels from {csv_filename}...")
        
        try:
            rows, fieldnames = self.read_csv(filepath)
            
            if not rows:
                return {}
            
            # Find the trade_channel column
            trade_channel_col = None
            for field in fieldnames:
                if field.strip('"').lower() == 'trade_channel':
                    trade_channel_col = field
                    break
            
            if not trade_channel_col:
                print("    ⚠ Column 'trade_channel' not found")
                return {}
            
            # Extract unique channels
            channels = set()
            for row in rows:
                ch = row.get(trade_channel_col, '').strip() if row.get(trade_channel_col) else None
                if ch:
                    channels.add(ch)
            
            print(f"    Found {len(channels)} unique trade channels")
            
            # Insert channels into lookup table
            channel_map = {}
            for ch in sorted(channels):
                try:
                    self.cursor.execute(
                        "INSERT INTO trade_channels (channel_name) VALUES (%s) "
                        "ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)",
                        (ch,)
                    )
                    self.connection.commit()
                    
                    # Get the ID
                    self.cursor.execute("SELECT LAST_INSERT_ID()")
                    channel_id = self.cursor.fetchone()[0]
                    channel_map[ch] = channel_id
                except Exception as e:
                    print(f"    ⚠ Error inserting channel '{ch}': {str(e)[:80]}")
            
            print(f"    ✓ Populated {len(channel_map)} trade channels")
            return channel_map
            
        except Exception as e:
            print(f"    ✗ Error: {e}")
            return {}
    
    def import_csv(self, csv_filename, config):
        """Import a single CSV file using optimized batch inserts"""
        filepath = Path(self.config['data_dir']) / csv_filename
        
        if not filepath.exists():
            print(f"⚠ File not found: {filepath}")
            return 0
        
        table_name = config['table']
        expected_columns = config['columns']
        batch_size = 10000
        
        print(f"\nImporting {csv_filename} → {table_name}...")
        
        # Run preprocessing if specified
        category_map = {}
        if "preprocessing" in config:
            preprocessing_func = getattr(self, config["preprocessing"], None)
            if preprocessing_func:
                category_map = preprocessing_func(csv_filename)
        
        try:
            rows, fieldnames = self.read_csv(filepath)
            
            if not rows:
                print(f"  ⚠ No data rows found")
                return 0
            
            csv_columns = {f.strip('"').lower(): f for f in fieldnames}
            
            # Get column mapping if it exists
            column_map = config.get("column_map", {})
            
            # Disable foreign key checks during import for large datasets
            try:
                self.cursor.execute("SET FOREIGN_KEY_CHECKS=0")
                self.connection.commit()
            except:
                pass
            
            placeholders = ','.join(['%s'] * len(expected_columns))
            insert_sql = f"""
                INSERT INTO {table_name} ({','.join(expected_columns)})
                VALUES ({placeholders})
                ON DUPLICATE KEY UPDATE
                {','.join([f'{col}=VALUES({col})' for col in expected_columns[1:]])}
            """
            
            inserted = 0
            skipped = 0
            errors = []
            batch_data = []
            
            for row_idx, row in enumerate(rows, 2):
                try:
                    values = []
                    for col in expected_columns:
                        col_lower = col.lower().replace('`', '')  # Remove backticks from column name
                        
                        # Check if there's a mapping for this column
                        if col_lower in column_map:
                            csv_col_name = column_map[col_lower]
                        else:
                            csv_col_name = col_lower
                        
                        # Find matching CSV column
                        csv_col = None
                        for key in csv_columns:
                            if key == csv_col_name:
                                csv_col = csv_columns[key]
                                break
                        
                        if csv_col is None:
                            values.append(None)
                        else:
                            val = row.get(csv_col, '')
                            
                            # Handle category mapping for branded_food_category_id
                            if col_lower == 'branded_food_category_id' and category_map:
                                val_str = val.strip() if isinstance(val, str) else str(val)
                                if val_str in category_map:
                                    values.append(category_map[val_str])
                                else:
                                    values.append(None)
                            # Handle trade_channel_id mapping
                            elif col_lower == 'trade_channel_id' and category_map:
                                val_str = val.strip() if isinstance(val, str) else str(val)
                                if val_str in category_map:
                                    values.append(category_map[val_str])
                                else:
                                    values.append(None)
                            else:
                                cleaned = self.clean_value(val, col)
                                values.append(cleaned)
                    
                    batch_data.append(tuple(values))
                    
                    # Execute batch when it reaches batch_size
                    if len(batch_data) >= batch_size:
                        self.cursor.executemany(insert_sql, batch_data)
                        self.connection.commit()
                        inserted += len(batch_data)
                        batch_data = []
                        if row_idx % (batch_size * 5) == 0:
                            print(f"  ✓ Progress: {inserted:,} rows inserted...")
                    
                except Exception as e:
                    skipped += 1
                    if skipped <= 5:
                        errors.append(f"Row {row_idx}: {str(e)[:150]}")
            
            # Insert remaining rows in final batch
            if batch_data:
                self.cursor.executemany(insert_sql, batch_data)
                self.connection.commit()
                inserted += len(batch_data)
            
            # Re-enable foreign key checks
            try:
                self.cursor.execute("SET FOREIGN_KEY_CHECKS=1")
                self.connection.commit()
            except:
                pass
            
            print(f"  ✓ Inserted: {inserted:,} rows")
            if skipped > 0:
                print(f"  ⚠ Skipped: {skipped:,} rows")
            if errors:
                print(f"  Errors encountered:")
                for error in errors[:5]:
                    print(f"    {error}")
            
            self.stats[table_name] = {"inserted": inserted, "skipped": skipped}
            return inserted
            
        except Exception as e:
            # Re-enable foreign key checks even on error
            try:
                self.cursor.execute("SET FOREIGN_KEY_CHECKS=1")
                self.connection.commit()
            except:
                pass
            print(f"  ✗ Error: {e}")
            import traceback
            traceback.print_exc()
            return 0
    
    def run_imports(self):
        """Run all imports"""
        # Create database and schema first
        self.create_database_and_schema()
        
        total_inserted = 0
        
        # Import in dependency order
        import_order = [
            "nutrient_source.csv",
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
        ]
        
        for csv_file in import_order:
            if csv_file in IMPORTS:
                inserted = self.import_csv(csv_file, IMPORTS[csv_file])
                total_inserted += inserted
        
        self.print_summary()
        self.disconnect()
    
    def print_summary(self):
        """Print import summary"""
        print("\n" + "="*60)
        print("IMPORT SUMMARY")
        print("="*60)
        
        for table, stats in self.stats.items():
            print(f"{table:30} Inserted: {stats['inserted']:>10,}  Skipped: {stats['skipped']:>10,}")
        
        total_inserted = sum(s['inserted'] for s in self.stats.values())
        total_skipped = sum(s['skipped'] for s in self.stats.values())
        
        print("-"*60)
        print(f"{'TOTAL':30} Inserted: {total_inserted:>10,}  Skipped: {total_skipped:>10,}")
        print("="*60)


def main():
    parser = argparse.ArgumentParser(description='Import USDA FoodData Central CSVs')
    parser.add_argument('--host', default=CONFIG['host'], help='MySQL host')
    parser.add_argument('--user', default=CONFIG['user'], help='MySQL user')
    parser.add_argument('--password', default=CONFIG['password'], help='MySQL password')
    parser.add_argument('--database', default=CONFIG['database'], help='Database name')
    parser.add_argument('--data-dir', default=CONFIG['data_dir'], help='Data directory')
    
    args = parser.parse_args()
    
    config = {
        'host': args.host,
        'user': args.user,
        'password': args.password if args.password else '',
        'database': args.database,
        'data_dir': args.data_dir
    }
    
    importer = FoodDataImporter(config)
    importer.run_imports()


if __name__ == '__main__':
    main()
