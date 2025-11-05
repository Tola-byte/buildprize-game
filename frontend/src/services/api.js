const API_BASE = 'http://localhost:8080/api/v1';

export const api = {
  // Create a new lobby
  createLobby: async (name, maxRounds = 10) => {
    const response = await fetch(`${API_BASE}/lobbies`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, max_rounds: maxRounds }),
    });
    if (!response.ok) throw new Error('Failed to create lobby');
    return response.json();
  },

  // List all lobbies
  listLobbies: async () => {
    const response = await fetch(`${API_BASE}/lobbies`);
    if (!response.ok) throw new Error('Failed to fetch lobbies');
    return response.json();
  },

  // Get lobby details
  getLobby: async (lobbyId) => {
    const response = await fetch(`${API_BASE}/lobbies/${lobbyId}`);
    if (!response.ok) throw new Error('Failed to fetch lobby');
    return response.json();
  },

  // Join a lobby
  joinLobby: async (lobbyId, username) => {
    const response = await fetch(`${API_BASE}/lobbies/${lobbyId}/join`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username }),
    });
    if (!response.ok) throw new Error('Failed to join lobby');
    return response.json();
  },

  // Leave a lobby
  leaveLobby: async (lobbyId, playerId) => {
    const response = await fetch(`${API_BASE}/lobbies/${lobbyId}/leave`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ player_id: playerId }),
    });
    if (!response.ok) throw new Error('Failed to leave lobby');
    return response.json();
  },

  // Start the game
  startGame: async (lobbyId) => {
    const response = await fetch(`${API_BASE}/lobbies/${lobbyId}/start`, {
      method: 'POST',
    });
    if (!response.ok) throw new Error('Failed to start game');
    return response.json();
  },

  // Submit an answer
  submitAnswer: async (lobbyId, playerId, answer, responseTime) => {
    const response = await fetch(`${API_BASE}/lobbies/${lobbyId}/answer`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        player_id: playerId,
        answer,
        response_time: responseTime,
      }),
    });
    if (!response.ok) throw new Error('Failed to submit answer');
    return response.json();
  },
};

