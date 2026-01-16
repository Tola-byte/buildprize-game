# BuildPrize Game - Architecture Documentation

## Executive Summary

BuildPrize is a real-time multiplayer quiz game built with Go (backend) and React (frontend). The system uses WebSocket connections for real-time gameplay, PostgreSQL for persistence, and implements a hub-based architecture for managing concurrent game sessions.

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Browser 1  │  │   Browser 2  │  │   Browser N  │     │
│  │   (React)    │  │   (React)    │  │   (React)    │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
│         │                  │                  │              │
│         └──────────────────┼──────────────────┘              │
│                            │                                  │
└────────────────────────────┼──────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │   HTTP/WS API    │
                    │   (Port 8080)    │
                    └────────┬─────────┘
                             │
┌────────────────────────────┼──────────────────────────────────┐
│                    APPLICATION LAYER                         │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Gin HTTP Router                         │   │
│  │  ┌──────────────┐  ┌──────────────┐                │   │
│  │  │ REST API     │  │ WebSocket    │                │   │
│  │  │ Endpoints    │  │ Handler      │                │   │
│  │  └──────┬───────┘  └──────┬───────┘                │   │
│  └─────────┼──────────────────┼────────────────────────┘   │
│            │                  │                              │
│  ┌─────────▼──────────────────▼─────────┐                 │
│  │         Game Service                   │                 │
│  │  - Create/Join/Leave Lobby            │                 │
│  │  - Start Game                          │                 │
│  │  - Submit Answers                     │                 │
│  │  - Calculate Scores                    │                 │
│  └─────────┬──────────────────┬──────────┘                 │
│            │                  │                              │
│  ┌─────────▼──────────┐  ┌───▼──────────────┐            │
│  │   Hub Manager      │  │  Repository      │            │
│  │  - Lobby Hubs      │  │  - PostgreSQL    │            │
│  │  - WebSocket Clients│  │  - CRUD Ops     │            │
│  └────────────────────┘  └──────────────────┘            │
└─────────────────────────────────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │   PostgreSQL     │
                    │   (Port 5432)    │
                    └──────────────────┘
```

## Component Architecture

### 1. Backend Architecture (Go)

#### 1.1 Entry Point (`main.go`)
```
┌─────────────┐
│  main.go    │
│             │
│  1. Load    │
│     Config  │
│  2. Create  │
│     Server  │
│  3. Start   │
│     Server  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Server    │
└─────────────┘
```

**Responsibilities:**
- Application bootstrap
- Configuration loading
- Server initialization

#### 1.2 Server Layer (`internal/server/server.go`)

```
┌─────────────────────────────────────────┐
│           Server Struct                  │
│  ┌───────────────────────────────────┐  │
│  │  - config: Config                 │  │
│  │  - hub: Hub                       │  │
│  │  - gameService: GameService       │  │
│  │  - router: Gin Engine            │  │
│  │  - upgrader: WebSocket Upgrader   │  │
│  └───────────────────────────────────┘  │
│                                          │
│  Routes:                                 │
│  ├─ REST API (/api/v1/*)                │
│  │  ├─ POST /lobbies                   │
│  │  ├─ GET /lobbies                    │
│  │  ├─ POST /lobbies/:id/join          │
│  │  ├─ POST /lobbies/:id/start         │
│  │  └─ POST /lobbies/:id/answer        │
│  │                                      │
│  └─ WebSocket (/ws)                     │
│     └─ handleWebSocket()                │
└─────────────────────────────────────────┘
```

**Key Features:**
- RESTful API for lobby management
- WebSocket upgrade handler
- CORS middleware
- Static file serving (for frontend)

#### 1.3 Hub System (`internal/hub/hub.go`)

The hub system manages WebSocket connections using a **Hub Pattern**:

```
┌──────────────────────────────────────────────┐
│              Hub (Singleton)                 │
│  ┌────────────────────────────────────────┐ │
│  │  lobbies: map[string]*LobbyHub         │ │
│  │  mu: sync.RWMutex                      │ │
│  └────────────────────────────────────────┘ │
│                                               │
│  Methods:                                     │
│  - GetLobbyHub(lobbyID)                      │
│  - CreateLobbyHub(lobby)                      │
│  - RemoveLobbyHub(lobbyID)                    │
└───────────────────┬───────────────────────────┘
                    │
        ┌───────────┼───────────┐
        │           │           │
        ▼           ▼           ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ LobbyHub 1 │ │ LobbyHub 2 │ │ LobbyHub N │
│            │ │            │ │            │
│ lobby: *Lobby│ │ lobby: *Lobby│ │ lobby: *Lobby│
│ clients: map│ │ clients: map│ │ clients: map│
│ register: chan│ │ register: chan│ │ register: chan│
│ broadcast: chan│ │ broadcast: chan│ │ broadcast: chan│
│            │ │            │ │            │
│ ┌────────┐ │ │ ┌────────┐ │ │ ┌────────┐ │
│ │Client 1│ │ │ │Client 1│ │ │ │Client 1│ │
│ │Client 2│ │ │ │Client 2│ │ │ │Client 2│ │
│ │Client N│ │ │ │Client N│ │ │ │Client N│ │
│ └────────┘ │ │ └────────┘ │ │ └────────┘ │
└─────────────┘ └─────────────┘ └─────────────┘
```

**LobbyHub Lifecycle:**

```
1. Lobby Created
   └─> CreateLobbyHub()
       └─> Start goroutine: run()
           └─> Event Loop:
               ├─ register channel
               ├─ unregister channel
               ├─ broadcast channel
               └─ ticker (1s)

2. Client Connects
   └─> Register(client)
       └─> Add to clients map
           └─> Send to register channel

3. Broadcast Message
   └─> Broadcast(data)
       └─> Send to broadcast channel
           └─> Loop through clients
               └─> Send to each client.Send channel

4. Client Disconnects
   └─> Unregister(client)
       └─> Remove from clients map
           └─> Close client.Send channel
```

**Concurrency Model:**
- Each `LobbyHub` runs in its own goroutine
- Uses channels for thread-safe communication
- `sync.RWMutex` for map access protection
- Non-blocking channel sends (with `select` default)

#### 1.4 Game Service (`internal/services/game_service.go`)

```
┌─────────────────────────────────────────┐
│         GameService                     │
│  ┌───────────────────────────────────┐  │
│  │  - hub: *Hub                      │  │
│  │  - repo: Repository               │  │
│  │  - questionDB: QuestionDatabase    │  │
│  └───────────────────────────────────┘  │
│                                          │
│  Core Methods:                           │
│  ├─ CreateLobby(name, maxRounds)         │
│  ├─ JoinLobby(lobbyID, username)        │
│  ├─ LeaveLobby(lobbyID, playerID)        │
│  ├─ StartGame(lobbyID)                   │
│  ├─ SubmitAnswer(lobbyID, playerID, ...) │
│  └─ CalculateScore(...)                  │
└─────────────────────────────────────────┘
```

**Game Flow:**

```
┌─────────────┐
│ Create Lobby│
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Join Lobby  │
└──────┬──────┘
       │
       ▼
┌─────────────┐      ┌─────────────┐
│ Start Game  │─────>│ Round 1     │
└─────────────┘      └──────┬──────┘
                            │
                            ▼
                    ┌─────────────┐
                    │ Send Question│
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ Collect     │
                    │ Answers     │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ Calculate  │
                    │ Scores     │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ Next Round?│
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    │             │
                    ▼             ▼
            ┌───────────┐  ┌───────────┐
            │ Next Round│  │ Game End  │
            └───────────┘  └───────────┘
```

#### 1.5 Repository Pattern (`internal/repository/`)

```
┌─────────────────────────────────────────┐
│         Repository Interface            │
│  ┌───────────────────────────────────┐  │
│  │  - SaveLobby(lobby)                │  │
│  │  - GetLobby(id)                    │  │
│  │  - ListLobbies()                   │  │
│  │  - DeleteLobby(id)                 │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
                    ▲
                    │ implements
                    │
┌─────────────────────────────────────────┐
│      PostgresRepository                 │
│  ┌───────────────────────────────────┐  │
│  │  - db: *sql.DB                      │  │
│  │  - Connection Pool                  │  │
│  │  - createTables()                   │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

### 2. Frontend Architecture (React)

#### 2.1 Component Hierarchy

```
┌─────────────────────────────────────────┐
│              App.jsx                    │
│  ┌───────────────────────────────────┐  │
│  │  BrowserRouter                     │  │
│  │  ├─ Route: /                       │  │
│  │  │  └─> LobbyScreen               │  │
│  │  └─ Route: /game/:lobbyId         │  │
│  │     └─> GameScreen                │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
        ▼                       ▼
┌──────────────┐        ┌──────────────┐
│LobbyScreen   │        │ GameScreen   │
│              │        │              │
│ - List       │        │ - Question   │
│   Lobbies    │        │ - Answers    │
│ - Create     │        │ - Leaderboard│
│   Lobby      │        │ - Chat       │
│ - Join       │        │ - Timer      │
│   Lobby      │        │              │
└──────────────┘        └──────┬───────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
                    ▼                   ▼
            ┌──────────────┐    ┌──────────────┐
            │ Leaderboard  │    │ Connection   │
            │ Component    │    │ Status       │
            └──────────────┘    └──────────────┘
```

#### 2.2 Service Layer

```
┌─────────────────────────────────────────┐
│         WebSocketService                │
│  ┌───────────────────────────────────┐  │
│  │  - ws: WebSocket                   │  │
│  │  - listeners: Map                 │  │
│  │  - connectionStatus: string        │  │
│  │                                    │  │
│  │  Methods:                          │  │
│  │  - connect()                       │  │
│  │  - disconnect()                    │  │
│  │  - joinLobby(id, username)         │  │
│  │  - submitAnswer(...)               │  │
│  │  - sendChatMessage(...)            │  │
│  │  - on(event, callback)             │  │
│  │  - off(event, callback)            │  │
│  │  - emit(event, data)               │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│              API Service                │
│  ┌───────────────────────────────────┐  │
│  │  REST API Methods:                  │  │
│  │  - createLobby(name, maxRounds)     │  │
│  │  - getLobby(id)                     │  │
│  │  - listLobbies()                    │  │
│  │  - joinLobby(id, username)          │  │
│  │  - startGame(id)                    │  │
│  │  - submitAnswer(id, playerId, ...)  │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

#### 2.3 State Management

```
┌─────────────────────────────────────────┐
│         GameScreen Component            │
│  ┌───────────────────────────────────┐  │
│  │  State:                           │  │
│  │  - lobby: Lobby                   │  │
│  │  - player: Player                 │  │
│  │  - currentQuestion: Question      │  │
│  │  - timeRemaining: number          │  │
│  │  - chatMessages: Array            │  │
│  │                                    │  │
│  │  Refs:                             │  │
│  │  - timerRef                       │  │
│  │  - questionTimerRef               │  │
│  │  - questionEndTimeRef             │  │
│  │  - serverTimeOffsetRef            │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

## Data Flow

### 1. Lobby Creation Flow

```
Client                    Server                    Database
  │                         │                          │
  │ POST /api/v1/lobbies    │                          │
  │────────────────────────>│                          │
  │                         │                          │
  │                         │ CreateLobby()            │
  │                         │─────────────────────────>│
  │                         │                          │ Save
  │                         │<─────────────────────────│
  │                         │                          │
  │                         │ CreateLobbyHub()         │
  │                         │                          │
  │<────────────────────────│ 201 Created              │
  │ {lobby}                 │                          │
```

### 2. WebSocket Connection Flow

```
Client                    Server
  │                         │
  │ GET /ws                 │
  │────────────────────────>│
  │                         │ Upgrade to WebSocket
  │<────────────────────────│ 101 Switching Protocols
  │                         │
  │ WebSocket Open          │
  │────────────────────────>│
  │                         │ Register Client
  │                         │ Add to Hub
  │                         │
  │ send: join_lobby        │
  │────────────────────────>│
  │                         │ JoinLobby()
  │                         │ Broadcast player_joined
  │<────────────────────────│
  │ player_joined event     │
```

### 3. Game Play Flow

```
Client                    Server                    Hub
  │                         │                         │
  │ send: start_game        │                         │
  │────────────────────────>│                         │
  │                         │ StartGame()              │
  │                         │─────────────────────────>│
  │                         │                         │ Broadcast
  │<────────────────────────│ game_started            │
  │                         │<────────────────────────│
  │                         │                         │
  │<────────────────────────│ new_question             │
  │                         │                         │
  │ Timer starts            │                         │
  │                         │                         │
  │ User selects answer     │                         │
  │ send: submit_answer     │                         │
  │────────────────────────>│                         │
  │                         │ SubmitAnswer()           │
  │                         │ CalculateScore()         │
  │                         │─────────────────────────>│
  │<────────────────────────│ answer_received          │
  │                         │<────────────────────────│
  │                         │                         │
  │<────────────────────────│ question_results         │
  │                         │                         │
```

## Concurrency Model

### Goroutine Architecture

```
Main Goroutine
  │
  ├─> HTTP Server (Gin)
  │   └─> Request Handlers
  │
  ├─> WebSocket Handler
  │   └─> Client Connection Goroutines
  │       ├─> Read Pump
  │       └─> Write Pump
  │
  └─> Hub System
      └─> LobbyHub Goroutines (per lobby)
          └─> Event Loop:
              ├─ register channel
              ├─ unregister channel
              ├─ broadcast channel
              └─ ticker (1s)
```

### Synchronization Primitives

1. **Mutexes:**
   - `Hub.mu`: `sync.RWMutex` for lobby map access
   - `LobbyHub.mu`: `sync.RWMutex` for client map access

2. **Channels:**
   - `register`: Buffered channel for client registration
   - `unregister`: Buffered channel for client removal
   - `broadcast`: Buffered channel for message broadcasting
   - `client.Send`: Per-client buffered channel (256)

3. **Race Condition Prevention:**
   - All map access protected by mutexes
   - `GetClients()` returns a copy to prevent concurrent iteration
   - Channel-based communication for thread-safe operations

## Scalability Considerations

### Current Limitations

1. **Single Server Instance:**
   - All lobbies in memory
   - No horizontal scaling support
   - WebSocket connections tied to single server

2. **Database:**
   - PostgreSQL connection pooling
   - No read replicas
   - No caching layer

3. **State Management:**
   - Lobby state in memory (Hub)
   - Database for persistence only
   - No distributed state management

### Scaling Strategies

1. **Horizontal Scaling:**
   ```
   Load Balancer
        │
   ┌────┴────┐
   │         │
   ▼         ▼
  Server 1  Server 2  Server N
   │         │         │
   └────┬────┴────┬────┘
        │         │
        ▼         ▼
    Redis Pub/Sub
        │
        ▼
    PostgreSQL
   ```

2. **State Synchronization:**
   - We can use Redis for distributed state
   - Pub/Sub for cross-server communication
   - Sticky sessions (session affinity)

3. **Database Optimization:**
   - Read replicas for queries
   - Connection pooling
   - Query optimization
   - Caching layer (Redis)

## Security Considerations

### Current Implementation

1. **CORS:**
   - Currently allows all origins (`*`)
   - Should be restricted in production

2. **WebSocket:**
   - No authentication/authorization
   - No rate limiting
   - No input validation on messages

3. **Database:**
   - Parameterized queries (SQL injection protection)
   - No encryption at rest mentioned

### Recommendations

1. **Authentication:**
   - JWT tokens for WebSocket connections
   - Session management
   - User authentication middleware

2. **Authorization:**
   - Lobby access control
   - Host-only game start
   - Player verification

3. **Rate Limiting:**
   - Per-IP rate limiting
   - Per-user rate limiting
   - WebSocket message throttling

4. **Input Validation:**
   - Validate all WebSocket messages
   - Sanitize user inputs
   - Validate answer submissions

## Performance Metrics

### Expected Performance

- **Concurrent Connections:** ~1,000 per server instance
- **Lobby Capacity:** 8 players per lobby
- **Question Time:** 30 seconds
- **Response Time:** <100ms for API calls
- **WebSocket Latency:** <50ms

### Bottlenecks

1. **Database Writes:**
   - Every answer submission writes to DB
   - Consider batching or async writes

2. **Broadcast Operations:**
   - Linear time complexity O(n) per broadcast
   - Could be optimized with fan-out pattern

3. **Memory Usage:**
   - All lobbies in memory
   - No cleanup for abandoned lobbies (except 10min cleanup)

## Deployment Architecture

```
┌─────────────────────────────────────────┐
│         Docker Compose                  │
│  ┌───────────────────────────────────┐  │
│  │  Services:                         │  │
│  │  ├─ postgres:5432                  │  │
│  │  └─ app:8080                       │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│         Railway.app                     │
│  ┌───────────────────────────────────┐  │
│  │  - Auto PostgreSQL provisioning   │  │
│  │  - Environment variables           │  │
│  │  - Build & deploy                  │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

## Technology Stack

### Backend
- **Language:** Go 1.21+
- **Framework:** Gin (HTTP router)
- **WebSocket:** gorilla/websocket
- **Database:** PostgreSQL (lib/pq driver)
- **UUID:** google/uuid

### Frontend
- **Framework:** React 18
- **Build Tool:** Vite
- **Routing:** react-router-dom
- **Styling:** CSS Modules

### Infrastructure
- **Containerization:** Docker
- **Orchestration:** Docker Compose
- **Deployment:** Railway.app
- **Database:** PostgreSQL 15

## Design Patterns Used

1. **Repository Pattern:** Data access abstraction
2. **Hub Pattern:** WebSocket connection management
3. **Observer Pattern:** Event-driven updates
4. **Singleton Pattern:** Hub instance
5. **Factory Pattern:** Lobby creation
6. **Strategy Pattern:** Scoring calculation

## Future Enhancements

1. **Microservices:**
   - Separate game service
   - Separate lobby service
   - Separate notification service

2. **Real-time Infrastructure:**
   - Redis Pub/Sub
   - Message queue (RabbitMQ/Kafka)
   - WebSocket gateway

3. **Monitoring:**
   - Prometheus metrics
   - Grafana dashboards
   - Distributed tracing

4. **Testing:**
   - Integration tests
   - Load testing
   - Chaos engineering
