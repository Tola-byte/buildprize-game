package services

import "errors"

var (
	ErrLobbyNotFound     = errors.New("lobby not found")
	ErrLobbyFull         = errors.New("lobby is full")
	ErrGameInProgress    = errors.New("game is already in progress")
	ErrPlayerNotFound    = errors.New("player not found")
	ErrCannotStartGame   = errors.New("cannot start game")
	ErrQuestionNotActive = errors.New("no active question")
)
