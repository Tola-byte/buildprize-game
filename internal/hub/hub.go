package hub

import (
	"log"
	"sync"
	"time"

	"buildprize-game/internal/models"
)

// Hub manages all lobbies and their connections
type Hub struct {
	lobbies map[string]*LobbyHub
	mu      sync.RWMutex
}

// LobbyHub manages connections for a specific lobby
type LobbyHub struct {
	lobby      *models.Lobby
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
}

// Client represents a WebSocket connection
type Client struct {
	ID       string
	LobbyID  string
	PlayerID string
	Send     chan []byte
	Hub      *LobbyHub
}

type LobbyHubInterface interface {
	Register(client *Client)
	Unregister(client *Client)
	Broadcast(data []byte)
	GetLobby() *models.Lobby
	GetClients() map[string]*Client
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
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
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
			lh.mu.Lock()
			lh.clients[client.ID] = client
			lh.mu.Unlock()
			log.Printf("Client %s joined lobby %s", client.ID, lh.lobby.ID)

		case client := <-lh.unregister:
			lh.mu.Lock()
			if _, ok := lh.clients[client.ID]; ok {
				delete(lh.clients, client.ID)
				close(client.Send)
			}
			lh.mu.Unlock()
			log.Printf("Client %s left lobby %s", client.ID, lh.lobby.ID)

		case message := <-lh.broadcast:
			lh.mu.RLock()
			for _, client := range lh.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(lh.clients, client.ID)
				}
			}
			lh.mu.RUnlock()

		case <-ticker.C:
			// Check if question time has expired
			if lh.lobby.IsQuestionActive() && time.Now().After(*lh.lobby.QuestionEnd) {
				lh.handleQuestionTimeout()
			}
		}
	}
}

func (lh *LobbyHub) Register(client *Client) {
	log.Printf("Registering client %s with lobby %s", client.ID, lh.lobby.ID)
	lh.register <- client
}

func (lh *LobbyHub) Unregister(client *Client) {
	lh.unregister <- client
}

func (lh *LobbyHub) Broadcast(data []byte) {
	lh.broadcast <- data
}

func (lh *LobbyHub) GetLobby() *models.Lobby {
	return lh.lobby
}

func (lh *LobbyHub) GetClients() map[string]*Client {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	return lh.clients
}

func (lh *LobbyHub) handleQuestionTimeout() {
	// End current question and move to next round
	lh.lobby.CurrentQ = nil
	lh.lobby.QuestionEnd = nil
	lh.lobby.NextRound()

	// Broadcast round end
	// This would be handled by the game service
	// The actual broadcasting is done in the game service
}
