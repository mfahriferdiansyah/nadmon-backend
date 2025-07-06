package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nadmon-backend/internal/config"
	"nadmon-backend/internal/database"
	"nadmon-backend/internal/handlers"
	"nadmon-backend/internal/repository"
	"nadmon-backend/internal/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Connect to Envio database
	envioDB, err := database.ConnectToEnvio(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to Envio database:", err)
	}
	defer envioDB.Close()

	// Test database connection
	if err := envioDB.TestConnection(); err != nil {
		log.Fatal("Failed to test database connection:", err)
	}

	// Create indexes for better performance
	if err := envioDB.CreateIndexes(); err != nil {
		log.Printf("Warning: Failed to create some indexes: %v", err)
	}

	// Initialize WebSocket manager for real-time updates
	wsManager := websocket.NewManager()
	go wsManager.Start()

	// Initialize repository layer
	nadmonRepo := repository.NewNadmonRepository(envioDB)

	// Initialize Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:3002", "http://localhost:3003"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize handlers
	nadmonHandler := handlers.NewNadmonHandler(nadmonRepo)
	wsHandler := handlers.NewWebSocketHandler(wsManager)

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		stats, err := envioDB.GetStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"database":  stats,
		})
	})

	// Database stats endpoint
	r.GET("/stats", nadmonHandler.GetGameStats)

	// API routes
	api := r.Group("/api")
	{
		// Player endpoints
		api.GET("/players/:address/nadmons", nadmonHandler.GetInventory)
		api.GET("/players/:address/profile", nadmonHandler.GetPlayerProfile)
		api.GET("/players/:address/packs", nadmonHandler.GetPlayerPacks)
		api.GET("/players/:address/stats", nadmonHandler.GetStats)
		api.GET("/players/:address/search", nadmonHandler.SearchNFTs)

		// NFT endpoints
		api.GET("/nfts/:tokenId", nadmonHandler.GetNFT)
		api.GET("/nfts/:tokenId/history", nadmonHandler.GetNFT) // Same endpoint, returns history
		api.GET("/nfts", nadmonHandler.GetNFTsByIDs) // Batch fetch NFTs by IDs
		
		// Pack endpoints
		api.GET("/packs/:packId", nadmonHandler.GetPackDetails)

		// Game data endpoints
		api.GET("/packs/recent", nadmonHandler.GetRecentPacks)
		api.GET("/leaderboard/collectors", nadmonHandler.GetLeaderboard)
		api.GET("/stats/game", nadmonHandler.GetGameStats)

		// Legacy endpoints for backward compatibility
		api.GET("/inventory/:address", nadmonHandler.GetInventory)
		api.GET("/inventory/:address/search", nadmonHandler.SearchNFTs)
		api.GET("/nft/:tokenId", nadmonHandler.GetNFT)
		api.GET("/stats/:address", nadmonHandler.GetStats)

		// WebSocket endpoint for real-time updates
		api.GET("/ws/:address", wsHandler.HandleConnection)
	}

	// Start server
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	log.Printf("🚀 Nadmon Backend started on port %s", port)
	log.Printf("📊 Health check: http://localhost:%s/health", port)
	log.Printf("🔌 WebSocket: ws://localhost:%s/api/ws/{address}", port)
	log.Printf("📋 API Documentation:")
	log.Printf("   GET /api/players/{address}/nadmons    - Get player's NFTs")
	log.Printf("   GET /api/players/{address}/profile    - Get player profile")
	log.Printf("   GET /api/players/{address}/packs      - Get player's pack history")
	log.Printf("   GET /api/players/{address}/stats      - Get player statistics")
	log.Printf("   GET /api/nfts/{tokenId}               - Get NFT details and history")
	log.Printf("   GET /api/packs/{packId}               - Get pack details with NFTs")
	log.Printf("   GET /api/nfts?ids=1,2,3               - Get multiple NFTs by IDs")
	log.Printf("   GET /api/packs/recent                 - Get recent pack purchases")
	log.Printf("   GET /api/leaderboard/collectors       - Get top collectors")
	log.Printf("   GET /api/stats/game                   - Get game statistics")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Shutdown server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("✅ Server exited")
}