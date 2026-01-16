import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../services/api';
import { wsService } from '../services/websocket';
import './LobbyScreen.css';

export default function LobbyScreen() {
  const [screen, setScreen] = useState('home'); // 'home', 'create', 'join', 'created'
  const [lobbyName, setLobbyName] = useState('');
  const [maxRounds, setMaxRounds] = useState(10);
  const [username, setUsername] = useState('');
  const [lobbyId, setLobbyId] = useState('');
  const [createdLobby, setCreatedLobby] = useState(null);
  const [lobbies, setLobbies] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    loadLobbies();
    // Connect WebSocket if not already connected and not connecting
    if (!wsService.ws || 
        (wsService.ws.readyState !== WebSocket.OPEN && 
         wsService.ws.readyState !== WebSocket.CONNECTING)) {
      wsService.connect();
    }
    // Auto-refresh lobby list every 5 seconds
    const interval = setInterval(loadLobbies, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadLobbies = async () => {
    try {
      const data = await api.listLobbies();
      console.log('Loaded lobbies from API:', data);
      // Handle null/undefined response - ensure we have an array
      if (data && Array.isArray(data)) {
        // Backend now only returns waiting lobbies, but filter just in case
        const waitingLobbies = data.filter(l => l.state === 'waiting');
        console.log('Filtered waiting lobbies:', waitingLobbies);
        setLobbies(waitingLobbies);
      } else {
        console.log('No data or not an array, setting empty array');
        setLobbies([]);
      }
    } catch (err) {
      console.error('Failed to load lobbies:', err);
      setLobbies([]); // Set empty array on error
    }
  };

  const handleCreateLobby = async (e) => {
    e.preventDefault();
    if (!lobbyName || !username) {
      setError('Please enter lobby name and username');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const lobby = await api.createLobby(lobbyName, maxRounds);
      setCreatedLobby(lobby);
      setScreen('created');
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleJoinCreatedLobby = async () => {
    if (!createdLobby) return;
    setLoading(true);
    setError('');
    try {
      const joinResult = await api.joinLobby(createdLobby.id, username);
      
      // Ensure WebSocket is connected (reuse existing if available)
      if (!wsService.ws || wsService.ws.readyState !== WebSocket.OPEN) {
        if (wsService.ws?.readyState !== WebSocket.CONNECTING) {
          wsService.connect();
        }
      }
      
      // Join via WebSocket (will wait for connection if needed)
      wsService.joinLobby(createdLobby.id, username);
      
      navigate(`/game/${createdLobby.id}`, {
        state: {
          lobby: joinResult.lobby,
          player: joinResult.player,
        },
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const copyLobbyId = () => {
    navigator.clipboard.writeText(createdLobby.id);
    alert('Lobby ID copied to clipboard!');
  };

  const handleJoinLobby = async (targetLobbyId) => {
    if (!username) {
      setError('Please enter your username');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const result = await api.joinLobby(targetLobbyId, username);
      
      // Ensure WebSocket is connected (reuse existing if available)
      if (!wsService.ws || wsService.ws.readyState !== WebSocket.OPEN) {
        if (wsService.ws?.readyState !== WebSocket.CONNECTING) {
          wsService.connect();
        }
      }
      
      // Join via WebSocket (will wait for connection if needed)
      wsService.joinLobby(targetLobbyId, username);
      
      navigate(`/game/${targetLobbyId}`, {
        state: {
          lobby: result.lobby,
          player: result.player,
        },
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleJoinById = async (e) => {
    e.preventDefault();
    if (!lobbyId || !username) {
      setError('Please enter lobby ID and username');
      return;
    }
    await handleJoinLobby(lobbyId);
  };

  const handleJoinRandom = async () => {
    if (!username) {
      setError('Please enter your username first');
      return;
    }
    if (lobbies.length === 0) {
      setError('No available lobbies. Create one or wait for others to create lobbies.');
      return;
    }
    // Pick a random lobby from available lobbies
    const randomLobby = lobbies[Math.floor(Math.random() * lobbies.length)];
    await handleJoinLobby(randomLobby.id);
  };

  if (screen === 'home') {
    return (
      <div className="lobby-screen">
        <div className="lobby-container">
          <h1>Quiz Game</h1>
          <div className="form-group">
            <label htmlFor="usernameHome">Your Username</label>
            <input
              id="usernameHome"
              type="text"
              placeholder="Enter your username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="input"
            />
          </div>
          
          <div className="button-group">
            <button onClick={() => setScreen('create')} className="btn btn-primary">
              Create Lobby
            </button>
            <button onClick={() => setScreen('join')} className="btn btn-secondary">
              Join Lobby
            </button>
          </div>

          {error && <div className="error">{error}</div>}

          <div className="lobbies-list">
            <div className="lobbies-header">
              <h2>Available Lobbies</h2>
              <div className="lobbies-actions">
                <button onClick={loadLobbies} className="btn btn-small btn-secondary">
                  Refresh
                </button>
                <button 
                  onClick={handleJoinRandom} 
                  className="btn btn-small btn-primary"
                  disabled={loading || lobbies.filter(l => l.state === 'waiting').length === 0}
                >
                  Join Random
                </button>
              </div>
            </div>
            {lobbies.length === 0 ? (
              <div className="no-lobbies">
                <p>No lobbies available</p>
                <p className="hint">Create a lobby or wait for others to create one!</p>
              </div>
            ) : (
              <ul className="lobbies-grid">
                {lobbies.map((lobby) => (
                  <li key={lobby.id} className="lobby-item">
                    <div className="lobby-item-content">
                      <div className="lobby-item-header">
                        <strong className="lobby-name">{lobby.name}</strong>
                        <span className="lobby-status lobby-status-waiting">
                          Waiting
                        </span>
                      </div>
                      <div className="lobby-item-details">
                        <span className="lobby-players">
                          {lobby.players?.length || 0}/8 players
                        </span>
                        <span className="lobby-rounds">
                          {lobby.max_rounds || 10} questions
                        </span>
                      </div>
                      {lobby.players && lobby.players.length > 0 && (
                        <div className="lobby-players-list">
                          <span className="players-label">Players:</span>
                          <div className="players-badges">
                            {lobby.players.slice(0, 4).map((p) => (
                              <span key={p.id} className="player-badge-small">
                                {p.username}
                              </span>
                            ))}
                            {lobby.players.length > 4 && (
                              <span className="player-badge-small">
                                +{lobby.players.length - 4} more
                              </span>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                    <button
                      onClick={() => handleJoinLobby(lobby.id)}
                      className="btn btn-primary btn-join"
                      disabled={loading}
                      title="Join this lobby"
                    >
                      Join
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (screen === 'create') {
    return (
      <div className="lobby-screen">
        <div className="lobby-container">
          <h1>Create Lobby</h1>
          <form onSubmit={handleCreateLobby}>
            <div className="form-group">
              <label htmlFor="lobbyName">Lobby Name</label>
              <input
                id="lobbyName"
                type="text"
                placeholder="Enter lobby name"
                value={lobbyName}
                onChange={(e) => setLobbyName(e.target.value)}
                className="input"
                required
              />
            </div>
            <div className="form-group">
              <label htmlFor="maxRounds">Number of Questions</label>
              <select
                id="maxRounds"
                value={maxRounds}
                onChange={(e) => setMaxRounds(parseInt(e.target.value))}
                className="input"
              >
                {[5, 10, 15, 20, 25, 30].map((num) => (
                  <option key={num} value={num}>
                    {num} questions
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label htmlFor="username">Your Username</label>
              <input
                id="username"
                type="text"
                placeholder="Enter your username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input"
                required
              />
            </div>
            <div className="button-group">
              <button type="submit" className="btn btn-primary" disabled={loading}>
                {loading ? 'Creating...' : 'Create & Join'}
              </button>
              <button
                type="button"
                onClick={() => setScreen('home')}
                className="btn btn-secondary"
              >
                Back
              </button>
            </div>
            {error && <div className="error">{error}</div>}
          </form>
        </div>
      </div>
    );
  }

  if (screen === 'join') {
    return (
      <div className="lobby-screen">
        <div className="lobby-container">
          <h1>Join Lobby</h1>
          <form onSubmit={handleJoinById}>
            <div className="form-group">
              <label htmlFor="lobbyId">Lobby ID</label>
              <input
                id="lobbyId"
                type="text"
                placeholder="Enter lobby ID"
                value={lobbyId}
                onChange={(e) => setLobbyId(e.target.value)}
                className="input"
                required
              />
            </div>
            <div className="form-group">
              <label htmlFor="usernameJoin">Your Username</label>
              <input
                id="usernameJoin"
                type="text"
                placeholder="Enter your username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input"
                required
              />
            </div>
            <div className="button-group">
              <button type="submit" className="btn btn-primary" disabled={loading}>
                {loading ? 'Joining...' : 'Join'}
              </button>
              <button
                type="button"
                onClick={() => setScreen('home')}
                className="btn btn-secondary"
              >
                Back
              </button>
            </div>
            {error && <div className="error">{error}</div>}
          </form>
        </div>
      </div>
    );
  }

  if (screen === 'created' && createdLobby) {
    return (
      <div className="lobby-screen">
        <div className="lobby-container">
          <h1>Lobby Created!</h1>
          <div className="lobby-info">
            <p><strong>Lobby Name:</strong> {createdLobby.name}</p>
            <div className="lobby-id-section">
              <p><strong>Lobby ID:</strong></p>
              <div className="lobby-id-box">
                <code className="lobby-id">{createdLobby.id}</code>
                <button onClick={copyLobbyId} className="btn btn-small btn-secondary">
                  Copy
                </button>
              </div>
              <p className="lobby-id-hint">Share this ID with friends so they can join!</p>
            </div>
          </div>
          <div className="button-group">
            <button onClick={handleJoinCreatedLobby} className="btn btn-primary" disabled={loading}>
              {loading ? 'Joining...' : 'Join Lobby'}
            </button>
            <button
              onClick={() => setScreen('home')}
              className="btn btn-secondary"
            >
              Back to Home
            </button>
          </div>
          {error && <div className="error">{error}</div>}
        </div>
      </div>
    );
  }
}

