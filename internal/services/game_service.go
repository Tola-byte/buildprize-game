package services

import (
	"encoding/json"
	"log"
	"math"
	"time"

	"buildprize-game/internal/hub"
	"buildprize-game/internal/models"
	"buildprize-game/internal/repository"
)

type GameService struct {
	hub        *hub.Hub
	repo       repository.Repository
	questionDB *QuestionDatabase
}

func NewGameService(hub *hub.Hub, repo repository.Repository) *GameService {
	return &GameService{
		hub:        hub,
		repo:       repo,
		questionDB: NewQuestionDatabase(),
	}
}

func (gs *GameService) GetRepository() repository.Repository {
	return gs.repo
}

func (gs *GameService) CreateLobby(name string, maxRounds int) *models.Lobby {
	lobby := models.NewLobby(name, maxRounds)
	gs.hub.CreateLobbyHub(lobby)
	gs.repo.SaveLobby(lobby)

	log.Printf("Created lobby %s with ID %s", name, lobby.ID)
	return lobby
}

func (gs *GameService) JoinLobby(lobbyID, username string) (*models.Lobby, *models.Player, error) {
	lobbyHub := gs.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		return nil, nil, ErrLobbyNotFound
	}

	lobby := lobbyHub.GetLobby()
	if len(lobby.Players) >= 8 { // Max lobby size
		return nil, nil, ErrLobbyFull
	}

	if lobby.State != models.Waiting {
		return nil, nil, ErrGameInProgress
	}

	player := lobby.AddPlayer(username)
	gs.repo.SaveLobby(lobby)

	// Broadcast player joined
	gs.broadcastLobbyUpdate(lobbyHub, "player_joined", map[string]interface{}{
		"player": player,
		"lobby":  lobby,
	})

	return lobby, player, nil
}

func (gs *GameService) LeaveLobby(lobbyID, playerID string) error {
	lobbyHub := gs.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		return ErrLobbyNotFound
	}

	lobby := lobbyHub.GetLobby()
	removed := lobby.RemovePlayer(playerID)
	if !removed {
		return ErrPlayerNotFound
	}

	gs.repo.SaveLobby(lobby)

	// Broadcast player left
	gs.broadcastLobbyUpdate(lobbyHub, "player_left", map[string]interface{}{
		"player_id": playerID,
		"lobby":     lobby,
	})

	// If no players left, remove lobby
	if len(lobby.Players) == 0 {
		gs.hub.RemoveLobbyHub(lobbyID)
		gs.repo.DeleteLobby(lobbyID)
	}

	return nil
}

func (gs *GameService) StartGame(lobbyID string) error {
	lobbyHub := gs.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		return ErrLobbyNotFound
	}

	lobby := lobbyHub.GetLobby()
	if !lobby.CanStart() {
		return ErrCannotStartGame
	}

	lobby.StartGame()
	gs.repo.SaveLobby(lobby)

	// Start first question
	gs.startNextQuestion(lobbyHub)

	return nil
}

func (gs *GameService) SubmitAnswer(lobbyID, playerID string, answer int, responseTime int64) error {
	lobbyHub := gs.hub.GetLobbyHub(lobbyID)
	if lobbyHub == nil {
		return ErrLobbyNotFound
	}

	lobby := lobbyHub.GetLobby()
	if !lobby.IsQuestionActive() {
		return ErrQuestionNotActive
	}

	player := lobby.GetPlayer(playerID)
	if player == nil {
		return ErrPlayerNotFound
	}

	// Calculate score
	score := gs.calculateScore(lobby.CurrentQ, answer, responseTime)
	player.Score += score

	// Update streak
	if answer == lobby.CurrentQ.Correct {
		player.Streak++
	} else {
		player.Streak = 0
	}

	gs.repo.SaveLobby(lobby)

	// Broadcast answer received
	gs.broadcastLobbyUpdate(lobbyHub, "answer_received", map[string]interface{}{
		"player_id": playerID,
		"score":     score,
		"streak":    player.Streak,
	})

	return nil
}

func (gs *GameService) calculateScore(question *models.Question, answer int, responseTime int64) int {
	if answer != question.Correct {
		return 0
	}

	// Base score for correct answer
	baseScore := 100

	// Time bonus (faster = higher score)
	timeBonus := int(math.Max(0, float64(50-(responseTime/1000)))) // 50 points max for speed

	// Accuracy bonus
	accuracyBonus := 25

	return baseScore + timeBonus + accuracyBonus
}

func (gs *GameService) startNextQuestion(lobbyHub *hub.LobbyHub) {
	lobby := lobbyHub.GetLobby()

	if lobby.Round > lobby.MaxRounds {
		gs.endGame(lobbyHub)
		return
	}

	question := gs.questionDB.GetRandomQuestion()
	lobby.SetQuestion(question, 30*time.Second)

	gs.repo.SaveLobby(lobby)

	// Broadcast new question
	gs.broadcastLobbyUpdate(lobbyHub, "new_question", map[string]interface{}{
		"question":  question,
		"round":     lobby.Round,
		"time_left": 30,
	})

	// Schedule question end
	go func() {
		time.Sleep(30 * time.Second)
		gs.endQuestion(lobbyHub)
	}()
}

func (gs *GameService) endQuestion(lobbyHub *hub.LobbyHub) {
	lobby := lobbyHub.GetLobby()

	// Calculate leaderboard
	leaderboard := gs.calculateLeaderboard(lobby)

	// Broadcast results
	gs.broadcastLobbyUpdate(lobbyHub, "question_results", map[string]interface{}{
		"correct_answer": lobby.CurrentQ.Correct,
		"leaderboard":    leaderboard,
		"round":          lobby.Round,
	})

	// Clear current question
	lobby.CurrentQ = nil
	lobby.QuestionEnd = nil
	lobby.NextRound()

	gs.repo.SaveLobby(lobby)

	// Start next question after delay
	time.Sleep(3 * time.Second)
	gs.startNextQuestion(lobbyHub)
}

func (gs *GameService) endGame(lobbyHub *hub.LobbyHub) {
	lobby := lobbyHub.GetLobby()
	lobby.State = models.Finished

	leaderboard := gs.calculateLeaderboard(lobby)

	gs.broadcastLobbyUpdate(lobbyHub, "game_ended", map[string]interface{}{
		"final_leaderboard": leaderboard,
		"winner":            leaderboard[0],
	})

	gs.repo.SaveLobby(lobby)
}

func (gs *GameService) calculateLeaderboard(lobby *models.Lobby) []*models.Player {
	// Sort players by score (descending)
	players := make([]*models.Player, len(lobby.Players))
	copy(players, lobby.Players)

	// Simple bubble sort for leaderboard
	for i := 0; i < len(players)-1; i++ {
		for j := 0; j < len(players)-i-1; j++ {
			if players[j].Score < players[j+1].Score {
				players[j], players[j+1] = players[j+1], players[j]
			}
		}
	}

	return players
}

func (gs *GameService) broadcastLobbyUpdate(lobbyHub *hub.LobbyHub, eventType string, data interface{}) {
	event := models.GameEvent{
		Type:      eventType,
		LobbyID:   lobbyHub.GetLobby().ID,
		Data:      data,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling event: %v", err)
		return
	}

	log.Printf("Broadcasting %s event to lobby %s with %d clients", eventType, lobbyHub.GetLobby().ID, len(lobbyHub.GetClients()))
	lobbyHub.Broadcast(jsonData)
}
