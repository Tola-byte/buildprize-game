// Get API base URL
// When frontend is on ngrok and backend is on localhost, you need to expose backend via ngrok too
// Set VITE_API_BASE_URL in .env file to your backend ngrok URL
function getApiBase() {
  // Check for environment variable first (for explicit configuration)
  // Example: VITE_API_BASE_URL=https://your-backend-ngrok.ngrok.io/api/v1
  if (import.meta.env.VITE_API_BASE_URL) {
    return import.meta.env.VITE_API_BASE_URL;
  }
  
  // In development with vite proxy (localhost), use relative URL
  if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
    return '/api/v1';
  }
  
  // If frontend is on ngrok but no env var set, try same origin
  // (This only works if backend is on same ngrok tunnel)
  const protocol = window.location.protocol;
  const host = window.location.host;
  return `${protocol}//${host}/api/v1`;
}

const API_BASE = getApiBase();

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

  // Send a chat message
  sendChatMessage: async (lobbyId, playerId, message) => {
    const response = await fetch(`${API_BASE}/lobbies/${lobbyId}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        player_id: playerId,
        message: message,
      }),
    });
    if (!response.ok) throw new Error('Failed to send chat message');
    return response.json();
  },
};
