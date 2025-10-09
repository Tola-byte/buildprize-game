package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	// Use PostgreSQL if DATABASE_URL is provided, otherwise use in-memory
	var repo repository.Repository
	var err error

	if cfg.DatabaseURL != "" {
		log.Printf("Using PostgreSQL database")
		repo, err = repository.NewPostgresRepository(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}
	} else {
		log.Printf("Using in-memory database (development mode)")
		repo = repository.NewInMemoryRepository()
	}

	gameService := services.NewGameService(gameHub, repo)

	// Setup WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
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

	// WebSocket endpoint
	s.router.GET("/ws", s.handleWebSocket)
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
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Create client
	client := &hub.Client{
		ID:   generateClientID(),
		Send: make(chan []byte, 256),
	}

	// Handle messages
	go s.handleClientMessages(conn, client)
	go s.handleClientWrites(conn, client)
}

func (s *Server) handleClientMessages(conn *websocket.Conn, client *hub.Client) {
	defer func() {
		if client.Hub != nil {
			client.Hub.Unregister(client)
		}
		conn.Close()
	}()

	for {
		var msg WebSocketMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		s.handleWebSocketMessage(client, &msg)
	}
}

func (s *Server) handleClientWrites(conn *websocket.Conn, client *hub.Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
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

	client.LobbyID = lobbyID
	client.Hub = lobbyHub
	lobbyHub.Register(client)

	// Join the lobby through game service
	s.gameService.JoinLobby(lobbyID, username)
}

func (s *Server) handleLeaveLobby(client *hub.Client, msg *WebSocketMessage) {
	if client.Hub != nil {
		client.Hub.Unregister(client)
		client.Hub = nil
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

func generateClientID() string {
	return "client_" + time.Now().Format("20060102150405")
}
