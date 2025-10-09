package repository

import (
	"buildprize-game/internal/models"
	"sync"
)

type Repository interface {
	SaveLobby(lobby *models.Lobby) error
	GetLobby(lobbyID string) (*models.Lobby, error)
	DeleteLobby(lobbyID string) error
	ListLobbies() ([]*models.Lobby, error)
}

type InMemoryRepository struct {
	lobbies map[string]*models.Lobby
	mu      sync.RWMutex
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		lobbies: make(map[string]*models.Lobby),
	}
}

func (r *InMemoryRepository) SaveLobby(lobby *models.Lobby) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lobbies[lobby.ID] = lobby
	return nil
}

func (r *InMemoryRepository) GetLobby(lobbyID string) (*models.Lobby, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lobby, exists := r.lobbies[lobbyID]
	if !exists {
		return nil, ErrLobbyNotFound
	}
	return lobby, nil
}

func (r *InMemoryRepository) DeleteLobby(lobbyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.lobbies, lobbyID)
	return nil
}

func (r *InMemoryRepository) ListLobbies() ([]*models.Lobby, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	lobbies := make([]*models.Lobby, 0, len(r.lobbies))
	for _, lobby := range r.lobbies {
		lobbies = append(lobbies, lobby)
	}
	return lobbies, nil
}
