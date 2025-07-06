package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"nadmon-backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type NadmonHandler struct {
	repo *repository.NadmonRepository
}

// NewNadmonHandler creates a new handler with repository
func NewNadmonHandler(repo *repository.NadmonRepository) *NadmonHandler {
	return &NadmonHandler{repo: repo}
}

// PaginationQuery represents pagination parameters
type PaginationQuery struct {
	Page  int `form:"page,default=1"`
	Limit int `form:"limit,default=20"`
}

// SearchQuery represents search parameters
type SearchQuery struct {
	Element    string `form:"element"`
	Rarity     string `form:"rarity"`
	Type       string `form:"type"`
	Evo        int    `form:"evo"`
	MinHP      int    `form:"min_hp"`
	MinAttack  int    `form:"min_attack"`
	MinDefense int    `form:"min_defense"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"totalPages"`
	HasNext    bool        `json:"hasNext"`
	HasPrev    bool        `json:"hasPrev"`
}

// GetInventory returns NFT inventory for an address
func (h *NadmonHandler) GetInventory(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Address parameter required"})
		return
	}

	// Validate Ethereum address format
	if !isValidEthereumAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Ethereum address format"})
		return
	}

	// Get player's NFTs
	nadmons, err := h.repo.GetPlayerNadmons(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch NFTs: " + err.Error()})
		return
	}

	// Convert to frontend format
	nfts := make([]map[string]interface{}, len(nadmons))
	for i, nadmon := range nadmons {
		nfts[i] = nadmon.ToFrontendFormat()
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  nfts,
		"total": len(nfts),
	})
}

// SearchNFTs searches NFTs with filters
func (h *NadmonHandler) SearchNFTs(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Address parameter required"})
		return
	}

	// Validate Ethereum address format
	if !isValidEthereumAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Ethereum address format"})
		return
	}

	// Parse search parameters
	var search SearchQuery
	if err := c.ShouldBindQuery(&search); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid search parameters"})
		return
	}

	// Build filters map
	filters := make(map[string]interface{})
	if search.Element != "" {
		filters["element"] = search.Element
	}
	if search.Rarity != "" {
		filters["rarity"] = search.Rarity
	}
	if search.Type != "" {
		filters["type"] = search.Type
	}
	if search.Evo > 0 {
		filters["evo"] = search.Evo
	}

	// Search NFTs
	nadmons, err := h.repo.SearchNadmons(address, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search NFTs: " + err.Error()})
		return
	}

	// Convert to frontend format
	nfts := make([]map[string]interface{}, len(nadmons))
	for i, nadmon := range nadmons {
		nfts[i] = nadmon.ToFrontendFormat()
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  nfts,
		"total": len(nfts),
	})
}

// GetNFT returns a single NFT by token ID with current stats and evolution history
func (h *NadmonHandler) GetNFT(c *gin.Context) {
	tokenIDStr := c.Param("tokenId")
	tokenID, err := strconv.ParseInt(tokenIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID"})
		return
	}

	// Get NFT details
	nadmon, err := h.repo.GetSingleNadmon(tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch NFT: " + err.Error()})
		return
	}

	if nadmon == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "NFT not found"})
		return
	}

	// Get evolution history for this NFT
	history, err := h.repo.GetNadmonHistory(tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch NFT history: " + err.Error()})
		return
	}

	response := gin.H{
		"nft":     nadmon.ToFrontendFormat(),
		"history": history,
	}

	c.JSON(http.StatusOK, response)
}

// GetPackDetails returns detailed information about a specific pack including all NFTs
func (h *NadmonHandler) GetPackDetails(c *gin.Context) {
	packIDStr := c.Param("packId")
	packID, err := strconv.ParseInt(packIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pack ID"})
		return
	}

	// Get pack information
	pack, err := h.repo.GetPackByID(packID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pack: " + err.Error()})
		return
	}

	if pack == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pack not found"})
		return
	}

	// Get all NFTs in this pack
	nadmons, err := h.repo.GetNadmonsByIDs(pack.TokenIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pack NFTs: " + err.Error()})
		return
	}

	// Convert to frontend format
	nfts := make([]map[string]interface{}, len(nadmons))
	for i, nadmon := range nadmons {
		nfts[i] = nadmon.ToFrontendFormat()
	}

	response := gin.H{
		"pack_id":       pack.PackID,
		"player":        pack.Player,
		"payment_type":  pack.PaymentType,
		"purchased_at":  pack.PurchasedAt,
		"token_ids":     pack.TokenIDs,
		"nfts":          nfts,
		"total_nfts":    len(nfts),
	}

	c.JSON(http.StatusOK, response)
}

// GetNFTsByIDs returns multiple NFTs by their token IDs (for batch fetching)
func (h *NadmonHandler) GetNFTsByIDs(c *gin.Context) {
	// Parse token IDs from query parameter
	tokenIDsStr := c.Query("ids")
	if tokenIDsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token IDs parameter required"})
		return
	}

	// Split and parse token IDs
	idStrings := strings.Split(tokenIDsStr, ",")
	tokenIDs := make([]int64, 0, len(idStrings))
	
	for _, idStr := range idStrings {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID: " + idStr})
			return
		}
		tokenIDs = append(tokenIDs, id)
	}

	// Limit to prevent abuse
	if len(tokenIDs) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many token IDs (max 50)"})
		return
	}

	// Get NFTs
	nadmons, err := h.repo.GetNadmonsByIDs(tokenIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch NFTs: " + err.Error()})
		return
	}

	// Convert to frontend format
	nfts := make([]map[string]interface{}, len(nadmons))
	for i, nadmon := range nadmons {
		nfts[i] = nadmon.ToFrontendFormat()
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  nfts,
		"total": len(nfts),
	})
}

// GetPlayerProfile returns complete player profile
func (h *NadmonHandler) GetPlayerProfile(c *gin.Context) {
	address := c.Param("address")
	if !isValidEthereumAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Ethereum address"})
		return
	}

	profile, err := h.repo.GetPlayerProfile(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player profile: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// GetPlayerPacks returns player's pack purchase history
func (h *NadmonHandler) GetPlayerPacks(c *gin.Context) {
	address := c.Param("address")
	if !isValidEthereumAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Ethereum address"})
		return
	}

	packs, err := h.repo.GetPlayerPacks(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player packs: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  packs,
		"total": len(packs),
	})
}

// GetStats returns player statistics
func (h *NadmonHandler) GetStats(c *gin.Context) {
	address := c.Param("address")
	if !isValidEthereumAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Ethereum address"})
		return
	}

	// Get player profile which includes stats
	profile, err := h.repo.GetPlayerProfile(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player stats: " + err.Error()})
		return
	}

	// Calculate additional statistics
	stats := gin.H{
		"address":      profile.Address,
		"totalNFTs":    profile.TotalNFTs,
		"packsBought":  profile.PacksBought,
		"lastActivity": profile.LastActive,
	}

	// Add rarity and element breakdown
	if len(profile.Nadmons) > 0 {
		rarityStats := make(map[string]int)
		elementStats := make(map[string]int)
		evolvedCount := 0

		for _, nadmon := range profile.Nadmons {
			rarityStats[nadmon.Rarity]++
			elementStats[nadmon.Element]++
			if nadmon.Evo > 1 {
				evolvedCount++
			}
		}

		stats["rarityStats"] = rarityStats
		stats["elementStats"] = elementStats
		stats["evolvedNFTs"] = evolvedCount
	}

	c.JSON(http.StatusOK, stats)
}

// GetRecentPacks returns recent pack purchases across all players
func (h *NadmonHandler) GetRecentPacks(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	packs, err := h.repo.GetRecentPacks(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent packs: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  packs,
		"total": len(packs),
	})
}

// GetLeaderboard returns top collectors
func (h *NadmonHandler) GetLeaderboard(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	collectors, err := h.repo.GetTopCollectors(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  collectors,
		"total": len(collectors),
	})
}

// GetGameStats returns overall game statistics
func (h *NadmonHandler) GetGameStats(c *gin.Context) {
	stats, err := h.repo.GetGameStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch game stats: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Helper functions

// isValidEthereumAddress validates Ethereum address format
func isValidEthereumAddress(address string) bool {
	return len(address) == 42 && strings.HasPrefix(address, "0x")
}