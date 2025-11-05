package repository

import (
	"buildprize-game/internal/models"
)

type Repository interface {
	SaveLobby(lobby *models.Lobby) error
	GetLobby(lobbyID string) (*models.Lobby, error)
	DeleteLobby(lobbyID string) error
	ListLobbies() ([]*models.Lobby, error)
}
