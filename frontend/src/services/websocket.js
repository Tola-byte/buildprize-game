class WebSocketService {
  constructor() {
    this.ws = null;
    this.listeners = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.connectionStatus = 'disconnected'; // 'connected', 'disconnected', 'connecting', 'error'
    this.isReconnecting = false;
  }

  getConnectionStatus() {
    // Check actual WebSocket state to ensure status is accurate
    if (this.ws) {
      const readyState = this.ws.readyState;
      if (readyState === WebSocket.OPEN) {
        return 'connected';
      } else if (readyState === WebSocket.CONNECTING) {
        return 'connecting';
      } else if (readyState === WebSocket.CLOSING || readyState === WebSocket.CLOSED) {
        // If closed/closing, check if we're attempting to reconnect
        if (this.isReconnecting) {
          return 'connecting';
        }
        return this.connectionStatus === 'error' ? 'error' : 'disconnected';
      }
    }
    // Fallback to internal status if no WebSocket exists
    return this.connectionStatus;
  }

  getWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Use the same host as the current page, but default to port 8080 for backend
    let host = window.location.host;
    
    // If on localhost, use port 8080 for backend
    if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
      const port = window.location.port ? window.location.port : (protocol === 'wss:' ? '443' : '80');
      // Use backend port 8080
      host = `${window.location.hostname}:8080`;
    }
    
    return `${protocol}//${host}/ws`;
  }

  connect() {
    // CRITICAL: Prevent duplicate connections - check ALL states first
    if (this.ws) {
      const state = this.ws.readyState;
      if (state === WebSocket.OPEN) {
        console.log('WebSocket already connected, updating status');
        // Ensure status is correct if already connected
        if (this.connectionStatus !== 'connected') {
          this.connectionStatus = 'connected';
          this.emit('connected');
          this.emit('status', { status: this.connectionStatus });
        }
        return;
      }
      if (state === WebSocket.CONNECTING) {
        console.log('WebSocket already connecting, skipping duplicate connection');
        return;
      }
      // Only proceed if CLOSED or CLOSING
      if (state === WebSocket.CLOSING) {
        console.log('WebSocket is closing, waiting...');
        return;
      }
      // If CLOSED, clean it up
      if (state === WebSocket.CLOSED) {
        try {
          this.ws.close();
        } catch (e) {
          // Ignore errors
        }
        this.ws = null;
      }
    }

    // Don't reconnect if already attempting
    if (this.isReconnecting) {
      console.log('WebSocket reconnection already in progress, skipping');
      return;
    }

    this.connectionStatus = 'connecting';
    this.emit('status', { status: this.connectionStatus });
    
    const wsUrl = this.getWebSocketUrl();
    console.log('Connecting to WebSocket:', wsUrl);
    console.log('Current WebSocket state before connect:', this.ws?.readyState);
    
    try {
      this.ws = new WebSocket(wsUrl);
      console.log('WebSocket instance created, readyState:', this.ws.readyState);

      // Set a connection timeout - increased to 30s to avoid premature timeouts
      const connectionStartTime = Date.now();
      const connectionTimeout = setTimeout(() => {
        if (this.ws && this.ws.readyState !== WebSocket.OPEN) {
          const elapsed = Date.now() - connectionStartTime;
          console.error(`⏱️ WebSocket connection timeout after ${elapsed}ms (30s limit)`);
          console.error('   Current state:', this.ws.readyState);
          console.error('   WebSocket URL was:', wsUrl);
          console.error('   Possible causes:');
          console.error('   1. Server not responding');
          console.error('   2. Network firewall/proxy blocking connection');
          console.error('   3. Server overloaded');
          console.error('   4. Incorrect WebSocket URL');
          if (this.ws) {
            this.ws.close();
          }
          this.connectionStatus = 'error';
          this.emit('status', { status: this.connectionStatus });
          this.attemptReconnect();
        }
      }, 30000); // 30 second timeout (was 10s - too short)

      this.ws.onopen = () => {
        clearTimeout(connectionTimeout);
        const connectTime = Date.now();
        console.log('✅ WebSocket connected successfully');
        console.log(`   Connection established in ${connectTime}ms`);
        this.reconnectAttempts = 0;
        this.isReconnecting = false;
        this.connectionStatus = 'connected';
        this.emit('connected');
        this.emit('status', { status: this.connectionStatus });
        
        // Log connection health periodically
        let messageCount = 0;
        const healthCheck = setInterval(() => {
          if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            messageCount++;
            if (messageCount % 20 === 0) {
              console.log(`✅ WebSocket connection healthy (${messageCount * 30}s uptime)`);
            }
          } else {
            clearInterval(healthCheck);
          }
        }, 30000); // Every 30 seconds
      };

      this.ws.onmessage = (event) => {
        // Browsers handle ping/pong automatically, so we only need to handle text messages
        if (typeof event.data === 'string') {
          try {
            const data = JSON.parse(event.data);
            console.log('WebSocket message received:', data.type, data);
            this.emit(data.type, data);
          } catch (error) {
            console.error('Failed to parse WebSocket message:', error, event.data);
          }
        }
      };

      this.ws.onerror = (error) => {
        clearTimeout(connectionTimeout);
        console.error('❌ WebSocket error event:', error);
        console.error('   WebSocket readyState:', this.ws?.readyState);
        console.error('   Error details:', {
          type: error.type,
          target: error.target?.readyState,
          timestamp: new Date().toISOString()
        });
        // Set error status on error
        this.connectionStatus = 'error';
        this.emit('status', { status: this.connectionStatus });
        
        // Don't attempt reconnect here - wait for onclose to handle it
        // The onclose handler will determine if we should reconnect
      };

      this.ws.onclose = (event) => {
        clearTimeout(connectionTimeout);
        console.log('WebSocket disconnected', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
          timestamp: new Date().toISOString()
        });
        
        // Map close codes to status
        if (event.code === 1000) {
          // Normal closure - client/server initiated clean close
          console.log('WebSocket closed normally (1000)');
          this.connectionStatus = 'disconnected';
        } else if (event.code === 1001) {
          // Going away - server closing or client navigating away
          console.log('WebSocket going away (1001)');
          this.connectionStatus = 'disconnected';
        } else if (event.code === 1006) {
          // Abnormal closure - connection lost without close frame
          console.error('❌ WebSocket connection lost (1006) - abnormal closure');
          console.error('   Possible causes:');
          console.error('   1. Network interruption (WiFi disconnect, mobile data loss)');
          console.error('   2. Server crashed or restarted');
          console.error('   3. Firewall/proxy blocking connection');
          console.error('   4. Read deadline timeout (no pong response for 60s)');
          this.connectionStatus = 'error';
        } else {
          console.error(`❌ WebSocket closed with code ${event.code}: ${event.reason}`);
          console.error(`   Close code ${event.code} details:`);
          if (event.code === 1001) {
            console.error('   - 1001: Going away (server closing or navigated away)');
          } else if (event.code === 1002) {
            console.error('   - 1002: Protocol error');
          } else if (event.code === 1003) {
            console.error('   - 1003: Unsupported data type');
          } else if (event.code === 1005) {
            console.error('   - 1005: No status code (abnormal closure)');
          } else if (event.code >= 1015) {
            console.error('   - 1015+: TLS handshake failed or extension error');
          }
          this.connectionStatus = 'error';
        }
        
        this.emit('disconnected');
        this.emit('status', { status: this.connectionStatus });
        
        // Only attempt reconnect if not a clean close and not already reconnecting
        // Don't reconnect on 1000 (normal) or 1001 (going away)
        if (event.code !== 1000 && event.code !== 1001 && !this.isReconnecting) {
          console.log('Attempting to reconnect...');
          this.attemptReconnect();
        }
      };
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
      this.connectionStatus = 'error';
      this.emit('error', error);
      this.emit('status', { status: this.connectionStatus });
      this.attemptReconnect();
    }
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnection attempts reached');
      this.isReconnecting = false;
      this.connectionStatus = 'error';
      this.emit('status', { status: this.connectionStatus });
      return;
    }

    if (!this.isReconnecting) {
      this.isReconnecting = true;
      this.reconnectAttempts++;
      const delay = Math.min(1000 * this.reconnectAttempts, 5000); // Max 5 second delay
      console.log(`Scheduling reconnect in ${delay}ms... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
      
      setTimeout(() => {
        if (this.isReconnecting) { // Only reconnect if still needed
          console.log(`Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
          this.connectionStatus = 'connecting';
          this.emit('status', { status: this.connectionStatus });
          this.connect();
        }
      }, delay);
    }
  }

  send(type, data = {}) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, ...data }));
    } else {
      console.error('WebSocket is not connected');
    }
  }

  joinLobby(lobbyId, username) {
    // Wait for connection if not ready
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.log('WebSocket not ready, waiting...');
      const handleConnected = () => {
        this.off('connected', handleConnected);
        this.send('join_lobby', {
          lobby_id: lobbyId,
          data: { username },
        });
      };
      this.on('connected', handleConnected);
      // Connect if not already connecting
      if (!this.ws || this.ws.readyState === WebSocket.CLOSED) {
        this.connect();
      }
      return;
    }
    
    this.send('join_lobby', {
      lobby_id: lobbyId,
      data: { username },
    });
  }

  leaveLobby(lobbyId, playerId) {
    // Only send if WebSocket is open
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.send('leave_lobby', { 
        lobby_id: lobbyId,
        player_id: playerId 
      });
    } else {
      console.log('WebSocket not open, skipping leave_lobby message');
    }
  }

  startGame(lobbyId) {
    this.send('start_game', { lobby_id: lobbyId });
  }

  sendChatMessage(lobbyId, playerId, message) {
    console.log('Sending chat message:', { lobbyId, playerId, message });
    console.log('WebSocket state:', this.ws?.readyState, this.ws ? 'OPEN=' + WebSocket.OPEN : 'no ws');
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.error('Cannot send chat message: WebSocket not connected');
      return;
    }
    const messageData = {
      type: 'chat_message',
      lobby_id: lobbyId,
      player_id: playerId,
      data: { message },
    };
    console.log('Sending WebSocket message:', messageData);
    this.ws.send(JSON.stringify(messageData));
  }

  submitAnswer(lobbyId, playerId, answer, responseTime) {
    this.send('submit_answer', {
      lobby_id: lobbyId,
      player_id: playerId,
      data: { answer, response_time: responseTime },
    });
  }

  // Also support submitting via REST API (via WebSocket for real-time)
  submitAnswerViaAPI(lobbyId, playerId, answer, responseTime) {
    // This is handled by REST API, WebSocket just for receiving updates
    return fetch(`http://localhost:8080/api/v1/lobbies/${lobbyId}/answer`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        player_id: playerId,
        answer,
        response_time: responseTime,
      }),
    });
  }

  on(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    this.listeners.get(event).push(callback);
  }

  off(event, callback) {
    if (this.listeners.has(event)) {
      const callbacks = this.listeners.get(event);
      const index = callbacks.indexOf(callback);
      if (index > -1) {
        callbacks.splice(index, 1);
      }
    }
  }

  emit(event, data) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).forEach((callback) => callback(data));
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'Client disconnecting');
      this.ws = null;
    }
    this.isReconnecting = false;
    this.reconnectAttempts = 0;
    this.connectionStatus = 'disconnected';
    this.emit('status', { status: this.connectionStatus });
    this.listeners.clear();
  }
}

export const wsService = new WebSocketService();

