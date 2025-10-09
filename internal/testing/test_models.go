package testing

import "buildprize-game/internal/models"

// Test data structures
type CreateLobbyRequest struct {
	Name      string `json:"name"`
	MaxRounds int    `json:"max_rounds"`
}

type JoinLobbyRequest struct {
	Username string `json:"username"`
}

type LeaveLobbyRequest struct {
	PlayerID string `json:"player_id"`
}

type SubmitAnswerRequest struct {
	PlayerID     string `json:"player_id"`
	Answer       int    `json:"answer"`
	ResponseTime int    `json:"response_time"`
}

type LobbyResponse struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Players   []models.Player  `json:"players"`
	State     string           `json:"state"`
	Round     int              `json:"round"`
	MaxRounds int              `json:"max_rounds"`
	CurrentQ  *models.Question `json:"current_question,omitempty"`
	CreatedAt string           `json:"created_at"`
}

type JoinLobbyResponse struct {
	Lobby  LobbyResponse `json:"lobby"`
	Player models.Player `json:"player"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
