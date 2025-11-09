package repository

import (
	"buildprize-game/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
		started_at TIMESTAMP WITH TIME ZONE,
		finished_at TIMESTAMP WITH TIME ZONE,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);`
	
	// Add finished_at column if it doesn't exist (for existing databases)
	addFinishedAtColumn := `
	ALTER TABLE lobbies ADD COLUMN IF NOT EXISTS finished_at TIMESTAMP WITH TIME ZONE;
	ALTER TABLE lobbies ADD COLUMN IF NOT EXISTS started_at TIMESTAMP WITH TIME ZONE;
	`

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
	// Add missing columns for existing databases
	if _, err := db.Exec(addFinishedAtColumn); err != nil {
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
		INSERT INTO lobbies (id, name, state, round, max_rounds, current_question, created_at, started_at, finished_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			state = EXCLUDED.state,
			round = EXCLUDED.round,
			max_rounds = EXCLUDED.max_rounds,
			current_question = EXCLUDED.current_question,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at,
			updated_at = EXCLUDED.updated_at
	`

	var questionJSON interface{} // Use interface{} so we can pass NULL to PostgreSQL
	if lobby.CurrentQ != nil {
		jsonBytes, err := json.Marshal(lobby.CurrentQ)
		if err != nil {
			log.Printf("WARNING: Failed to marshal current question for lobby %s: %v", lobby.ID, err)
			questionJSON = nil
		} else {
			questionJSON = jsonBytes
		}
	} else {
		questionJSON = nil // NULL for PostgreSQL when no question
	}

	log.Printf("DEBUG SaveLobby: Saving lobby '%s' (ID: %s) with State: '%s' (type: %T), Round: %d", lobby.Name, lobby.ID, lobby.State, lobby.State, lobby.Round)
	
	_, err = tx.Exec(query,
		lobby.ID,
		lobby.Name,
		lobby.State,
		lobby.Round,
		lobby.MaxRounds,
		questionJSON, // This will be NULL if no question, or JSON bytes if there is one
		lobby.CreatedAt,
		lobby.StartedAt,
		lobby.FinishedAt,
		time.Now(),
	)
	if err != nil {
		log.Printf("ERROR SaveLobby: Failed to save lobby %s: %v", lobby.ID, err)
		return err
	}
	
	log.Printf("DEBUG SaveLobby: Successfully saved lobby '%s' with state '%s'", lobby.Name, lobby.State)

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
		SELECT id, name, state, round, max_rounds, current_question, created_at, started_at, finished_at
		FROM lobbies WHERE id = $1
	`

	var lobby models.Lobby
	var questionJSON []byte
	var startedAt, finishedAt sql.NullTime

	err := r.db.QueryRow(lobbyQuery, lobbyID).Scan(
		&lobby.ID, &lobby.Name, &lobby.State, &lobby.Round,
		&lobby.MaxRounds, &questionJSON, &lobby.CreatedAt, &startedAt, &finishedAt,
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
	
	// Set started_at and finished_at if they exist
	if startedAt.Valid {
		lobby.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		lobby.FinishedAt = &finishedAt.Time
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
	// DEBUG: First check what's actually in the database
	debugQuery := `SELECT id, name, state, round, created_at FROM lobbies ORDER BY created_at DESC LIMIT 10`
	debugRows, _ := r.db.Query(debugQuery)
	if debugRows != nil {
		log.Printf("DEBUG: All lobbies in database:")
		debugCount := 0
		for debugRows.Next() {
			var id, name, state string
			var round int
			var createdAt time.Time
			if err := debugRows.Scan(&id, &name, &state, &round, &createdAt); err == nil {
				log.Printf("  - Lobby: '%s' (ID: %s, State: '%s', Round: %d, Created: %s)", name, id, state, round, createdAt.Format("15:04:05"))
				debugCount++
			}
		}
		debugRows.Close()
		log.Printf("DEBUG: Found %d total lobbies in database", debugCount)
	}

	// Query to get all waiting lobbies (including those with 0 players)
	query := `
		SELECT l.id, l.name, l.state, l.round, l.max_rounds, l.created_at
		FROM lobbies l
		WHERE LOWER(l.state) = 'waiting'
		ORDER BY l.created_at DESC
		LIMIT 50
	`

	log.Printf("DEBUG: Executing query for waiting lobbies: WHERE LOWER(l.state) = 'waiting'")
	rows, err := r.db.Query(query)
	if err != nil {
		log.Printf("ERROR: ListLobbies query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	lobbies := make([]*models.Lobby, 0) // Initialize as empty slice, not nil
	for rows.Next() {
		var lobby models.Lobby
		err := rows.Scan(&lobby.ID, &lobby.Name, &lobby.State, &lobby.Round, &lobby.MaxRounds, &lobby.CreatedAt)
		if err != nil {
			log.Printf("ERROR: Failed to scan lobby row: %v", err)
			return nil, err
		}
		
		// Load players for this lobby (even if 0 players, lobby should still show)
		playersQuery := `
			SELECT id, username, score, streak, is_ready
			FROM players WHERE lobby_id = $1
			ORDER BY score DESC, username
		`
		playerRows, err := r.db.Query(playersQuery, lobby.ID)
		if err == nil {
			defer playerRows.Close()
			for playerRows.Next() {
				var player models.Player
				if err := playerRows.Scan(&player.ID, &player.Username, &player.Score, &player.Streak, &player.IsReady); err == nil {
					lobby.Players = append(lobby.Players, &player)
				}
			}
		}
		
		log.Printf("ListLobbies: Found waiting lobby '%s' (ID: %s, State: '%s', Players: %d)", lobby.Name, lobby.ID, lobby.State, len(lobby.Players))
		lobbies = append(lobbies, &lobby)
	}

	log.Printf("ListLobbies: Returning %d waiting lobbies", len(lobbies))
	return lobbies, nil
}

// DeleteFinishedGamesOlderThan deletes finished games that finished more than the specified duration ago
func (r *PostgresRepository) DeleteFinishedGamesOlderThan(duration time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-duration)
	query := `
		DELETE FROM lobbies 
		WHERE state = 'finished' 
		AND finished_at IS NOT NULL 
		AND finished_at < $1
	`
	result, err := r.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, err
	}
	
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	
	return int(deleted), nil
}

func (r *PostgresRepository) Close() error {
	return r.db.Close()
}
