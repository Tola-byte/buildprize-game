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
  const [timeLeft, setTimeLeft] = useState(30);
  const [questionStartTime, setQuestionStartTime] = useState(null);
  const [showResults, setShowResults] = useState(false);
  const [correctAnswer, setCorrectAnswer] = useState(null);
  
  const timerRef = useRef(null);
  const questionTimerRef = useRef(null);

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
    const questionData = data.data.question;
    setQuestion(questionData);
    setSelectedAnswer(null);
    setAnswered(false);
    setShowResults(false);
    setTimeLeft(30);
    setQuestionStartTime(Date.now());
    
    // Start timer
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
                  {question.options.map((option, index) => (
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
                  ))}
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
        <Leaderboard players={lobby.players || []} />
      </div>
    </div>
  );
}

