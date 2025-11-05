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
    // Don't disconnect on unmount - let GameScreen use the same connection
    // return () => wsService.disconnect();
  }, []);

  const loadLobbies = async () => {
    try {
      const data = await api.listLobbies();
      setLobbies(data.filter(l => l.state === 'waiting'));
    } catch (err) {
      console.error('Failed to load lobbies:', err);
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

  if (screen === 'home') {
    return (
      <div className="lobby-screen">
        <div className="lobby-container">
          <h1>🎮 Quiz Game</h1>
          <div className="username-input">
            <input
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
            <h2>Available Lobbies</h2>
            <button onClick={loadLobbies} className="btn-small">Refresh</button>
            {lobbies.length === 0 ? (
              <p>No lobbies available</p>
            ) : (
              <ul>
                {lobbies.map((lobby) => (
                  <li key={lobby.id} className="lobby-item">
                    <div>
                      <strong>{lobby.name}</strong>
                      <span>{lobby.players?.length || 0}/8 players</span>
                    </div>
                    <button
                      onClick={() => handleJoinLobby(lobby.id)}
                      className="btn btn-small"
                      disabled={loading}
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
            <input
              type="text"
              placeholder="Lobby name"
              value={lobbyName}
              onChange={(e) => setLobbyName(e.target.value)}
              className="input"
              required
            />
            <input
              type="number"
              placeholder="Max rounds (default: 10)"
              value={maxRounds}
              onChange={(e) => setMaxRounds(parseInt(e.target.value) || 10)}
              className="input"
              min="1"
              max="50"
            />
            <input
              type="text"
              placeholder="Your username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="input"
              required
            />
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
            <input
              type="text"
              placeholder="Lobby ID"
              value={lobbyId}
              onChange={(e) => setLobbyId(e.target.value)}
              className="input"
              required
            />
            <input
              type="text"
              placeholder="Your username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="input"
              required
            />
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
          <h1>✅ Lobby Created!</h1>
          <div className="lobby-info">
            <p><strong>Lobby Name:</strong> {createdLobby.name}</p>
            <div className="lobby-id-section">
              <p><strong>Lobby ID:</strong></p>
              <div className="lobby-id-box">
                <code className="lobby-id">{createdLobby.id}</code>
                <button onClick={copyLobbyId} className="btn btn-small btn-secondary">
                  📋 Copy
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

