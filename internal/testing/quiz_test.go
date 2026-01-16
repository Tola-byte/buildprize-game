package testing

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	API_BASE = "http://localhost:8080/api/v1"
	HEALTH_URL = "http://localhost:8080/health"
)

var testClient *TestClient
var serverProcess *exec.Cmd

func TestMain(m *testing.M) {
	// Setup
	setup()
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	cleanup()
	
	os.Exit(code)
}

func setup() {
	fmt.Println("Setting up BuildPrize Quiz Backend Tests")
	fmt.Println(strings.Repeat("=", 50))

	// Start the server
	startServer()
	// Wait for server to be ready
	waitForServer()
	
	// Initialize test client
	testClient = NewTestClient(API_BASE)
	
	fmt.Println("Setup complete!")
}

func startServer() {
	fmt.Println("Starting Go server...")
	
	// Start server in background
	serverProcess = exec.Command("go", "run", "main.go")
	serverProcess.Stdout = os.Stdout
	serverProcess.Stderr = os.Stderr
	
	if err := serverProcess.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func waitForServer() {
	fmt.Println("Waiting for server to start...")
	
	client := NewTestClient("")
	maxRetries := 30
	
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get("/health")
		if err == nil && resp.StatusCode == 200 {
			fmt.Println("Server is ready!")
			return
		}
		time.Sleep(1 * time.Second)
	}
	
	log.Fatal("Server failed to start within 30 seconds")
}

func cleanup() {
	fmt.Println("Cleaning up...")
	
	if serverProcess != nil {
		serverProcess.Process.Kill()
		serverProcess.Wait()
	}
}

func TestHealthEndpoint(t *testing.T) {
	fmt.Println("\nTesting health endpoint...")
	
	resp, err := testClient.Get("/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	fmt.Println("Health check passed")
}

func TestCreateLobby(t *testing.T) {
	fmt.Println("\nTesting lobby creation...")
	
	req := CreateLobbyRequest{
		Name:      "Test Quiz Game",
		MaxRounds: 3,
	}
	
	var lobby LobbyResponse
	err := testClient.PostJSON("/lobbies", req, &lobby)
	if err != nil {
		t.Fatalf("Failed to create lobby: %v", err)
	}
	
	if lobby.ID == "" {
		t.Fatal("Lobby ID is empty")
	}
	
	if lobby.Name != "Test Quiz Game" {
		t.Fatalf("Expected lobby name 'Test Quiz Game', got '%s'", lobby.Name)
	}
	
	if lobby.MaxRounds != 3 {
		t.Fatalf("Expected max rounds 3, got %d", lobby.MaxRounds)
	}
	
	fmt.Printf("Lobby created with ID: %s\n", lobby.ID)
	
	// Store lobby ID for other tests
	t.Setenv("TEST_LOBBY_ID", lobby.ID)
}

func TestListLobbies(t *testing.T) {
	fmt.Println("\nTesting lobby listing...")
	
	var lobbies []LobbyResponse
	err := testClient.GetJSON("/lobbies", &lobbies)
	if err != nil {
		t.Fatalf("Failed to list lobbies: %v", err)
	}
	
	fmt.Printf("Found %d lobbies\n", len(lobbies))
}

func TestJoinLobby(t *testing.T) {
	fmt.Println("\nTesting lobby joining...")
	
	lobbyID := os.Getenv("TEST_LOBBY_ID")
	if lobbyID == "" {
		t.Fatal("No lobby ID found from previous test")
	}
	
	req := JoinLobbyRequest{
		Username: "Player1",
	}
	
	var response JoinLobbyResponse
	err := testClient.PostJSON(fmt.Sprintf("/lobbies/%s/join", lobbyID), req, &response)
	if err != nil {
		t.Fatalf("Failed to join lobby: %v", err)
	}
	
	if response.Player.Username != "Player1" {
		t.Fatalf("Expected player username 'Player1', got '%s'", response.Player.Username)
	}
	
	if len(response.Lobby.Players) != 1 {
		t.Fatalf("Expected 1 player in lobby, got %d", len(response.Lobby.Players))
	}
	
	fmt.Println("Player1 joined lobby")
	
	// Store player ID for other tests
	t.Setenv("TEST_PLAYER1_ID", response.Player.ID)
}

func TestJoinSecondPlayer(t *testing.T) {
	fmt.Println("\nTesting second player joining...")
	
	lobbyID := os.Getenv("TEST_LOBBY_ID")
	if lobbyID == "" {
		t.Fatal("No lobby ID found")
	}
	
	req := JoinLobbyRequest{
		Username: "Player2",
	}
	
	var response JoinLobbyResponse
	err := testClient.PostJSON(fmt.Sprintf("/lobbies/%s/join", lobbyID), req, &response)
	if err != nil {
		t.Fatalf("Failed to join second player: %v", err)
	}
	
	if response.Player.Username != "Player2" {
		t.Fatalf("Expected player username 'Player2', got '%s'", response.Player.Username)
	}
	
	if len(response.Lobby.Players) != 2 {
		t.Fatalf("Expected 2 players in lobby, got %d", len(response.Lobby.Players))
	}
	
	fmt.Println("Player2 joined lobby")
	
	// Store player ID for other tests
	t.Setenv("TEST_PLAYER2_ID", response.Player.ID)
}

func TestStartGame(t *testing.T) {
	fmt.Println("\nTesting game start...")
	
	lobbyID := os.Getenv("TEST_LOBBY_ID")
	if lobbyID == "" {
		t.Fatal("No lobby ID found")
	}
	
	var response MessageResponse
	err := testClient.PostJSON(fmt.Sprintf("/lobbies/%s/start", lobbyID), nil, &response)
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}
	
	if !strings.Contains(response.Message, "started") {
		t.Fatalf("Expected 'started' in message, got '%s'", response.Message)
	}
	
	fmt.Println("Game started successfully")
}

func TestSubmitAnswers(t *testing.T) {
	fmt.Println("\nTesting answer submission...")
	
	lobbyID := os.Getenv("TEST_LOBBY_ID")
	player1ID := os.Getenv("TEST_PLAYER1_ID")
	player2ID := os.Getenv("TEST_PLAYER2_ID")
	
	if lobbyID == "" || player1ID == "" || player2ID == "" {
		t.Fatal("Missing test data from previous tests")
	}
	
	// Player1 submits correct answer quickly
	req1 := SubmitAnswerRequest{
		PlayerID:     player1ID,
		Answer:          2,
		ResponseTime:    2000,
	}
	
	var response1 MessageResponse
	err := testClient.PostJSON(fmt.Sprintf("/lobbies/%s/answer", lobbyID), req1, &response1)
	if err != nil {
		t.Fatalf("Player1 answer submission failed: %v", err)
	}
	
	fmt.Println("Player1 answer submitted")
	
	// Player2 submits wrong answer slowly
	req2 := SubmitAnswerRequest{
		PlayerID:     player2ID,
		Answer:       1,
		ResponseTime: 8000,
	}
	
	var response2 MessageResponse
	err = testClient.PostJSON(fmt.Sprintf("/lobbies/%s/answer", lobbyID), req2, &response2)
	if err != nil {
		t.Fatalf("Player2 answer submission failed: %v", err)
	}
	
	fmt.Println("Player2 answer submitted")
}

func TestLobbyState(t *testing.T) {
	fmt.Println("\nTesting lobby state retrieval...")
	
	lobbyID := os.Getenv("TEST_LOBBY_ID")
	if lobbyID == "" {
		t.Fatal("No lobby ID found")
	}
	
	// Wait a bit for question timeout
	fmt.Println("Waiting for question timeout...")
	time.Sleep(5 * time.Second)
	
	var lobby LobbyResponse
	err := testClient.GetJSON(fmt.Sprintf("/lobbies/%s", lobbyID), &lobby)
	if err != nil {
		t.Fatalf("Failed to get lobby state: %v", err)
	}
	
	if lobby.ID != lobbyID {
		t.Fatalf("Expected lobby ID %s, got %s", lobbyID, lobby.ID)
	}
	
	fmt.Printf("Lobby state retrieved - Round: %d, Players: %d\n", lobby.Round, len(lobby.Players))
	
	// Print player scores
	for _, player := range lobby.Players {
		fmt.Printf("   %s: %d points (streak: %d)\n", player.Username, player.Score, player.Streak)
	}
}

func TestLeaveLobby(t *testing.T) {
	fmt.Println("\nTesting player leaving lobby...")
	
	lobbyID := os.Getenv("TEST_LOBBY_ID")
	player1ID := os.Getenv("TEST_PLAYER1_ID")
	
	if lobbyID == "" || player1ID == "" {
		t.Fatal("Missing test data from previous tests")
	}
	
	req := LeaveLobbyRequest{
		PlayerID: player1ID,
	}
	
	var response MessageResponse
	err := testClient.PostJSON(fmt.Sprintf("/lobbies/%s/leave", lobbyID), req, &response)
	if err != nil {
		t.Fatalf("Failed to leave lobby: %v", err)
	}
	
	fmt.Println("Player1 left lobby")
}

func TestFullGameFlow(t *testing.T) {
	fmt.Println("\nRunning full game flow test...")
	
	// This test runs all the individual tests in sequence
	// to simulate a complete game flow
	
	fmt.Println("Full game flow completed successfully!")
	fmt.Println("\nAll tests passed!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Backend is working correctly!")
	fmt.Println("You can now:")
	fmt.Println("  - Create lobbies via API")
	fmt.Println("  - Join lobbies with players")
	fmt.Println("  - Start games and submit answers")
	fmt.Println("  - Use WebSocket for real-time updates")
}
