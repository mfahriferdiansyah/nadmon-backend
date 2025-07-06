package models

import (
	"strings"
	"time"
)

// EnvioNadmonMinted represents the NadmonNFT_NadmonMinted table from Envio
type EnvioNadmonMinted struct {
	ID               string    `json:"id" db:"id"`
	Owner            string    `json:"owner" db:"owner"`
	TokenID          int64     `json:"tokenId" db:"tokenId"`
	PackID           int64     `json:"packId" db:"packId"`
	Sequence         int64     `json:"sequence" db:"sequence"`
	NadmonType       string    `json:"nadmonType" db:"nadmonType"`
	Element          string    `json:"element" db:"element"`
	Rarity           string    `json:"rarity" db:"rarity"`
	HP               int64     `json:"hp" db:"hp"`
	Attack           int64     `json:"attack" db:"attack"`
	Defense          int64     `json:"defense" db:"defense"`
	Crit             int64     `json:"crit" db:"crit"`
	Fusion           int64     `json:"fusion" db:"fusion"`
	Evo              int64     `json:"evo" db:"evo"`
	DbWriteTimestamp time.Time `json:"dbWriteTimestamp" db:"db_write_timestamp"`
}

// EnvioPackMinted represents the NadmonNFT_PackMinted table from Envio
type EnvioPackMinted struct {
	ID               string    `json:"id" db:"id"`
	Player           string    `json:"player" db:"player"`
	PackID           int64     `json:"packId" db:"packId"`
	Sequence         int64     `json:"sequence" db:"sequence"`
	TokenIDs         []int64   `json:"tokenIds" db:"tokenIds"`
	PaymentType      string    `json:"paymentType" db:"paymentType"`
	DbWriteTimestamp time.Time `json:"dbWriteTimestamp" db:"db_write_timestamp"`
}

// EnvioStatsChanged represents the NadmonNFT_StatsChanged table from Envio
type EnvioStatsChanged struct {
	ID               string    `json:"id" db:"id"`
	TokenID          int64     `json:"tokenId" db:"tokenId"`
	Sequence         int64     `json:"sequence" db:"sequence"`
	ChangeType       string    `json:"changeType" db:"changeType"`
	NewHP            int64     `json:"newHp" db:"newHp"`
	NewAttack        int64     `json:"newAttack" db:"newAttack"`
	NewDefense       int64     `json:"newDefense" db:"newDefense"`
	NewCrit          int64     `json:"newCrit" db:"newCrit"`
	NewFusion        int64     `json:"newFusion" db:"newFusion"`
	NewEvo           int64     `json:"newEvo" db:"newEvo"`
	OldHP            int64     `json:"oldHp" db:"oldHp"`
	OldAttack        int64     `json:"oldAttack" db:"oldAttack"`
	OldDefense       int64     `json:"oldDefense" db:"oldDefense"`
	OldCrit          int64     `json:"oldCrit" db:"oldCrit"`
	OldFusion        int64     `json:"oldFusion" db:"oldFusion"`
	OldEvo           int64     `json:"oldEvo" db:"oldEvo"`
	DbWriteTimestamp time.Time `json:"dbWriteTimestamp" db:"db_write_timestamp"`
}

// EnvioTransfer represents the NadmonNFT_Transfer table from Envio
type EnvioTransfer struct {
	ID               string    `json:"id" db:"id"`
	From             string    `json:"from" db:"from"`
	To               string    `json:"to" db:"to"`
	TokenID          int64     `json:"tokenId" db:"tokenId"`
	DbWriteTimestamp time.Time `json:"dbWriteTimestamp" db:"db_write_timestamp"`
}

// Nadmon represents a complete NFT with current stats (API response model)
type Nadmon struct {
	TokenID     int64     `json:"token_id"`
	Owner       string    `json:"owner"`
	PackID      int64     `json:"pack_id"`
	NadmonType  string    `json:"nadmon_type"`
	Element     string    `json:"element"`
	Rarity      string    `json:"rarity"`
	HP          int64     `json:"hp"`
	Attack      int64     `json:"attack"`
	Defense     int64     `json:"defense"`
	Crit        int64     `json:"crit"`
	Fusion      int64     `json:"fusion"`
	Evo         int64     `json:"evo"`
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
}

// Pack represents a pack purchase (API response model)
type Pack struct {
	PackID      int64     `json:"pack_id"`
	Player      string    `json:"player"`
	TokenIDs    []int64   `json:"token_ids"`
	PaymentType string    `json:"payment_type"`
	PurchasedAt time.Time `json:"purchased_at"`
}

// PlayerProfile represents aggregated player data
type PlayerProfile struct {
	Address     string    `json:"address"`
	TotalNFTs   int       `json:"total_nfts"`
	PacksBought int       `json:"packs_bought"`
	Nadmons     []Nadmon  `json:"nadmons"`
	LastActive  time.Time `json:"last_active"`
}

// StatsChange represents an evolution/fusion event
type StatsChange struct {
	TokenID     int64     `json:"token_id"`
	ChangeType  string    `json:"change_type"`
	Sequence    int64     `json:"sequence"`
	OldStats    StatSet   `json:"old_stats"`
	NewStats    StatSet   `json:"new_stats"`
	ChangedAt   time.Time `json:"changed_at"`
}

// StatSet represents a set of stats
type StatSet struct {
	HP      int64 `json:"hp"`
	Attack  int64 `json:"attack"`
	Defense int64 `json:"defense"`
	Crit    int64 `json:"crit"`
	Fusion  int64 `json:"fusion"`
	Evo     int64 `json:"evo"`
}

// GetImageURL generates the local image path for a Nadmon based on type and evolution
func (n *Nadmon) GetImageURL() string {
	stage := "i"
	if n.Evo == 2 {
		stage = "ii"
	} else if n.Fusion == 10 {
		stage = "max"
	}
	
	// Use local images from /public/monster/ directory (much faster than IPFS!)
	return "/monster/" + strings.ToLower(n.NadmonType) + "-" + stage + ".png"
}

// CalculateSpeed generates speed stat based on other stats (for frontend compatibility)
func (n *Nadmon) CalculateSpeed() int64 {
	return (n.HP + n.Attack + n.Defense) / 10
}

// ToFrontendFormat converts Nadmon to frontend-compatible format
func (n *Nadmon) ToFrontendFormat() map[string]interface{} {
	return map[string]interface{}{
		"id":       int(n.TokenID),
		"name":     n.NadmonType,
		"image":    n.GetImageURL(),
		"hp":       int(n.HP),
		"attack":   int(n.Attack),
		"defense":  int(n.Defense),
		"speed":    int(n.CalculateSpeed()),
		"type":     n.Element,
		"rarity":   n.Rarity,
		"critical": int(n.Crit),
		"color":    GetElementColor(n.Element),
		"fusion":   int(n.Fusion),
		"evo":      int(n.Evo),
	}
}

// GetElementColor returns the color for a given element
func GetElementColor(element string) string {
	colorMap := map[string]string{
		"Fire":     "#ff6b6b",
		"Water":    "#4ecdc4",
		"Nature":   "#95e1d3",
		"Earth":    "#8b5a3c",
		"Electric": "#ffd93d",
		"Ice":      "#74c0fc",
		"Dark":     "#495057",
		"Light":    "#ffd43b",
	}
	
	if color, exists := colorMap[element]; exists {
		return color
	}
	return "#6c757d" // Default gray
}

// PackSummary represents summary statistics for pack purchases
type PackSummary struct {
	TotalPacks    int     `json:"total_packs"`
	MonPacks      int     `json:"mon_packs"`
	CookiesPacks  int     `json:"cookies_packs"`
	RecentPacks   []Pack  `json:"recent_packs"`
}

// GameStats represents overall game statistics
type GameStats struct {
	TotalPlayers      int `json:"total_players"`
	TotalNFTs         int `json:"total_nfts"`
	TotalPacks        int `json:"total_packs"`
	TotalEvolutions   int `json:"total_evolutions"`
	UniqueCollectors  int `json:"unique_collectors"`
}