package hub

import (
	"log"
	"sync"
	"time"

	"buildprize-game/internal/models"
)

type Hub struct {
	lobbies map[string]*LobbyHub
	mu      sync.RWMutex // read-write mutex , allows mutliple readers or one writer
}
type LobbyHub struct {
	lobby      *models.Lobby
	clients    map[string]*WebSocketClient
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	broadcast  chan []byte
	mu         sync.RWMutex
}

// WebSocketClient represents a player's WebSocket connection to a lobby.
// Each player connected to a lobby has one WebSocketClient instance.
type WebSocketClient struct {
	ID       string      // Unique connection ID (different from PlayerID)
	LobbyID  string      // Which lobby this connection belongs to
	PlayerID string      // Links to the actual Player in the lobby
	Send     chan []byte // Channel for sending messages to this player
	Hub      *LobbyHub   // The lobby hub managing this connection
}

// Client is an alias for WebSocketClient (for backward compatibility)
type Client = WebSocketClient

type LobbyHubInterface interface {
	Register(client *WebSocketClient)
	Unregister(client *WebSocketClient)
	Broadcast(data []byte)
	GetLobby() *models.Lobby
	GetClients() map[string]*WebSocketClient
}

func NewHub() *Hub {
	return &Hub{
		lobbies: make(map[string]*LobbyHub),
	}
}

func (h *Hub) GetLobbyHub(lobbyID string) *LobbyHub {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lobbies[lobbyID]
}

func (h *Hub) CreateLobbyHub(lobby *models.Lobby) *LobbyHub {
	h.mu.Lock()
	defer h.mu.Unlock()

	lobbyHub := &LobbyHub{
		lobby:      lobby,
		clients:    make(map[string]*WebSocketClient),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		broadcast:  make(chan []byte),
	}

	h.lobbies[lobby.ID] = lobbyHub
	go lobbyHub.run()

	return lobbyHub
}

func (h *Hub) RemoveLobbyHub(lobbyID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.lobbies, lobbyID)
}

func (lh *LobbyHub) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-lh.register:
			// Client already registered synchronously in Register(), just log
			log.Printf("Player connection %s (player: %s) registered with lobby %s", client.ID, client.PlayerID, lh.lobby.ID)

		case client := <-lh.unregister:
			lh.mu.Lock()
			wasRegistered := false
			if _, ok := lh.clients[client.ID]; ok {
				delete(lh.clients, client.ID)
				close(client.Send)
				wasRegistered = true
			}
			remainingConnections := len(lh.clients)
			lh.mu.Unlock()
			if wasRegistered {
				log.Printf("Player connection %s (player: %s) left lobby %s - %d connection(s) remaining", client.ID, client.PlayerID, lh.lobby.ID, remainingConnections)
			} else {
				log.Printf("Player connection %s was not registered in lobby %s (already removed?)", client.ID, lh.lobby.ID)
			}

		case message := <-lh.broadcast:
			lh.mu.RLock()
			// Collect clients that need to be removed
			var clientsToRemove []string
			for _, client := range lh.clients {
				select {
				case client.Send <- message:
				default:
					// Client's send channel is full, mark for removal
					clientsToRemove = append(clientsToRemove, client.ID)
				}
			}
			lh.mu.RUnlock()

			// Remove dead clients (requires write lock)
			if len(clientsToRemove) > 0 {
				lh.mu.Lock()
				for _, clientID := range clientsToRemove {
					if client, ok := lh.clients[clientID]; ok {
						close(client.Send)
						delete(lh.clients, clientID)
					}
				}
				lh.mu.Unlock()
			}

		case <-ticker.C:
			// Periodic health check (can be used for connection monitoring)
			// Question timeouts are handled by game service, not here
		}
	}
}

func (lh *LobbyHub) Register(client *WebSocketClient) {
	log.Printf("Registering player connection %s (player: %s) with lobby %s", client.ID, client.PlayerID, lh.lobby.ID)
	// Register synchronously to ensure client is in map before any broadcasts
	lh.mu.Lock()
	// Check if client with this ID already exists - prevent duplicate registrations
	if existing, ok := lh.clients[client.ID]; ok {
		log.Printf("⚠️ WARNING: Client %s already registered in lobby %s! This might indicate duplicate connections.", client.ID, lh.lobby.ID)
		log.Printf("  Existing client Send channel: %p, New client Send channel: %p", existing.Send, client.Send)
		// Close old connection's send channel if different
		if existing.Send != client.Send {
			log.Printf("  Closing old connection's Send channel")
			close(existing.Send)
		}
	}
	lh.clients[client.ID] = client
	clientCount := len(lh.clients)
	lh.mu.Unlock()
	log.Printf("Lobby %s now has %d registered connection(s)", lh.lobby.ID, clientCount)
	// Also send to channel for logging/notification purposes (non-blocking)
	select {
	case lh.register <- client:
	default:
		// Channel full, skip (client already registered)
	}
}

func (lh *LobbyHub) Unregister(client *WebSocketClient) {
	lh.unregister <- client
}

func (lh *LobbyHub) Broadcast(data []byte) {
	lh.broadcast <- data
}

func (lh *LobbyHub) GetLobby() *models.Lobby {
	return lh.lobby
}

func (lh *LobbyHub) GetClients() map[string]*WebSocketClient {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	return lh.clients
}
