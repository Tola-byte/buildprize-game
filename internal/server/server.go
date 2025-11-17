package server

import (
	"encoding/json"
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
	"buildprize-game/internal/models"
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
	gameHub := hub.NewHub()

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

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

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

	wd, _ := os.Getwd()
	clientPath := filepath.Join(wd, "client")
	s.router.Static("/client", clientPath)
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/client/index.html")
	})

	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	s.router.GET("/ws-test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "WebSocket endpoint is accessible",
			"path":    "/ws",
			"method":  "GET",
		})
	})

	api := s.router.Group("/api/v1")
	{
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
		api.OPTIONS("/lobbies/:id/chat", func(c *gin.Context) { c.Status(204) })
		api.POST("/lobbies/:id/chat", s.sendChatMessage)
	}

	s.router.GET("/ws", s.handleWebSocket)
	log.Printf("WebSocket route registered at GET /ws")
	log.Printf("Chat route registered at POST /api/v1/lobbies/:id/chat")
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
		req.MaxRounds = 10
	}

	lobby := s.gameService.CreateLobby(req.Name, req.MaxRounds)
	c.JSON(201, lobby)
}

func (s *Server) listLobbies(c *gin.Context) {
	lobbies, err := s.gameService.GetRepository().ListLobbies()
	if err != nil {
		log.Printf("Error listing lobbies: %v", err)
		c.JSON(500, gin.H{"error": "Failed to list lobbies"})
		return
	}
	// Ensure we always return an array, not null
	if lobbies == nil {
		lobbies = []*models.Lobby{}
	}
	log.Printf("ListLobbies: Returning %d waiting lobbies", len(lobbies))
	for _, lobby := range lobbies {
		log.Printf("  - Lobby: %s (ID: %s, State: %s, Players: %d)", lobby.Name, lobby.ID, lobby.State, len(lobby.Players))
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

func (s *Server) sendChatMessage(c *gin.Context) {
	lobbyID := c.Param("id")

	var req struct {
		PlayerID string `json:"player_id" binding:"required"`
		Message  string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get lobby hub
	lobbyHub := s.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		c.JSON(404, gin.H{"error": "Lobby not found"})
		return
	}

	// Get player username
	lobby := lobbyHub.GetLobby()
	player := lobby.GetPlayer(req.PlayerID)
	if player == nil {
		c.JSON(404, gin.H{"error": "Player not found in lobby"})
		return
	}

	// Broadcast chat message to all clients in the lobby
	log.Printf("REST API: Broadcasting chat message from player %s (%s) in lobby %s: %s", req.PlayerID, player.Username, lobbyID, req.Message)
	clients := lobbyHub.GetClients()
	log.Printf("Lobby %s has %d clients to receive the message", lobbyID, len(clients))

	// Log each client that will receive the message
	for clientID, client := range clients {
		log.Printf("  Client %s (player: %s) will receive message", clientID, client.PlayerID)
	}

	s.gameService.BroadcastLobbyUpdate(lobbyHub, "chat_message", map[string]interface{}{
		"player_id": req.PlayerID,
		"username":  player.Username,
		"message":   req.Message,
		"timestamp": time.Now().UnixMilli(),
	})

	log.Printf("REST API: Chat message broadcast completed for lobby %s", lobbyID)

	c.JSON(200, gin.H{"message": "Chat message sent"})
}

func (s *Server) handleWebSocket(c *gin.Context) {
	log.Printf("WebSocket connection attempt from %s", c.Request.RemoteAddr)
	log.Printf("WebSocket request headers: %v", c.Request.Header.Get("Upgrade"))
	log.Printf("WebSocket request method: %s", c.Request.Method)

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade FAILED from %s: %v", c.Request.RemoteAddr, err)
		return
	}
	log.Printf("WebSocket upgrade successful from %s", c.Request.RemoteAddr)
	client := &hub.Client{
		ID:   generateClientID(),
		Send: make(chan []byte, 256),
	}

	log.Printf("WebSocket client connected: %s (from %s)", client.ID, c.Request.RemoteAddr)
	log.Printf("New WebSocket connection created - client ID: %s", client.ID)

	totalConnections := s.countTotalConnections()
	log.Printf("Total active WebSocket connections (registered with lobbies): %d", totalConnections)

	const (
		writeWait  = 10 * time.Second
		pongWait   = 24 * time.Hour
		pingPeriod = 30 * time.Second
	)

	pongReceived := make(chan bool, 1)
	conn.SetPongHandler(func(string) error {
		select {
		case pongReceived <- true:
		default:
		}
		return nil
	})

	go func() {
		pongCount := 0
		lastPongTime := time.Now()
		for {
			select {
			case <-pongReceived:
				pongCount++
				lastPongTime = time.Now()
				if pongCount%10 == 0 {
					log.Printf("Client %s: Received %d pongs (connection healthy)", client.ID, pongCount)
				}
			case <-time.After(5 * time.Minute):
				timeSinceLastPong := time.Since(lastPongTime)
				if pongCount == 0 && timeSinceLastPong > 5*time.Minute {
					log.Printf("NOTE: Client %s has not received any pongs in 5 minutes (connection still open, just monitoring)", client.ID)
				}
			}
		}
	}()

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "connected",
		"client_id": client.ID,
	}); err != nil {
		log.Printf("FAILED to send initial connection message to client %s: %v", client.ID, err)
		conn.Close()
		return
	}
	log.Printf("Sent initial connection message to client %s", client.ID)

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
		totalConnections := s.countTotalConnections()
		log.Printf("Total active WebSocket connections after disconnect: %d", totalConnections)
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		var msg WebSocketMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			errStr := err.Error()

			if strings.Contains(strings.ToLower(errStr), "use of closed network connection") ||
				strings.Contains(strings.ToLower(errStr), "broken pipe") ||
				strings.Contains(strings.ToLower(errStr), "connection reset") ||
				strings.Contains(strings.ToLower(errStr), "closed network") ||
				errors.Is(err, net.ErrClosed) {
				break
			}

			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				break
			}

			if strings.Contains(strings.ToLower(errStr), "i/o timeout") ||
				strings.Contains(strings.ToLower(errStr), "deadline exceeded") {
				log.Printf("WebSocket read deadline reached for client %s - resetting deadline (connection stays open)", client.ID)
				conn.SetReadDeadline(time.Now().Add(pongWait))
				continue
			}
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("WebSocket unexpected close error: %v", err)
			} else {
				if !strings.Contains(strings.ToLower(errStr), "use of closed") {
					log.Printf("WebSocket read error: %v", err)
				}
			}
			break
		}

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
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Log message being sent (for debugging chat messages)
			var msgData map[string]interface{}
			if err := json.Unmarshal(message, &msgData); err == nil {
				if msgType, ok := msgData["type"].(string); ok && msgType == "chat_message" {
					log.Printf("Writing chat_message to client %s (player: %s)", client.ID, client.PlayerID)
				}
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) &&
					!errors.Is(err, net.ErrClosed) &&
					!strings.Contains(err.Error(), "use of closed network connection") &&
					!strings.Contains(err.Error(), "broken pipe") &&
					!strings.Contains(err.Error(), "connection reset") {
					log.Printf("WebSocket write error (unexpected): %v", err)
				}
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) &&
					!errors.Is(err, net.ErrClosed) &&
					!strings.Contains(err.Error(), "use of closed network connection") &&
					!strings.Contains(err.Error(), "broken pipe") &&
					!strings.Contains(err.Error(), "connection reset") {
					log.Printf("WebSocket ping error (unexpected) for client %s: %v", client.ID, err)
				}
				return
			}
		}
	}
}

func (s *Server) handleWebSocketMessage(client *hub.Client, msg *WebSocketMessage) {
	log.Printf("handleWebSocketMessage: Received message type=%s from client=%s", msg.Type, client.ID)
	switch msg.Type {
	case "join_lobby":
		s.handleJoinLobby(client, msg)
	case "leave_lobby":
		s.handleLeaveLobby(client, msg)
	case "start_game":
		s.handleStartGame(client, msg)
	case "submit_answer":
		s.handleSubmitAnswer(client, msg)
	case "chat_message":
		s.handleChatMessage(client, msg)
	default:
		log.Printf("handleWebSocketMessage: Unknown message type: %s", msg.Type)
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

	lobby := lobbyHub.GetLobby()
	playerExists := false
	for _, p := range lobby.Players {
		if p.Username == username {
			playerExists = true
			client.PlayerID = p.ID
			break
		}
	}

	if client.Hub != nil && client.Hub != lobbyHub {
		client.Hub.Unregister(client)
	} else if client.Hub == lobbyHub {
		existingClients := lobbyHub.GetClients()
		for _, existingClient := range existingClients {
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

	if !playerExists {
		// Join the player and broadcast to all clients (including the one just registered)
		_, newPlayer, err := s.gameService.JoinLobby(lobbyID, username)
		if err == nil && newPlayer != nil {
			// Set the client's PlayerID from the newly created player
			client.PlayerID = newPlayer.ID
			log.Printf("handleJoinLobby: Set client.PlayerID to %s for newly joined player %s", newPlayer.ID, username)
		} else if err != nil {
			log.Printf("handleJoinLobby: Failed to join lobby %s for player %s: %v", lobbyID, username, err)
		}
	} else {
		currentLobby := lobbyHub.GetLobby()
		s.gameService.BroadcastLobbyUpdate(lobbyHub, "player_joined", map[string]interface{}{
			"lobby": currentLobby,
		})
	}

	
	currentLobby := lobbyHub.GetLobby()
	if currentLobby.State == models.InProgress && currentLobby.IsQuestionActive() && currentLobby.CurrentQ != nil {
		
		questionEndTimestamp := currentLobby.QuestionEnd.UnixMilli()
		currentServerTime := time.Now().UnixMilli()
		remainingSeconds := int(time.Until(*currentLobby.QuestionEnd).Seconds())
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}

		event := models.GameEvent{
			Type:    "new_question",
			LobbyID: currentLobby.ID,
			Data: map[string]interface{}{
				"question":          currentLobby.CurrentQ,
				"round":             currentLobby.Round,
				"time_left":         remainingSeconds,
				"question_end_time": questionEndTimestamp,
				"server_time":       currentServerTime,
			},
			Timestamp: time.Now(),
		}

		jsonData, err := json.Marshal(event)
		if err != nil {
			log.Printf("Error marshaling current question for client %s: %v", client.ID, err)
		} else {
		
			select {
			case client.Send <- jsonData:
				log.Printf("Sent current question to newly connected client %s (player: %s) in lobby %s", client.ID, client.PlayerID, lobbyID)
			default:
				log.Printf("Warning: Could not send current question to client %s (channel full)", client.ID)
			}
		}
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

	playerID := client.PlayerID
	if playerID == "" {
		
		if data, ok := msg.Data.(map[string]interface{}); ok {
			if pid, ok := data["player_id"].(string); ok {
				playerID = pid
			}
		}
	}

	if playerID == "" {
		log.Printf("handleLeaveLobby: No player ID found for client %s in lobby %s", client.ID, lobbyID)
		if client.Hub != nil {
			client.Hub.Unregister(client)
			client.Hub = nil
		}
		return
	}

	err := s.gameService.LeaveLobby(lobbyID, playerID)
	if err != nil {
		log.Printf("handleLeaveLobby: Failed to leave lobby %s for player %s: %v", lobbyID, playerID, err)
	}

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

func (s *Server) handleChatMessage(client *hub.Client, msg *WebSocketMessage) {
	log.Printf("handleChatMessage called: client=%s, msg.Type=%s, msg.LobbyID=%s, msg.PlayerID=%s, msg.Data=%v",
		client.ID, msg.Type, msg.LobbyID, msg.PlayerID, msg.Data)

	lobbyID := msg.LobbyID
	if lobbyID == "" && client.LobbyID != "" {
		lobbyID = client.LobbyID
	}

	if lobbyID == "" {
		log.Printf("handleChatMessage: No lobby ID provided")
		return
	}

	// Get message data
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		log.Printf("handleChatMessage: Invalid message data type, got %T, expected map[string]interface{}", msg.Data)
		return
	}
	log.Printf("handleChatMessage: Message data extracted: %v", data)

	messageText, ok := data["message"].(string)
	if !ok || messageText == "" {
		log.Printf("handleChatMessage: Invalid or empty message")
		return
	}

	// Get player info - try multiple sources
	playerID := client.PlayerID
	if playerID == "" {
		// Try from top-level message first (frontend sends it there)
		if msg.PlayerID != "" {
			playerID = msg.PlayerID
		}
	}
	if playerID == "" {
		// Try to get from message data as fallback
		if pid, ok := data["player_id"].(string); ok {
			playerID = pid
		}
	}

	if playerID == "" {
		log.Printf("handleChatMessage: No player ID found for client %s (client.PlayerID: %s, msg.PlayerID: %s, data: %v)",
			client.ID, client.PlayerID, msg.PlayerID, data)
		return
	}

	// Update client.PlayerID if we found it from the message (for future messages)
	if client.PlayerID == "" && playerID != "" {
		client.PlayerID = playerID
		log.Printf("handleChatMessage: Set client.PlayerID to %s for client %s", playerID, client.ID)
	}

	// Get lobby hub
	lobbyHub := s.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		log.Printf("handleChatMessage: Lobby %s not found", lobbyID)
		return
	}

	// Get player username
	lobby := lobbyHub.GetLobby()
	player := lobby.GetPlayer(playerID)
	if player == nil {
		log.Printf("handleChatMessage: Player %s not found in lobby %s", playerID, lobbyID)
		return
	}

	// Broadcast chat message to all clients in the lobby
	log.Printf("WebSocket: Broadcasting chat message from player %s (%s) in lobby %s: %s", playerID, player.Username, lobbyID, messageText)
	clients := lobbyHub.GetClients()
	log.Printf("Lobby %s has %d clients to receive the message", lobbyID, len(clients))

	// Log each client that will receive the message
	for clientID, client := range clients {
		log.Printf("  Client %s (player: %s) will receive message", clientID, client.PlayerID)
	}

	s.gameService.BroadcastLobbyUpdate(lobbyHub, "chat_message", map[string]interface{}{
		"player_id": playerID,
		"username":  player.Username,
		"message":   messageText,
		"timestamp": time.Now().UnixMilli(),
	})

	log.Printf("WebSocket: Chat message broadcast completed for lobby %s", lobbyID)
}

func (s *Server) Start() error {
	return s.router.Run(":" + s.config.Port)
}

func (s *Server) countTotalConnections() int {
	total := 0

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
	return "client_" + time.Now().Format("20060102150405") + "_" + time.Now().Format("000000000")
}
