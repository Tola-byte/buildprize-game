package hub

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"buildprize-game/internal/models"
)

type Hub struct {
	lobbies map[string]*LobbyHub
	mu      sync.RWMutex
}
type LobbyHub struct {
	lobby      *models.Lobby
	clients    map[string]*WebSocketClient
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	broadcast  chan []byte
	mu         sync.RWMutex
}

type WebSocketClient struct {
	ID       string
	LobbyID  string
	PlayerID string
	Send     chan []byte
	Hub      *LobbyHub
}

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

func (h *Hub) GetAllLobbies() map[string]*LobbyHub {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]*LobbyHub)
	for k, v := range h.lobbies {
		result[k] = v
	}
	return result
}

func (lh *LobbyHub) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-lh.register:
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
			clientCount := len(lh.clients)
			log.Printf("LobbyHub: Broadcasting message to %d clients in lobby %s", clientCount, lh.lobby.ID)

			// Collect clients that need to be removed
			var clientsToRemove []string
			successCount := 0
			for clientID, client := range lh.clients {
				select {
				case client.Send <- message:
					successCount++
					// Log chat messages being sent
					var msgData map[string]interface{}
					if err := json.Unmarshal(message, &msgData); err == nil {
						if msgType, ok := msgData["type"].(string); ok && msgType == "chat_message" {
							log.Printf("  Sent chat_message to client %s (player: %s)", clientID, client.PlayerID)
						}
					}
				default:
					// Client's send channel is full, mark for removal
					log.Printf("  Client %s send channel full, marking for removal", clientID)
					clientsToRemove = append(clientsToRemove, client.ID)
				}
			}
			log.Printf("LobbyHub: Successfully queued message to %d/%d clients", successCount, clientCount)
			lh.mu.RUnlock()

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
		}
	}
}

func (lh *LobbyHub) Register(client *WebSocketClient) {
	log.Printf("Registering player connection %s (player: %s) with lobby %s", client.ID, client.PlayerID, lh.lobby.ID)
	lh.mu.Lock()
	if existing, ok := lh.clients[client.ID]; ok {
		log.Printf("WARNING: Client %s already registered in lobby %s! This might indicate duplicate connections.", client.ID, lh.lobby.ID)
		log.Printf("  Existing client Send channel: %p, New client Send channel: %p", existing.Send, client.Send)
		if existing.Send != client.Send {
			log.Printf("  Closing old connection's Send channel")
			close(existing.Send)
		}
	}
	lh.clients[client.ID] = client
	clientCount := len(lh.clients)
	lh.mu.Unlock()
	log.Printf("Lobby %s now has %d registered connection(s)", lh.lobby.ID, clientCount)
	select {
	case lh.register <- client:
	default:
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
	result := make(map[string]*WebSocketClient)
	for k, v := range lh.clients {
		result[k] = v
	}
	return result
}
