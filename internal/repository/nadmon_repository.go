package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"nadmon-backend/internal/database"
	"nadmon-backend/internal/models"

	"github.com/lib/pq"
)

// NadmonRepository handles database operations for Nadmon data
type NadmonRepository struct {
	db *database.EnvioDB
}

// NewNadmonRepository creates a new repository instance
func NewNadmonRepository(db *database.EnvioDB) *NadmonRepository {
	return &NadmonRepository{db: db}
}

// GetPlayerNadmons retrieves all NFTs owned by a player with their current stats
func (r *NadmonRepository) GetPlayerNadmons(address string) ([]models.Nadmon, error) {
	query := `
		WITH current_owners AS (
			-- Get the most recent Transfer event for each token to determine current owner
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		),
		latest_stats AS (
			-- Get the most recent stats for each token
			SELECT DISTINCT ON (s."tokenId")
				s."tokenId", s."newHp", s."newAttack", s."newDefense", 
				s."newCrit", s."newFusion", s."newEvo", s.db_write_timestamp
			FROM "NadmonNFT_StatsChanged" s
			ORDER BY s."tokenId", s.sequence DESC
		)
		SELECT 
			m."tokenId", 
			COALESCE(co.current_owner, m.owner) as owner, 
			m."packId", m."nadmonType", 
			m.element, m.rarity,
			COALESCE(ls."newHp", m.hp) as hp,
			COALESCE(ls."newAttack", m.attack) as attack,
			COALESCE(ls."newDefense", m.defense) as defense,
			COALESCE(ls."newCrit", m.crit) as crit,
			COALESCE(ls."newFusion", m.fusion) as fusion,
			COALESCE(ls."newEvo", m.evo) as evo,
			m.db_write_timestamp as created_at,
			COALESCE(ls.db_write_timestamp, m.db_write_timestamp) as last_updated
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		LEFT JOIN latest_stats ls ON m."tokenId" = ls."tokenId"
		WHERE COALESCE(co.current_owner, m.owner) = $1 
			AND COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
		ORDER BY m."tokenId"
	`

	rows, err := r.db.DB.Query(query, address)
	if err != nil {
		return nil, fmt.Errorf("failed to query player nadmons: %w", err)
	}
	defer rows.Close()

	var nadmons []models.Nadmon
	for rows.Next() {
		var n models.Nadmon
		err := rows.Scan(
			&n.TokenID, &n.Owner, &n.PackID, &n.NadmonType,
			&n.Element, &n.Rarity, &n.HP, &n.Attack,
			&n.Defense, &n.Crit, &n.Fusion, &n.Evo,
			&n.CreatedAt, &n.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nadmon: %w", err)
		}
		nadmons = append(nadmons, n)
	}

	return nadmons, nil
}

// GetPlayerProfile retrieves complete player profile with aggregated stats
func (r *NadmonRepository) GetPlayerProfile(address string) (*models.PlayerProfile, error) {
	// Get player's NFTs
	nadmons, err := r.GetPlayerNadmons(address)
	if err != nil {
		return nil, err
	}

	// Get pack count
	var packCount int
	err = r.db.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_PackMinted" WHERE player = $1`, address).Scan(&packCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count packs: %w", err)
	}

	// Get last activity
	var lastActive sql.NullTime
	err = r.db.DB.QueryRow(`
		SELECT MAX(db_write_timestamp) FROM (
			SELECT db_write_timestamp FROM "NadmonNFT_PackMinted" WHERE player = $1
			UNION ALL
			SELECT s.db_write_timestamp FROM "NadmonNFT_StatsChanged" s
			JOIN "NadmonNFT_NadmonMinted" m ON s."tokenId" = m."tokenId"
			LEFT JOIN (
				SELECT DISTINCT ON (t."tokenId") 
					t."tokenId", t."to" as current_owner
				FROM "NadmonNFT_Transfer" t
				ORDER BY t."tokenId", t.db_write_timestamp DESC
			) co ON m."tokenId" = co."tokenId"
			WHERE COALESCE(co.current_owner, m.owner) = $1
				AND COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
		) combined
	`, address).Scan(&lastActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get last activity: %w", err)
	}

	profile := &models.PlayerProfile{
		Address:     address,
		TotalNFTs:   len(nadmons),
		PacksBought: packCount,
		Nadmons:     nadmons,
	}

	if lastActive.Valid {
		profile.LastActive = lastActive.Time
	}

	return profile, nil
}

// GetPlayerPacks retrieves all pack purchases by a player
func (r *NadmonRepository) GetPlayerPacks(address string) ([]models.Pack, error) {
	query := `
		SELECT "packId", player, "tokenIds", "paymentType", db_write_timestamp
		FROM "NadmonNFT_PackMinted"
		WHERE player = $1
		ORDER BY sequence DESC
	`

	rows, err := r.db.DB.Query(query, address)
	if err != nil {
		return nil, fmt.Errorf("failed to query player packs: %w", err)
	}
	defer rows.Close()

	var packs []models.Pack
	for rows.Next() {
		var p models.Pack
		var tokenIDs pq.Int64Array
		err := rows.Scan(&p.PackID, &p.Player, &tokenIDs, &p.PaymentType, &p.PurchasedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pack: %w", err)
		}
		p.TokenIDs = []int64(tokenIDs)
		packs = append(packs, p)
	}

	return packs, nil
}

// GetNadmonHistory retrieves evolution/fusion history for a specific NFT
func (r *NadmonRepository) GetNadmonHistory(tokenID int64) ([]models.StatsChange, error) {
	query := `
		SELECT "tokenId", "changeType", sequence,
			"newHp", "newAttack", "newDefense", "newCrit", "newFusion", "newEvo",
			"oldHp", "oldAttack", "oldDefense", "oldCrit", "oldFusion", "oldEvo",
			db_write_timestamp
		FROM "NadmonNFT_StatsChanged"
		WHERE "tokenId" = $1
		ORDER BY sequence ASC
	`

	rows, err := r.db.DB.Query(query, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to query nadmon history: %w", err)
	}
	defer rows.Close()

	var changes []models.StatsChange
	for rows.Next() {
		var change models.StatsChange
		err := rows.Scan(
			&change.TokenID, &change.ChangeType, &change.Sequence,
			&change.NewStats.HP, &change.NewStats.Attack, &change.NewStats.Defense,
			&change.NewStats.Crit, &change.NewStats.Fusion, &change.NewStats.Evo,
			&change.OldStats.HP, &change.OldStats.Attack, &change.OldStats.Defense,
			&change.OldStats.Crit, &change.OldStats.Fusion, &change.OldStats.Evo,
			&change.ChangedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stats change: %w", err)
		}
		changes = append(changes, change)
	}

	return changes, nil
}

// GetNadmonsByIDs retrieves multiple NFTs by their token IDs
func (r *NadmonRepository) GetNadmonsByIDs(tokenIDs []int64) ([]models.Nadmon, error) {
	if len(tokenIDs) == 0 {
		return []models.Nadmon{}, nil
	}

	// Build the query with placeholders for token IDs
	placeholders := make([]string, len(tokenIDs))
	args := make([]interface{}, len(tokenIDs))
	for i, id := range tokenIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		WITH current_owners AS (
			-- Get the most recent Transfer event for each token to determine current owner
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		),
		latest_stats AS (
			-- Get the most recent stats for each token
			SELECT DISTINCT ON (s."tokenId")
				s."tokenId", s."newHp", s."newAttack", s."newDefense", 
				s."newCrit", s."newFusion", s."newEvo", s.db_write_timestamp
			FROM "NadmonNFT_StatsChanged" s
			ORDER BY s."tokenId", s.sequence DESC
		)
		SELECT DISTINCT ON (m."tokenId")
			m."tokenId", 
			COALESCE(co.current_owner, m.owner) as owner, 
			m."packId", m."nadmonType", 
			m.element, m.rarity,
			COALESCE(ls."newHp", m.hp) as hp,
			COALESCE(ls."newAttack", m.attack) as attack,
			COALESCE(ls."newDefense", m.defense) as defense,
			COALESCE(ls."newCrit", m.crit) as crit,
			COALESCE(ls."newFusion", m.fusion) as fusion,
			COALESCE(ls."newEvo", m.evo) as evo,
			m.db_write_timestamp as created_at,
			COALESCE(ls.db_write_timestamp, m.db_write_timestamp) as last_updated
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		LEFT JOIN latest_stats ls ON m."tokenId" = ls."tokenId"
		WHERE m."tokenId" IN (%s)
			AND COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
		ORDER BY m."tokenId"
	`, strings.Join(placeholders, ","))

	rows, err := r.db.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query nadmons by IDs: %w", err)
	}
	defer rows.Close()

	var nadmons []models.Nadmon
	for rows.Next() {
		var nadmon models.Nadmon
		err := rows.Scan(
			&nadmon.TokenID, &nadmon.Owner, &nadmon.PackID, &nadmon.NadmonType,
			&nadmon.Element, &nadmon.Rarity,
			&nadmon.HP, &nadmon.Attack, &nadmon.Defense, &nadmon.Crit, &nadmon.Fusion, &nadmon.Evo,
			&nadmon.CreatedAt, &nadmon.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nadmon: %w", err)
		}
		nadmons = append(nadmons, nadmon)
	}

	return nadmons, nil
}

// GetSingleNadmon retrieves a single NFT by token ID with current stats
func (r *NadmonRepository) GetSingleNadmon(tokenID int64) (*models.Nadmon, error) {
	query := `
		WITH current_owners AS (
			-- Get the most recent Transfer event for each token to determine current owner
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		),
		latest_stats AS (
			-- Get the most recent stats for each token
			SELECT DISTINCT ON (s."tokenId")
				s."tokenId", s."newHp", s."newAttack", s."newDefense", 
				s."newCrit", s."newFusion", s."newEvo", s.db_write_timestamp
			FROM "NadmonNFT_StatsChanged" s
			ORDER BY s."tokenId", s.sequence DESC
		)
		SELECT DISTINCT ON (m."tokenId")
			m."tokenId", 
			COALESCE(co.current_owner, m.owner) as owner, 
			m."packId", m."nadmonType", 
			m.element, m.rarity,
			COALESCE(ls."newHp", m.hp) as hp,
			COALESCE(ls."newAttack", m.attack) as attack,
			COALESCE(ls."newDefense", m.defense) as defense,
			COALESCE(ls."newCrit", m.crit) as crit,
			COALESCE(ls."newFusion", m.fusion) as fusion,
			COALESCE(ls."newEvo", m.evo) as evo,
			m.db_write_timestamp as created_at,
			COALESCE(ls.db_write_timestamp, m.db_write_timestamp) as last_updated
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		LEFT JOIN latest_stats ls ON m."tokenId" = ls."tokenId"
		WHERE m."tokenId" = $1
			AND COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
		ORDER BY m."tokenId"
	`

	var nadmon models.Nadmon
	err := r.db.DB.QueryRow(query, tokenID).Scan(
		&nadmon.TokenID, &nadmon.Owner, &nadmon.PackID, &nadmon.NadmonType,
		&nadmon.Element, &nadmon.Rarity,
		&nadmon.HP, &nadmon.Attack, &nadmon.Defense, &nadmon.Crit, &nadmon.Fusion, &nadmon.Evo,
		&nadmon.CreatedAt, &nadmon.LastUpdated,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query single nadmon: %w", err)
	}

	return &nadmon, nil
}

// GetPackByID retrieves a specific pack by its ID
func (r *NadmonRepository) GetPackByID(packID int64) (*models.Pack, error) {
	query := `
		SELECT "packId", player, "tokenIds", "paymentType", db_write_timestamp
		FROM "NadmonNFT_PackMinted"
		WHERE "packId" = $1
	`

	var pack models.Pack
	var tokenIDsStr string
	err := r.db.DB.QueryRow(query, packID).Scan(
		&pack.PackID, &pack.Player, &tokenIDsStr, &pack.PaymentType, &pack.PurchasedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query pack: %w", err)
	}

	// Parse token IDs - handle both PostgreSQL array format and JSON format
	if err := json.Unmarshal([]byte(tokenIDsStr), &pack.TokenIDs); err != nil {
		// Try parsing as PostgreSQL array format: {1,2,3,4,5}
		if strings.HasPrefix(tokenIDsStr, "{") && strings.HasSuffix(tokenIDsStr, "}") {
			// Remove braces and split by comma
			inner := strings.Trim(tokenIDsStr, "{}")
			if inner == "" {
				pack.TokenIDs = []int64{}
			} else {
				parts := strings.Split(inner, ",")
				pack.TokenIDs = make([]int64, len(parts))
				for i, part := range parts {
					id, parseErr := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
					if parseErr != nil {
						return nil, fmt.Errorf("failed to parse token ID %s: %w", part, parseErr)
					}
					pack.TokenIDs[i] = id
				}
			}
		} else {
			return nil, fmt.Errorf("failed to parse token IDs: %w", err)
		}
	}

	return &pack, nil
}

// GetRecentPacks retrieves the most recent pack purchases
func (r *NadmonRepository) GetRecentPacks(limit int) ([]models.Pack, error) {
	query := `
		SELECT "packId", player, "tokenIds", "paymentType", db_write_timestamp
		FROM "NadmonNFT_PackMinted"
		ORDER BY sequence DESC
		LIMIT $1
	`

	rows, err := r.db.DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent packs: %w", err)
	}
	defer rows.Close()

	var packs []models.Pack
	for rows.Next() {
		var p models.Pack
		var tokenIDs pq.Int64Array
		err := rows.Scan(&p.PackID, &p.Player, &tokenIDs, &p.PaymentType, &p.PurchasedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pack: %w", err)
		}
		p.TokenIDs = []int64(tokenIDs)
		packs = append(packs, p)
	}

	return packs, nil
}

// GetTopCollectors retrieves players with the most NFTs
func (r *NadmonRepository) GetTopCollectors(limit int) ([]models.PlayerProfile, error) {
	query := `
		WITH current_owners AS (
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		)
		SELECT 
			COALESCE(co.current_owner, m.owner) as owner, 
			COUNT(*) as nft_count
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		WHERE COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
		GROUP BY COALESCE(co.current_owner, m.owner)
		ORDER BY nft_count DESC
		LIMIT $1
	`

	rows, err := r.db.DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top collectors: %w", err)
	}
	defer rows.Close()

	var profiles []models.PlayerProfile
	for rows.Next() {
		var profile models.PlayerProfile
		err := rows.Scan(&profile.Address, &profile.TotalNFTs)
		if err != nil {
			return nil, fmt.Errorf("failed to scan collector: %w", err)
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// SearchNadmons searches for NFTs by various criteria
func (r *NadmonRepository) SearchNadmons(address string, filters map[string]interface{}) ([]models.Nadmon, error) {
	baseQuery := `
		WITH current_owners AS (
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		),
		latest_stats AS (
			SELECT DISTINCT ON (s."tokenId")
				s."tokenId", s."newHp", s."newAttack", s."newDefense", 
				s."newCrit", s."newFusion", s."newEvo", s.db_write_timestamp
			FROM "NadmonNFT_StatsChanged" s
			ORDER BY s."tokenId", s.sequence DESC
		)
		SELECT 
			m."tokenId", 
			COALESCE(co.current_owner, m.owner) as owner, 
			m."packId", m."nadmonType", 
			m.element, m.rarity,
			COALESCE(ls."newHp", m.hp) as hp,
			COALESCE(ls."newAttack", m.attack) as attack,
			COALESCE(ls."newDefense", m.defense) as defense,
			COALESCE(ls."newCrit", m.crit) as crit,
			COALESCE(ls."newFusion", m.fusion) as fusion,
			COALESCE(ls."newEvo", m.evo) as evo,
			m.db_write_timestamp as created_at,
			COALESCE(ls.db_write_timestamp, m.db_write_timestamp) as last_updated
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		LEFT JOIN latest_stats ls ON m."tokenId" = ls."tokenId"
		WHERE COALESCE(co.current_owner, m.owner) = $1 
			AND COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
	`

	var conditions []string
	var args []interface{}
	args = append(args, address)
	argIndex := 2

	// Add filters
	if element, ok := filters["element"].(string); ok && element != "" {
		conditions = append(conditions, fmt.Sprintf("m.element = $%d", argIndex))
		args = append(args, element)
		argIndex++
	}

	if rarity, ok := filters["rarity"].(string); ok && rarity != "" {
		conditions = append(conditions, fmt.Sprintf("m.rarity = $%d", argIndex))
		args = append(args, rarity)
		argIndex++
	}

	if nadmonType, ok := filters["type"].(string); ok && nadmonType != "" {
		conditions = append(conditions, fmt.Sprintf("m.\"nadmonType\" = $%d", argIndex))
		args = append(args, nadmonType)
		argIndex++
	}

	if evo, ok := filters["evo"].(int); ok && evo > 0 {
		conditions = append(conditions, fmt.Sprintf("COALESCE(s.\"newEvo\", m.evo) = $%d", argIndex))
		args = append(args, evo)
		argIndex++
	}

	// Add conditions to query
	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	baseQuery += " ORDER BY m.\"tokenId\", s.sequence DESC NULLS LAST"

	rows, err := r.db.DB.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search nadmons: %w", err)
	}
	defer rows.Close()

	var nadmons []models.Nadmon
	for rows.Next() {
		var n models.Nadmon
		err := rows.Scan(
			&n.TokenID, &n.Owner, &n.PackID, &n.NadmonType,
			&n.Element, &n.Rarity, &n.HP, &n.Attack,
			&n.Defense, &n.Crit, &n.Fusion, &n.Evo,
			&n.CreatedAt, &n.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nadmon: %w", err)
		}
		nadmons = append(nadmons, n)
	}

	return nadmons, nil
}

// GetGameStats retrieves overall game statistics
func (r *NadmonRepository) GetGameStats() (*models.GameStats, error) {
	stats := &models.GameStats{}

	// Total NFTs (excluding burned ones)
	err := r.db.DB.QueryRow(`
		WITH current_owners AS (
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		)
		SELECT COUNT(*) 
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		WHERE COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
	`).Scan(&stats.TotalNFTs)
	if err != nil {
		return nil, fmt.Errorf("failed to count NFTs: %w", err)
	}

	// Total packs
	err = r.db.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_PackMinted"`).Scan(&stats.TotalPacks)
	if err != nil {
		return nil, fmt.Errorf("failed to count packs: %w", err)
	}

	// Unique collectors (excluding those who only have burned NFTs)
	err = r.db.DB.QueryRow(`
		WITH current_owners AS (
			SELECT DISTINCT ON (t."tokenId") 
				t."tokenId", 
				t."to" as current_owner
			FROM "NadmonNFT_Transfer" t
			ORDER BY t."tokenId", t.db_write_timestamp DESC
		)
		SELECT COUNT(DISTINCT COALESCE(co.current_owner, m.owner)) 
		FROM "NadmonNFT_NadmonMinted" m
		LEFT JOIN current_owners co ON m."tokenId" = co."tokenId"
		WHERE COALESCE(co.current_owner, m.owner) != '0x0000000000000000000000000000000000000000'
	`).Scan(&stats.UniqueCollectors)
	if err != nil {
		return nil, fmt.Errorf("failed to count collectors: %w", err)
	}

	// Total evolutions
	err = r.db.DB.QueryRow(`SELECT COUNT(*) FROM "NadmonNFT_StatsChanged" WHERE "changeType" = 'evolution'`).Scan(&stats.TotalEvolutions)
	if err != nil {
		return nil, fmt.Errorf("failed to count evolutions: %w", err)
	}

	// Total players (unique pack buyers)
	err = r.db.DB.QueryRow(`SELECT COUNT(DISTINCT player) FROM "NadmonNFT_PackMinted"`).Scan(&stats.TotalPlayers)
	if err != nil {
		return nil, fmt.Errorf("failed to count players: %w", err)
	}

	return stats, nil
}