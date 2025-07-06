package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// EnvioDB wraps a SQL database connection for querying Envio tables
type EnvioDB struct {
	DB *sql.DB
}

// ConnectToEnvio establishes a connection to the Envio PostgreSQL database
func ConnectToEnvio(databaseURL string) (*EnvioDB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for high performance
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(50)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("✅ Connected to Envio PostgreSQL database")
	return &EnvioDB{DB: db}, nil
}

// Close closes the database connection
func (edb *EnvioDB) Close() error {
	return edb.DB.Close()
}

// CreateIndexes creates additional indexes for optimal query performance on Envio tables
func (edb *EnvioDB) CreateIndexes() error {
	log.Println("🔧 Creating indexes on Envio tables...")

	indexes := []string{
		// Indexes for common queries on NadmonMinted
		`CREATE INDEX IF NOT EXISTS idx_nadmon_minted_owner ON "NadmonNFT_NadmonMinted"(owner)`,
		`CREATE INDEX IF NOT EXISTS idx_nadmon_minted_tokenid ON "NadmonNFT_NadmonMinted"("tokenId")`,
		`CREATE INDEX IF NOT EXISTS idx_nadmon_minted_owner_sequence ON "NadmonNFT_NadmonMinted"(owner, sequence DESC)`,
		
		// Indexes for PackMinted queries
		`CREATE INDEX IF NOT EXISTS idx_pack_minted_player ON "NadmonNFT_PackMinted"(player)`,
		`CREATE INDEX IF NOT EXISTS idx_pack_minted_sequence ON "NadmonNFT_PackMinted"(sequence DESC)`,
		
		// Indexes for StatsChanged queries
		`CREATE INDEX IF NOT EXISTS idx_stats_changed_tokenid ON "NadmonNFT_StatsChanged"("tokenId")`,
		`CREATE INDEX IF NOT EXISTS idx_stats_changed_tokenid_sequence ON "NadmonNFT_StatsChanged"("tokenId", sequence DESC)`,
		
		// Indexes for Transfer queries
		`CREATE INDEX IF NOT EXISTS idx_transfer_to ON "NadmonNFT_Transfer"("to")`,
		`CREATE INDEX IF NOT EXISTS idx_transfer_tokenid ON "NadmonNFT_Transfer"("tokenId")`,
	}

	for _, index := range indexes {
		if _, err := edb.DB.Exec(index); err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
			// Continue with other indexes even if one fails
		}
	}

	log.Println("✅ Database indexes created")
	return nil
}

// GetStats returns database statistics from Envio tables
func (edb *EnvioDB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total NFTs
	var totalNFTs int
	err := edb.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_NadmonMinted"`).Scan(&totalNFTs)
	if err != nil {
		return nil, err
	}
	stats["total_nfts"] = totalNFTs

	// Count total packs
	var totalPacks int
	err = edb.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_PackMinted"`).Scan(&totalPacks)
	if err != nil {
		return nil, err
	}
	stats["total_packs"] = totalPacks

	// Count unique players
	var uniquePlayers int
	err = edb.DB.QueryRow(`SELECT COUNT(DISTINCT player) FROM "NadmonNFT_PackMinted"`).Scan(&uniquePlayers)
	if err != nil {
		return nil, err
	}
	stats["unique_players"] = uniquePlayers

	// Count total evolutions
	var totalEvolutions int
	err = edb.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_StatsChanged" WHERE "changeType" = 'evolution'`).Scan(&totalEvolutions)
	if err != nil {
		return nil, err
	}
	stats["total_evolutions"] = totalEvolutions

	return stats, nil
}

// TestConnection tests if the database connection is working and returns sample data
func (edb *EnvioDB) TestConnection() error {
	// First test basic connection
	var version string
	err := edb.DB.QueryRow(`SELECT version()`).Scan(&version)
	if err != nil {
		return err
	}
	log.Printf("✅ Database connection successful - PostgreSQL version: %s", version)
	
	// Check what database we're connected to
	var currentDB string
	err = edb.DB.QueryRow(`SELECT current_database()`).Scan(&currentDB)
	if err != nil {
		return err
	}
	log.Printf("📋 Connected to database: %s", currentDB)
	
	// Test table existence
	var tableExists bool
	err = edb.DB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'NadmonNFT_NadmonMinted'
		)
	`).Scan(&tableExists)
	if err != nil {
		return err
	}
	
	if !tableExists {
		return fmt.Errorf("table NadmonNFT_NadmonMinted does not exist")
	}
	
	// Count NFTs
	var count int
	err = edb.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_NadmonMinted"`).Scan(&count)
	if err != nil {
		return err
	}
	
	log.Printf("✅ Database test successful - found %d NFTs", count)
	return nil
}