package repository

import (
	"buildprize-game/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(databaseURL string) (*PostgresRepository, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &PostgresRepository{db: db}, nil
}

func createTables(db *sql.DB) error {
	createLobbiesTable := `
	CREATE TABLE IF NOT EXISTS lobbies (
		id VARCHAR(36) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		state VARCHAR(50) NOT NULL DEFAULT 'waiting',
		round INTEGER NOT NULL DEFAULT 0,
		max_rounds INTEGER NOT NULL DEFAULT 10,
		current_question JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);`

	createPlayersTable := `
	CREATE TABLE IF NOT EXISTS players (
		id VARCHAR(36) PRIMARY KEY,
		lobby_id VARCHAR(36) NOT NULL REFERENCES lobbies(id) ON DELETE CASCADE,
		username VARCHAR(255) NOT NULL,
		score INTEGER NOT NULL DEFAULT 0,
		streak INTEGER NOT NULL DEFAULT 0,
		is_ready BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);`

	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_players_lobby_id ON players(lobby_id);
	CREATE INDEX IF NOT EXISTS idx_lobbies_state ON lobbies(state);
	`

	if _, err := db.Exec(createLobbiesTable); err != nil {
		return err
	}
	if _, err := db.Exec(createPlayersTable); err != nil {
		return err
	}
	if _, err := db.Exec(createIndexes); err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) SaveLobby(lobby *models.Lobby) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update or insert lobby
	query := `
		INSERT INTO lobbies (id, name, state, round, max_rounds, current_question, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			state = EXCLUDED.state,
			round = EXCLUDED.round,
			max_rounds = EXCLUDED.max_rounds,
			current_question = EXCLUDED.current_question,
			updated_at = EXCLUDED.updated_at
	`

	var questionJSON []byte
	if lobby.CurrentQ != nil {
		questionJSON, _ = json.Marshal(lobby.CurrentQ)
	}

	_, err = tx.Exec(query,
		lobby.ID,
		lobby.Name,
		lobby.State,
		lobby.Round,
		lobby.MaxRounds,
		questionJSON,
		lobby.CreatedAt,
		time.Now(),
	)
	if err != nil {
		return err
	}

	// Delete existing players for this lobby
	_, err = tx.Exec("DELETE FROM players WHERE lobby_id = $1", lobby.ID)
	if err != nil {
		return err
	}

	// Insert players
	for _, player := range lobby.Players {
		_, err = tx.Exec(`
			INSERT INTO players (id, lobby_id, username, score, streak, is_ready, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, player.ID, lobby.ID, player.Username, player.Score, player.Streak, player.IsReady, time.Now())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresRepository) GetLobby(lobbyID string) (*models.Lobby, error) {
	// Get lobby
	lobbyQuery := `
		SELECT id, name, state, round, max_rounds, current_question, created_at
		FROM lobbies WHERE id = $1
	`

	var lobby models.Lobby
	var questionJSON []byte

	err := r.db.QueryRow(lobbyQuery, lobbyID).Scan(
		&lobby.ID, &lobby.Name, &lobby.State, &lobby.Round,
		&lobby.MaxRounds, &questionJSON, &lobby.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrLobbyNotFound
		}
		return nil, err
	}

	// Parse current question if exists
	if len(questionJSON) > 0 {
		var question models.Question
		if err := json.Unmarshal(questionJSON, &question); err == nil {
			lobby.CurrentQ = &question
		}
	}

	// Get players
	playersQuery := `
		SELECT id, username, score, streak, is_ready
		FROM players WHERE lobby_id = $1
		ORDER BY score DESC, username
	`

	rows, err := r.db.Query(playersQuery, lobbyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var player models.Player
		err := rows.Scan(&player.ID, &player.Username, &player.Score, &player.Streak, &player.IsReady)
		if err != nil {
			return nil, err
		}
		lobby.Players = append(lobby.Players, &player)
	}

	return &lobby, nil
}

func (r *PostgresRepository) DeleteLobby(lobbyID string) error {
	_, err := r.db.Exec("DELETE FROM lobbies WHERE id = $1", lobbyID)
	return err
}

func (r *PostgresRepository) ListLobbies() ([]*models.Lobby, error) {
	query := `
		SELECT l.id, l.name, l.state, l.round, l.max_rounds, l.created_at
		FROM lobbies l
		WHERE l.state IN ('waiting', 'playing')
		ORDER BY l.created_at DESC
		LIMIT 50
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lobbies []*models.Lobby
	for rows.Next() {
		var lobby models.Lobby
		err := rows.Scan(&lobby.ID, &lobby.Name, &lobby.State, &lobby.Round, &lobby.MaxRounds, &lobby.CreatedAt)
		if err != nil {
			return nil, err
		}
		lobbies = append(lobbies, &lobby)
	}

	return lobbies, nil
}

func (r *PostgresRepository) Close() error {
	return r.db.Close()
}
