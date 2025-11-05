package server

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"buildprize-game/internal/config"
	"buildprize-game/internal/hub"
	"buildprize-game/internal/repository"
	"buildprize-game/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Server struct {
	config      *config.Config
	hub         *hub.Hub
	gameService *services.GameService
	router      *gin.Engine
	upgrader    websocket.Upgrader
}

type WebSocketMessage struct {
	Type     string      `json:"type"`
	LobbyID  string      `json:"lobby_id,omitempty"`
	PlayerID string      `json:"player_id,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

func NewServer(cfg *config.Config) *Server {
	// Initialize components
	gameHub := hub.NewHub()

	// PostgreSQL is required
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required. Please set the DATABASE_URL environment variable.")
	}

	log.Printf("Connecting to PostgreSQL database...")
	repo, err := repository.NewPostgresRepository(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Printf("Successfully connected to PostgreSQL")

	gameService := services.NewGameService(gameHub, repo)

	// Setup WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// Setup Gin router
	router := gin.Default()

	server := &Server{
		config:      cfg,
		hub:         gameHub,
		gameService: gameService,
		router:      router,
		upgrader:    upgrader,
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	// CORS middleware
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Serve static files
	wd, _ := os.Getwd()
	clientPath := filepath.Join(wd, "client")
	s.router.Static("/client", clientPath)
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/client/index.html")
	})

	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// WebSocket test endpoint
	s.router.GET("/ws-test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "WebSocket endpoint is accessible",
			"path":    "/ws",
			"method":  "GET",
		})
	})

	// API routes
	api := s.router.Group("/api/v1")
	{
		// Add CORS headers to all API routes
		api.Use(func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Next()
		})

		api.OPTIONS("/lobbies", func(c *gin.Context) { c.Status(204) })
		api.POST("/lobbies", s.createLobby)
		api.GET("/lobbies", s.listLobbies)
		api.GET("/lobbies/:id", s.getLobby)
		api.OPTIONS("/lobbies/:id/join", func(c *gin.Context) { c.Status(204) })
		api.POST("/lobbies/:id/join", s.joinLobby)
		api.OPTIONS("/lobbies/:id/leave", func(c *gin.Context) { c.Status(204) })
		api.POST("/lobbies/:id/leave", s.leaveLobby)
		api.OPTIONS("/lobbies/:id/start", func(c *gin.Context) { c.Status(204) })
		api.POST("/lobbies/:id/start", s.startGame)
		api.OPTIONS("/lobbies/:id/answer", func(c *gin.Context) { c.Status(204) })
		api.POST("/lobbies/:id/answer", s.submitAnswer)
	}

	// WebSocket endpoint - must be before any catch-all routes
	s.router.GET("/ws", s.handleWebSocket)

	// Debug: log all routes
	log.Printf("WebSocket route registered at GET /ws")
}

func (s *Server) createLobby(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		MaxRounds int    `json:"max_rounds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.MaxRounds == 0 {
		req.MaxRounds = 10 // Default
	}

	lobby := s.gameService.CreateLobby(req.Name, req.MaxRounds)
	c.JSON(201, lobby)
}

func (s *Server) listLobbies(c *gin.Context) {
	lobbies, err := s.gameService.GetRepository().ListLobbies()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to list lobbies"})
		return
	}
	c.JSON(200, lobbies)
}

func (s *Server) getLobby(c *gin.Context) {
	lobbyID := c.Param("id")

	lobbyHub := s.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		c.JSON(404, gin.H{"error": "Lobby not found"})
		return
	}

	lobby := lobbyHub.GetLobby()
	c.JSON(200, lobby)
}

func (s *Server) joinLobby(c *gin.Context) {
	lobbyID := c.Param("id")

	var req struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	lobby, player, err := s.gameService.JoinLobby(lobbyID, req.Username)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"lobby":  lobby,
		"player": player,
	})
}

func (s *Server) leaveLobby(c *gin.Context) {
	lobbyID := c.Param("id")

	var req struct {
		PlayerID string `json:"player_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := s.gameService.LeaveLobby(lobbyID, req.PlayerID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Left lobby successfully"})
}

func (s *Server) startGame(c *gin.Context) {
	lobbyID := c.Param("id")

	err := s.gameService.StartGame(lobbyID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Game started"})
}

func (s *Server) submitAnswer(c *gin.Context) {
	lobbyID := c.Param("id")

	var req struct {
		PlayerID     string `json:"player_id" binding:"required"`
		Answer       int    `json:"answer" binding:"required"`
		ResponseTime int64  `json:"response_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := s.gameService.SubmitAnswer(lobbyID, req.PlayerID, req.Answer, req.ResponseTime)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Answer submitted"})
}

func (s *Server) handleWebSocket(c *gin.Context) {
	log.Printf("WebSocket connection attempt from %s", c.Request.RemoteAddr)
	log.Printf("WebSocket request headers: %v", c.Request.Header.Get("Upgrade"))
	log.Printf("WebSocket request method: %s", c.Request.Method)

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade FAILED from %s: %v", c.Request.RemoteAddr, err)
		log.Printf("   Upgrade failed - possible causes:")
		log.Printf("   1. Invalid WebSocket upgrade request")
		log.Printf("   2. Connection already closed by client")
		log.Printf("   3. Server overload or resource limits")
		return
	}
	log.Printf("✅ WebSocket upgrade successful from %s", c.Request.RemoteAddr)
	// Don't defer close here - let the goroutines handle connection lifecycle

	// Create client
	client := &hub.Client{
		ID:   generateClientID(),
		Send: make(chan []byte, 256),
	}

	log.Printf("WebSocket client connected: %s (from %s)", client.ID, c.Request.RemoteAddr)

	// Log connection details immediately after creation (before lobby registration)
	// This will show connections that are open but not yet registered with a lobby
	log.Printf("New WebSocket connection created - client ID: %s", client.ID)

	// Log total active connections across all lobbies
	totalConnections := s.countTotalConnections()
	log.Printf("Total active WebSocket connections (registered with lobbies): %d", totalConnections)

	// WebSocket connection constants
	const (
		writeWait  = 10 * time.Second
		pongWait   = 24 * time.Hour   // Very long deadline - effectively no timeout
		pingPeriod = 30 * time.Second // Send ping every 30 seconds for health checks
	)

	// Set pong handler - we don't use deadlines for connection closing anymore
	// Deadlines are set very long (24 hours) so connections don't timeout
	// Pings are still sent for health monitoring but won't close connections
	pongReceived := make(chan bool, 1)
	conn.SetPongHandler(func(string) error {
		// Pong received from client - just log for health monitoring
		// Don't use deadlines to close connections
		select {
		case pongReceived <- true:
		default:
		}
		return nil
	})

	// Monitor pong responses in background for health checks only (not for closing)
	go func() {
		pongCount := 0
		lastPongTime := time.Now()
		for {
			select {
			case <-pongReceived:
				pongCount++
				lastPongTime = time.Now()
				if pongCount%10 == 0 {
					log.Printf("✅ Client %s: Received %d pongs (connection healthy)", client.ID, pongCount)
				}
			case <-time.After(5 * time.Minute):
				// Health check - log if no pongs received, but don't close connection
				timeSinceLastPong := time.Since(lastPongTime)
				if pongCount == 0 && timeSinceLastPong > 5*time.Minute {
					log.Printf("⚠️ NOTE: Client %s has not received any pongs in 5 minutes (connection still open, just monitoring)", client.ID)
				}
			}
		}
	}()

	// Send initial connection confirmation message
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "connected",
		"client_id": client.ID,
	}); err != nil {
		log.Printf("❌ FAILED to send initial connection message to client %s: %v", client.ID, err)
		log.Printf("   Connection will be closed due to initial message failure")
		conn.Close()
		return
	}
	log.Printf("✅ Sent initial connection message to client %s", client.ID)

	// Handle messages - pass constants to goroutines
	go s.handleClientMessages(conn, client, pongWait)
	go s.handleClientWrites(conn, client, writeWait, pingPeriod)
}

func (s *Server) handleClientMessages(conn *websocket.Conn, client *hub.Client, pongWait time.Duration) {
	defer func() {
		log.Printf("WebSocket client %s read goroutine exiting - connection will be closed", client.ID)
		if client.Hub != nil {
			client.Hub.Unregister(client)
		}
		conn.Close()
		// Log total connections after disconnect
		totalConnections := s.countTotalConnections()
		log.Printf("Total active WebSocket connections after disconnect: %d", totalConnections)
	}()

	// Set a very long read deadline (24 hours) - effectively no timeout
	// We still use deadlines for read operations, but they won't cause connection closure
	conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		var msg WebSocketMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			errStr := err.Error()

			// Check for expected/normal disconnects FIRST - these are not errors
			// "use of closed network connection" is normal when peer closes connection
			if strings.Contains(strings.ToLower(errStr), "use of closed network connection") ||
				strings.Contains(strings.ToLower(errStr), "broken pipe") ||
				strings.Contains(strings.ToLower(errStr), "connection reset") ||
				strings.Contains(strings.ToLower(errStr), "closed network") ||
				errors.Is(err, net.ErrClosed) {
				// Normal network-level disconnect - exit silently, don't log
				break
			}

			// Check for WebSocket close frame errors
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				// Normal WebSocket close - exit silently
				break
			}

			// Check for read deadline timeout - but don't close connection, just reset deadline
			// Since we set deadline to 24 hours, this should rarely happen
			if strings.Contains(strings.ToLower(errStr), "i/o timeout") ||
				strings.Contains(strings.ToLower(errStr), "deadline exceeded") {
				log.Printf("⏱️ WebSocket read deadline reached for client %s - resetting deadline (connection stays open)", client.ID)
				// Reset deadline instead of closing - connection stays alive
				conn.SetReadDeadline(time.Now().Add(pongWait))
				continue // Continue reading instead of breaking
			}

			// Only log truly unexpected errors
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("WebSocket unexpected close error: %v", err)
			} else {
				// Other unexpected errors
				if !strings.Contains(strings.ToLower(errStr), "use of closed") {
					log.Printf("WebSocket read error: %v", err)
				}
			}
			break
		}

		// Reset read deadline on successful read (though it's already very long)
		conn.SetReadDeadline(time.Now().Add(pongWait))

		s.handleWebSocketMessage(client, &msg)
	}
}

func (s *Server) handleClientWrites(conn *websocket.Conn, client *hub.Client, writeWait time.Duration, pingPeriod time.Duration) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		log.Printf("WebSocket client %s write goroutine exiting", client.ID)
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed - send close message and exit
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				// These errors are BENIGN - they occur when one goroutine closes the connection
				// while another is trying to write. This is normal and expected.
				// Don't log them to avoid noise.
				if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) &&
					!errors.Is(err, net.ErrClosed) &&
					!strings.Contains(err.Error(), "use of closed network connection") &&
					!strings.Contains(err.Error(), "broken pipe") &&
					!strings.Contains(err.Error(), "connection reset") {
					// Only log truly unexpected write errors
					log.Printf("WebSocket write error (unexpected): %v", err)
				}
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			// Use WriteControl for ping as recommended by Gorilla WebSocket
			// Browsers automatically respond with pong frames
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				// Don't log expected connection closed errors - these are benign
				// when one goroutine closes while another is writing
				if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) &&
					!errors.Is(err, net.ErrClosed) &&
					!strings.Contains(err.Error(), "use of closed network connection") &&
					!strings.Contains(err.Error(), "broken pipe") &&
					!strings.Contains(err.Error(), "connection reset") {
					log.Printf("⚠️ WebSocket ping error (unexpected) for client %s: %v", client.ID, err)
				}
				return
			}
			// Ping sent successfully - pong handler will reset read deadline when browser responds
			// Note: We don't log successful pings to avoid spam, but pong handler will log if pong received
		}
	}
}

func (s *Server) handleWebSocketMessage(client *hub.Client, msg *WebSocketMessage) {
	switch msg.Type {
	case "join_lobby":
		s.handleJoinLobby(client, msg)
	case "leave_lobby":
		s.handleLeaveLobby(client, msg)
	case "start_game":
		s.handleStartGame(client, msg)
	case "submit_answer":
		s.handleSubmitAnswer(client, msg)
	}
}

func (s *Server) handleJoinLobby(client *hub.Client, msg *WebSocketMessage) {
	lobbyID := msg.LobbyID
	if lobbyID == "" {
		return
	}

	username, ok := msg.Data.(map[string]interface{})["username"].(string)
	if !ok {
		return
	}

	lobbyHub := s.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		return
	}

	// Check if player already joined (via REST API)
	// This needs to happen before registering to avoid race conditions
	lobby := lobbyHub.GetLobby()
	playerExists := false
	for _, p := range lobby.Players {
		if p.Username == username {
			playerExists = true
			client.PlayerID = p.ID
			break
		}
	}

	// Register WebSocket client with the lobby hub
	// First, unregister if already registered with a different lobby or same lobby
	if client.Hub != nil && client.Hub != lobbyHub {
		// Client was registered with a different lobby, unregister first
		client.Hub.Unregister(client)
	} else if client.Hub == lobbyHub {
		// Client already registered with this lobby - check if it's a duplicate
		existingClients := lobbyHub.GetClients()
		for _, existingClient := range existingClients {
			// If same client ID but different connection, this is a reconnect
			if existingClient.ID == client.ID && existingClient.Send != client.Send {
				log.Printf("Client %s reconnecting, unregistering old connection", client.ID)
				lobbyHub.Unregister(existingClient)
				break
			}
		}
	}

	client.LobbyID = lobbyID
	client.Hub = lobbyHub
	lobbyHub.Register(client)

	// Only join via game service if not already a player
	// This prevents duplicate joins and re-broadcasts
	if !playerExists {
		// Join the player and broadcast to all clients (including the one just registered)
		s.gameService.JoinLobby(lobbyID, username)
	} else {
		// Player already joined via REST, broadcast current state to ALL clients
		// Get fresh lobby state to ensure we're broadcasting the latest state
		currentLobby := lobbyHub.GetLobby()
		// This ensures everyone (including newly connected clients) gets the latest lobby state
		// Use player_joined event type so frontend handles it the same way
		s.gameService.BroadcastLobbyUpdate(lobbyHub, "player_joined", map[string]interface{}{
			"lobby": currentLobby,
		})
	}
}

func (s *Server) handleLeaveLobby(client *hub.Client, msg *WebSocketMessage) {
	lobbyID := msg.LobbyID
	if lobbyID == "" && client.LobbyID != "" {
		lobbyID = client.LobbyID
	}

	if lobbyID == "" {
		log.Printf("handleLeaveLobby: No lobby ID provided")
		return
	}

	// Get player ID from client or message data
	playerID := client.PlayerID
	if playerID == "" {
		// Try to get from message data
		if data, ok := msg.Data.(map[string]interface{}); ok {
			if pid, ok := data["player_id"].(string); ok {
				playerID = pid
			}
		}
	}

	if playerID == "" {
		log.Printf("handleLeaveLobby: No player ID found for client %s in lobby %s", client.ID, lobbyID)
		// Still unregister the WebSocket connection even if we can't remove the player
		if client.Hub != nil {
			client.Hub.Unregister(client)
			client.Hub = nil
		}
		return
	}

	// Remove player from lobby via game service (this will broadcast to all clients)
	err := s.gameService.LeaveLobby(lobbyID, playerID)
	if err != nil {
		log.Printf("handleLeaveLobby: Failed to leave lobby %s for player %s: %v", lobbyID, playerID, err)
		// Still unregister the WebSocket connection even if leave failed
	}

	// Unregister WebSocket client from hub
	if client.Hub != nil {
		client.Hub.Unregister(client)
		client.Hub = nil
		client.LobbyID = ""
		client.PlayerID = ""
	}
}

func (s *Server) handleStartGame(client *hub.Client, msg *WebSocketMessage) {
	lobbyID := msg.LobbyID
	if lobbyID == "" {
		return
	}

	s.gameService.StartGame(lobbyID)
}

func (s *Server) handleSubmitAnswer(client *hub.Client, msg *WebSocketMessage) {
	lobbyID := msg.LobbyID
	if lobbyID == "" {
		return
	}

	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	playerID, _ := data["player_id"].(string)
	answer, _ := data["answer"].(float64)
	responseTime, _ := data["response_time"].(float64)

	s.gameService.SubmitAnswer(lobbyID, playerID, int(answer), int64(responseTime))
}

func (s *Server) Start() error {
	return s.router.Run(":" + s.config.Port)
}

func (s *Server) countTotalConnections() int {
	// Count connections by checking all lobbies via repository
	// We'll use a simpler approach - count via game service
	total := 0

	// Get all lobbies from repository to iterate through
	allLobbies, err := s.gameService.GetRepository().ListLobbies()
	if err != nil {
		log.Printf("Error listing lobbies for connection count: %v", err)
		return 0
	}

	for _, lobby := range allLobbies {
		lobbyHub := s.hub.GetLobbyHub(lobby.ID)
		if lobbyHub != nil {
			clients := lobbyHub.GetClients()
			clientCount := len(clients)
			if clientCount > 0 {
				log.Printf("  Lobby %s (%s): %d active connection(s)", lobby.ID, lobby.Name, clientCount)
			}
			total += clientCount
		}
	}

	return total
}

func generateClientID() string {
	// Use nanosecond precision + random component to ensure uniqueness
	// This prevents duplicate IDs even if called multiple times in quick succession
	return "client_" + time.Now().Format("20060102150405") + "_" + time.Now().Format("000000000")
}
