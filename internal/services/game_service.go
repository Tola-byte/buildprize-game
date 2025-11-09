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
	gs := &GameService{
		hub:        hub,
		repo:       repo,
		questionDB: NewQuestionDatabase(),
	}

	// Start background cleanup task for finished games
	go gs.startCleanupTask()

	return gs
}

// startCleanupTask runs periodically to delete finished games older than 10 minutes
func (gs *GameService) startCleanupTask() {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		deleted, err := gs.repo.DeleteFinishedGamesOlderThan(10 * time.Minute)
		if err != nil {
			log.Printf("Error cleaning up finished games: %v", err)
		} else if deleted > 0 {
			log.Printf("Cleaned up %d finished game(s) older than 10 minutes", deleted)
		}
	}
}

func (gs *GameService) GetRepository() repository.Repository {
	return gs.repo
}

func (gs *GameService) CreateLobby(name string, maxRounds int) *models.Lobby {
	lobby := models.NewLobby(name, maxRounds)
	gs.hub.CreateLobbyHub(lobby)

	// Save lobby to database
	if err := gs.repo.SaveLobby(lobby); err != nil {
		log.Printf("ERROR: Failed to save lobby %s: %v", lobby.ID, err)
	} else {
		log.Printf("Created lobby %s with ID %s, State: %s, Players: %d - Saved to database", name, lobby.ID, lobby.State, len(lobby.Players))
	}

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

	log.Printf("Player %s joined lobby %s, State: %s, Total players: %d", username, lobbyID, lobby.State, len(lobby.Players))

	// Broadcast player joined
	gs.BroadcastLobbyUpdate(lobbyHub, "player_joined", map[string]interface{}{
		"player": player,
		"lobby":  lobby,
	})

	// Removed auto-start - lobbies now stay in "waiting" state until host manually starts
	// This keeps lobbies visible in the list so others can see and join them
	// Host can start the game when ready using the "Start Game" button

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
	gs.BroadcastLobbyUpdate(lobbyHub, "player_left", map[string]interface{}{
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

	// Broadcast game started event with updated lobby state
	gs.BroadcastLobbyUpdate(lobbyHub, "game_started", map[string]interface{}{
		"lobby": lobby,
	})

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
	gs.BroadcastLobbyUpdate(lobbyHub, "answer_received", map[string]interface{}{
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
	lobby.SetQuestion(question, 15*time.Second)

	gs.repo.SaveLobby(lobby)

	// Broadcast new question with server timestamp for synchronization
	// Capture timestamps right before broadcasting to ensure accuracy
	// All clients will use these timestamps to calculate synchronized countdown
	questionEndTimestamp := lobby.QuestionEnd.UnixMilli() // Absolute server time when question ends
	currentServerTime := time.Now().UnixMilli()           // Server time right before broadcast

	// Broadcast immediately to minimize delay differences between clients
	gs.BroadcastLobbyUpdate(lobbyHub, "new_question", map[string]interface{}{
		"question":          question,
		"round":             lobby.Round,
		"time_left":         15,
		"question_end_time": questionEndTimestamp, // Server timestamp when question ends (absolute time)
		"server_time":       currentServerTime,    // Server time when message is sent (for clock sync)
	})

	// Schedule question end
	go func() {
		time.Sleep(15 * time.Second)
		gs.endQuestion(lobbyHub)
	}()
}

func (gs *GameService) endQuestion(lobbyHub *hub.LobbyHub) {
	lobby := lobbyHub.GetLobby()

	// Calculate leaderboard
	leaderboard := gs.calculateLeaderboard(lobby)

	// Broadcast results
	gs.BroadcastLobbyUpdate(lobbyHub, "question_results", map[string]interface{}{
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

	// Set finished timestamp for cleanup tracking
	now := time.Now()
	lobby.FinishedAt = &now

	leaderboard := gs.calculateLeaderboard(lobby)

	eventData := map[string]interface{}{
		"final_leaderboard": leaderboard,
	}

	// Only set winner if there's at least one player
	if len(leaderboard) > 0 {
		eventData["winner"] = leaderboard[0]
	} else {
		log.Printf("⚠️ WARNING: Game ended with no players in lobby %s", lobby.ID)
		eventData["winner"] = nil
	}

	gs.BroadcastLobbyUpdate(lobbyHub, "game_ended", eventData)

	gs.repo.SaveLobby(lobby)
	log.Printf("Game finished for lobby %s, will be deleted in 10 minutes", lobby.ID)
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

func (gs *GameService) BroadcastLobbyUpdate(lobbyHub *hub.LobbyHub, eventType string, data interface{}) {
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
