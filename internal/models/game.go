package models

import (
	"time"
	"github.com/google/uuid"
)

type GameState string

const (
	Waiting    GameState = "waiting"
	InProgress GameState = "in_progress"
	Finished   GameState = "finished"
)

type Player struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Score    int    `json:"score"`
	Streak   int    `json:"streak"`
	IsReady  bool   `json:"is_ready"`
}

type Question struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Options  []string `json:"options"`
	Correct  int      `json:"correct"`
	Category string   `json:"category"`
}

type Answer struct {
	PlayerID string `json:"player_id"`
	Answer   int    `json:"answer"`
	Time     int64  `json:"time"` // milliseconds since question start
}

type Lobby struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Players     []*Player  `json:"players"`
	State       GameState  `json:"state"`
	CurrentQ    *Question  `json:"current_question,omitempty"`
	Round       int        `json:"round"`
	MaxRounds   int        `json:"max_rounds"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	QuestionEnd *time.Time `json:"question_end,omitempty"`
}

type GameEvent struct {
	Type      string      `json:"type"`
	LobbyID   string      `json:"lobby_id"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewLobby(name string, maxRounds int) *Lobby {
	return &Lobby{
		ID:        uuid.New().String(),
		Name:      name,
		Players:   make([]*Player, 0),
		State:     Waiting,
		Round:     0,
		MaxRounds: maxRounds,
		CreatedAt: time.Now(),
	}
}

func (l *Lobby) AddPlayer(username string) *Player {
	player := &Player{
		ID:       uuid.New().String(),
		Username: username,
		Score:    0,
		Streak:   0,
		IsReady:  false,
	}
	l.Players = append(l.Players, player)
	return player
}

func (l *Lobby) RemovePlayer(playerID string) bool {
	for i, player := range l.Players {
		if player.ID == playerID {
			l.Players = append(l.Players[:i], l.Players[i+1:]...)
			return true
		}
	}
	return false
}

func (l *Lobby) GetPlayer(playerID string) *Player {
	for _, player := range l.Players {
		if player.ID == playerID {
			return player
		}
	}
	return nil
}

func (l *Lobby) CanStart() bool {
	return len(l.Players) >= 2 && l.State == Waiting
}

func (l *Lobby) StartGame() {
	l.State = InProgress
	l.Round = 1
	now := time.Now()
	l.StartedAt = &now
}

func (l *Lobby) NextRound() {
	l.Round++
	if l.Round > l.MaxRounds {
		l.State = Finished
	}
}

func (l *Lobby) SetQuestion(question *Question, duration time.Duration) {
	l.CurrentQ = question
	endTime := time.Now().Add(duration)
	l.QuestionEnd = &endTime
}

func (l *Lobby) IsQuestionActive() bool {
	return l.CurrentQ != nil && l.QuestionEnd != nil && time.Now().Before(*l.QuestionEnd)
}
