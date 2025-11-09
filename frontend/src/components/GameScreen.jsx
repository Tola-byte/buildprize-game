import { useState, useEffect, useRef } from 'react';
import { useParams, useLocation, useNavigate } from 'react-router-dom';
import { api } from '../services/api';
import { wsService } from '../services/websocket';
import Leaderboard from './Leaderboard';
import ConnectionStatus from './ConnectionStatus';
import './GameScreen.css';

export default function GameScreen() {
  const { lobbyId } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  
  const [lobby, setLobby] = useState(location.state?.lobby || null);
  const [player, setPlayer] = useState(location.state?.player || null);
  const [question, setQuestion] = useState(null);
  const [selectedAnswer, setSelectedAnswer] = useState(null);
  const [answered, setAnswered] = useState(false);
  const [timeLeft, setTimeLeft] = useState(15);
  const [questionStartTime, setQuestionStartTime] = useState(null);
  const [showResults, setShowResults] = useState(false);
  const [correctAnswer, setCorrectAnswer] = useState(null);
  const [chatMessages, setChatMessages] = useState([]);
  const [chatInput, setChatInput] = useState('');
  const [showChat, setShowChat] = useState(true);
  
  const timerRef = useRef(null);
  const questionTimerRef = useRef(null);
  const questionEndTimeRef = useRef(null); // Store server's absolute end time
  const serverTimeOffsetRef = useRef(0); // Store clock offset for sync
  const chatEndRef = useRef(null);

  useEffect(() => {
    if (!lobby || !player) {
      // Try to load from API
      loadLobby();
    } else {
      setLobby(location.state.lobby);
      setPlayer(location.state.player);
    }

    // Connect WebSocket if not connected and not already connecting
    if (!wsService.ws || wsService.ws.readyState === WebSocket.CLOSED) {
      // Only connect if not already connecting or open
      if (wsService.ws?.readyState !== WebSocket.CONNECTING && 
          wsService.ws?.readyState !== WebSocket.OPEN) {
        wsService.connect();
      }
    }

    // Set up WebSocket listeners
    wsService.on('player_joined', handlePlayerJoined);
    wsService.on('player_left', handlePlayerLeft);
    wsService.on('game_started', handleGameStarted);
    wsService.on('new_question', handleNewQuestion);
    wsService.on('answer_received', handleAnswerReceived);
    wsService.on('question_results', handleQuestionResults);
    wsService.on('game_ended', handleGameEnded);
    wsService.on('lobby_updated', handlePlayerJoined); // Also listen for lobby updates
    wsService.on('chat_message', handleChatMessage);

    // Join lobby via WebSocket when connection is ready
    const joinWhenReady = () => {
      if (lobby && player) {
        if (wsService.ws && wsService.ws.readyState === WebSocket.OPEN) {
          wsService.joinLobby(lobby.id, player.username);
        } else {
          // Wait for connection
          const handleConnected = () => {
            wsService.off('connected', handleConnected);
            if (lobby && player) {
              wsService.joinLobby(lobby.id, player.username);
            }
          };
          wsService.on('connected', handleConnected);
        }
      }
    };

    // Wait a bit for connection to establish
    setTimeout(() => {
      joinWhenReady();
    }, 100);

    return () => {
      wsService.off('player_joined', handlePlayerJoined);
      wsService.off('player_left', handlePlayerLeft);
      wsService.off('game_started', handleGameStarted);
      wsService.off('new_question', handleNewQuestion);
      wsService.off('answer_received', handleAnswerReceived);
      wsService.off('question_results', handleQuestionResults);
      wsService.off('game_ended', handleGameEnded);
      wsService.off('chat_message', handleChatMessage);
      if (timerRef.current) clearInterval(timerRef.current);
      if (questionTimerRef.current) clearInterval(questionTimerRef.current);
      // Don't disconnect WebSocket - keep it alive for navigation
    };
  }, []);

  const loadLobby = async () => {
    try {
      const lobbyData = await api.getLobby(lobbyId);
      setLobby(lobbyData);
    } catch (err) {
      console.error('Failed to load lobby:', err);
    }
  };

  const handlePlayerJoined = (data) => {
    console.log('Player joined event received:', data);
    if (data.data && data.data.lobby) {
      setLobby(data.data.lobby);
    }
  };

  const handlePlayerLeft = (data) => {
    console.log('Player left event received:', data);
    if (data.data && data.data.lobby) {
      setLobby(data.data.lobby);
      
      // If current player left, navigate away
      if (data.data.player_id === player?.id) {
        console.log('You left the lobby, navigating away...');
        navigate('/');
      }
    }
  };

  const handleNewQuestion = (data) => {
    console.log('New question event received:', data);
    const questionData = data.data.question;
    console.log('Question data:', questionData);
    console.log('Question options:', questionData?.options);
    
    // Ensure question has options array
    if (!questionData || !questionData.options || !Array.isArray(questionData.options)) {
      console.error('Invalid question data received:', questionData);
      return;
    }
    
    setQuestion(questionData);
    setSelectedAnswer(null);
    setAnswered(false);
    setShowResults(false);
    
    // FIX: Use server timestamp for synchronized timer
    // Store server's absolute end time and calculate remaining time from it
    // This ensures all clients are synchronized even if they receive the message at different times
    const questionEndTime = data.data.question_end_time; // Server timestamp in milliseconds (absolute time)
    const serverTime = data.data.server_time; // Server's current time when message was sent
    const clientTime = Date.now(); // Client's current time when message received
    
    if (questionEndTime && serverTime) {
      // Calculate clock offset: how much ahead/behind client clock is vs server
      // serverTime was server's time when message was sent
      // clientTime is client's time when message was received
      // Offset = clientTime - serverTime (accounts for network delay + clock difference)
      serverTimeOffsetRef.current = clientTime - serverTime;
      
      // Store the server's absolute end time (this is the source of truth for all clients)
      questionEndTimeRef.current = questionEndTime;
      
      // Calculate remaining time function - always calculates from server's absolute end time
      // This ensures all clients show the same countdown regardless of when they received the message
      const calculateRemainingTime = () => {
        const now = Date.now();
        // Estimate current server time based on our clock offset
        const estimatedServerTimeNow = now - serverTimeOffsetRef.current;
        // Calculate remaining seconds until server's absolute end time
        const remainingMs = questionEndTimeRef.current - estimatedServerTimeNow;
        const remaining = Math.max(0, Math.floor(remainingMs / 1000));
        return remaining;
      };
      
      // Set initial time immediately (don't wait for first interval)
      const initialTime = calculateRemainingTime();
      setTimeLeft(initialTime);
      setQuestionStartTime(Date.now() - (15 - initialTime) * 1000);
      
      // Clear any existing timer
      if (questionTimerRef.current) clearInterval(questionTimerRef.current);
    
      // Start synchronized timer - updates very frequently for smooth, accurate countdown
      // All clients calculate from the same server end time, so they stay in sync
      questionTimerRef.current = setInterval(() => {
        const remaining = calculateRemainingTime();
        setTimeLeft(remaining);
        if (remaining <= 0) {
          clearInterval(questionTimerRef.current);
          setTimeLeft(0);
        }
      }, 50); // Update every 50ms for smooth, accurate countdown
    } else {
      // Fallback to old behavior if server doesn't send timestamp
      questionEndTimeRef.current = null;
      setTimeLeft(15);
      setQuestionStartTime(Date.now());
    if (questionTimerRef.current) clearInterval(questionTimerRef.current);
    questionTimerRef.current = setInterval(() => {
      setTimeLeft((prev) => {
        if (prev <= 1) {
          clearInterval(questionTimerRef.current);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    }
  };

  const handleAnswerReceived = (data) => {
    // Update lobby state if needed
    if (data.data.lobby) {
      setLobby(data.data.lobby);
    }
  };

  const handleQuestionResults = (data) => {
    setShowResults(true);
    setCorrectAnswer(data.data.correct_answer);
    setLobby((prev) => ({
      ...prev,
      players: data.data.leaderboard || prev.players,
    }));
    if (questionTimerRef.current) clearInterval(questionTimerRef.current);
  };

  const handleGameStarted = (data) => {
    console.log('Game started event received:', data);
    if (data.data && data.data.lobby) {
      setLobby(data.data.lobby);
      // Game will transition to in_progress state, new_question will be sent next
    }
  };

  const handleGameEnded = (data) => {
    setShowResults(true);
    setLobby((prev) => ({
      ...prev,
      state: 'finished',
      players: data.data.final_leaderboard || prev.players,
    }));
  };

  const handleChatMessage = (data) => {
    console.log('Chat message event received:', data);
    const messageData = data.data;
    console.log('Message data:', messageData);
    if (messageData && messageData.username && messageData.message) {
      const messageId = `msg_${messageData.player_id}_${messageData.timestamp}`;
      const isOwnMessage = messageData.player_id === player?.id;
      
      setChatMessages((prev) => {
        // Check if message already exists (to prevent duplicates from optimistic updates)
        const exists = prev.some(msg => msg.id === messageId);
        if (exists) {
          return prev; // Don't add duplicate
        }
        
        // If this is our own message, remove any pending temp messages with same content
        // This replaces the optimistic update with the confirmed server message
        const filtered = prev.filter(msg => {
          if (isOwnMessage && msg.isPending && msg.message === messageData.message) {
            return false; // Remove temp message
          }
          return true;
        });
        
        return [
          ...filtered,
          {
            id: messageId,
            username: messageData.username,
            message: messageData.message,
            timestamp: messageData.timestamp || Date.now(),
            isOwn: isOwnMessage,
          },
        ];
      });
      
      // Auto-scroll to bottom
      setTimeout(() => {
        if (chatEndRef.current) {
          chatEndRef.current.scrollIntoView({ behavior: 'smooth' });
        }
      }, 100);
    }
  };

  const handleSendChat = async (e) => {
    e.preventDefault();
    if (!chatInput.trim() || !lobby || !player) return;

    const messageText = chatInput.trim();
    const timestamp = Date.now();
    const tempId = `temp_${player.id}_${timestamp}`;
    
    // Optimistically add the message immediately (before server confirms)
    setChatMessages((prev) => [
      ...prev,
      {
        id: tempId,
        username: player.username,
        message: messageText,
        timestamp: timestamp,
        isOwn: true,
        isPending: true, // Mark as pending until server confirms
      },
    ]);
    
    // Clear input immediately for better UX
    setChatInput('');
    
    // Auto-scroll to show the new message
    setTimeout(() => {
      if (chatEndRef.current) {
        chatEndRef.current.scrollIntoView({ behavior: 'smooth' });
      }
    }, 50);
    
    // Send to server via REST API (which broadcasts via WebSocket)
    // The server will broadcast it back via WebSocket, and handleChatMessage will replace the temp message
    try {
      // Try REST API first (more reliable)
      await api.sendChatMessage(lobby.id, player.id, messageText);
      
      // Set a timeout to remove pending message if server doesn't respond within 5 seconds
      // This prevents messages from staying stuck as "Sending..." forever
      setTimeout(() => {
        setChatMessages((prev) => {
          const stillPending = prev.find(msg => msg.id === tempId && msg.isPending);
          if (stillPending) {
            console.warn('Chat message still pending after 5s, removing:', stillPending);
            return prev.filter(msg => msg.id !== tempId);
          }
          return prev;
        });
      }, 5000);
    } catch (error) {
      console.error('Failed to send chat message via REST API, trying WebSocket:', error);
      // Fallback to WebSocket if REST API fails
      try {
        wsService.sendChatMessage(lobby.id, player.id, messageText);
      } catch (wsError) {
        console.error('Failed to send chat message via WebSocket:', wsError);
        // Remove the pending message on error
        setChatMessages((prev) => prev.filter(msg => msg.id !== tempId));
      }
    }
  };

  // Auto-scroll chat to bottom when new messages arrive
  useEffect(() => {
    if (chatEndRef.current) {
      chatEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [chatMessages]);

  const handleStartGame = async () => {
    try {
      await api.startGame(lobbyId);
      wsService.startGame(lobbyId);
    } catch (err) {
      console.error('Failed to start game:', err);
    }
  };

  const handleSubmitAnswer = async () => {
    if (selectedAnswer === null || answered) return;

    const responseTime = Date.now() - questionStartTime;
    setAnswered(true);
    
    try {
      // Submit via REST API (WebSocket receives updates automatically)
      await api.submitAnswer(lobbyId, player.id, selectedAnswer, responseTime);
    } catch (err) {
      console.error('Failed to submit answer:', err);
      setAnswered(false);
    }
  };

  const handleLeave = async () => {
    try {
      // Leave via REST API first (this updates the backend state)
      await api.leaveLobby(lobbyId, player.id);
      
      // Also send WebSocket leave message (in case WebSocket is still connected)
      if (wsService.ws && wsService.ws.readyState === WebSocket.OPEN) {
        wsService.leaveLobby(lobbyId, player.id);
      }
      
      // Navigate away after leaving
      navigate('/');
    } catch (err) {
      console.error('Failed to leave lobby:', err);
      // Still try to navigate away even if leave fails
      // (user might have already left or connection might be lost)
      navigate('/');
    }
  };

  if (!lobby || !player) {
    return <div className="game-screen">Loading...</div>;
  }

  const isHost = lobby.players?.[0]?.id === player.id;
  const canStart = lobby.state === 'waiting' && lobby.players?.length >= 2 && isHost;

  return (
    <div className="game-screen">
      <ConnectionStatus />
      <div className="game-header">
        <div>
          <h1>{lobby.name}</h1>
          <p>Round {lobby.round} of {lobby.max_rounds}</p>
        </div>
        <div className="game-info">
          <span>Players: {lobby.players?.length || 0}/8</span>
          <button onClick={handleLeave} className="btn btn-small btn-secondary">
            Leave
          </button>
        </div>
      </div>

      {lobby.state === 'waiting' && (
        <div className="waiting-screen">
          <h2>Waiting for players...</h2>
          <div className="players-list">
            {lobby.players?.map((p) => (
              <div key={p.id} className="player-badge">
                {p.username}
                {p.id === player.id && <span className="you">(You)</span>}
              </div>
            ))}
          </div>
          {canStart && (
            <button onClick={handleStartGame} className="btn btn-primary btn-large">
              Start Game
            </button>
          )}
          {!canStart && isHost && (
            <p className="waiting-message">Need at least 2 players to start</p>
          )}
        </div>
      )}

      {lobby.state === 'in_progress' && question && (
        <div className="question-screen">
          {!showResults ? (
            <>
              <div className="timer">
                <div className="timer-circle">
                  <span>{timeLeft}s</span>
                </div>
              </div>
              
              <div className="question">
                <h2>{question.text}</h2>
                <div className="options">
                  {question.options && Array.isArray(question.options) && question.options.length > 0 ? (
                    question.options.map((option, index) => (
                      <button
                        key={index}
                        onClick={() => !answered && setSelectedAnswer(index)}
                        className={`option-btn ${selectedAnswer === index ? 'selected' : ''} ${
                          answered ? 'disabled' : ''
                        }`}
                        disabled={answered}
                      >
                        {option}
                      </button>
                    ))
                  ) : (
                    <p style={{ color: 'red', padding: '20px' }}>
                      No options available. Question data: {JSON.stringify(question, null, 2)}
                    </p>
                  )}
                </div>
                {selectedAnswer !== null && !answered && (
                  <button onClick={handleSubmitAnswer} className="btn btn-primary btn-large">
                    Submit Answer
                  </button>
                )}
                {answered && (
                  <p className="answer-submitted">Answer submitted! Waiting for others...</p>
                )}
              </div>
            </>
          ) : (
            <div className="results-screen">
              <h2>Results</h2>
              <p className="correct-answer">
                Correct answer: {question.options[correctAnswer]}
              </p>
              <Leaderboard players={lobby.players || []} />
            </div>
          )}
        </div>
      )}

      {lobby.state === 'finished' && (
        <div className="finished-screen">
          <h2>Game Finished!</h2>
          <Leaderboard players={lobby.players || []} />
          <button onClick={() => navigate('/')} className="btn btn-primary btn-large">
            Back to Lobby
          </button>
        </div>
      )}

      <div className="sidebar">
        <div style={{ flexShrink: 0 }}>
        <Leaderboard players={lobby.players || []} />
        </div>
        
        {/* Chat Section */}
        <div className="chat-container">
          <div className="chat-header">
            <h3>💬 Chat</h3>
            <button 
              onClick={() => setShowChat(!showChat)}
              className="chat-toggle-btn"
              aria-label={showChat ? 'Hide chat' : 'Show chat'}
            >
              {showChat ? '−' : '+'}
            </button>
          </div>
          
          {showChat && (
            <>
              <div className="chat-messages">
                {chatMessages.length === 0 ? (
                  <div className="chat-empty">No messages yet. Start the conversation!</div>
                ) : (
                  chatMessages.map((msg) => (
                    <div 
                      key={msg.id} 
                      className={`chat-message ${msg.isOwn ? 'own-message' : 'other-message'} ${msg.isPending ? 'pending' : ''}`}
                    >
                      <div className="chat-message-header">
                        <span className="chat-username">{msg.username}</span>
                        {msg.isOwn && <span className="chat-you-badge">You</span>}
                        <span className="chat-timestamp">
                          {new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                        </span>
                      </div>
                      <div className="chat-message-text">{msg.message}</div>
                      {msg.isPending && <div className="chat-pending-indicator">Sending...</div>}
                    </div>
                  ))
                )}
                <div ref={chatEndRef} />
              </div>
              
              <form onSubmit={handleSendChat} className="chat-input-form">
                <input
                  type="text"
                  value={chatInput}
                  onChange={(e) => setChatInput(e.target.value)}
                  placeholder="Type a message..."
                  className="chat-input"
                  maxLength={200}
                />
                <button 
                  type="submit" 
                  className="chat-send-btn"
                  disabled={!chatInput.trim()}
                >
                  Send
                </button>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

