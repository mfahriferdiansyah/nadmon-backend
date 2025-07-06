package handlers

import (
	"net/http"
	"strings"

	"nadmon-backend/internal/websocket"

	"github.com/gin-gonic/gin"
)

type WebSocketHandler struct {
	wsManager *websocket.Manager
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(wsManager *websocket.Manager) *WebSocketHandler {
	return &WebSocketHandler{
		wsManager: wsManager,
	}
}

// HandleConnection handles WebSocket connection requests
func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	address := c.Param("address")
	
	// Validate Ethereum address
	if !isValidEthereumAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Ethereum address"})
		return
	}

	// Normalize address to lowercase
	address = strings.ToLower(address)

	// Upgrade HTTP connection to WebSocket
	h.wsManager.UpgradeConnection(c.Writer, c.Request, address)
}

// GetConnectedUsers returns currently connected users (for debugging/admin)
func (h *WebSocketHandler) GetConnectedUsers(c *gin.Context) {
	stats := h.wsManager.GetStats()
	c.JSON(http.StatusOK, stats)
}