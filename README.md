# BuildPrize Quiz Game

A real-time multiplayer quiz application built in Go, designed to handle 50,000+ concurrent users with low latency and high scalability.

## Features

- **Real-time Multiplayer**: WebSocket-based real-time communication
- **Lobby System**: Create and join game lobbies
- **Live Scoring**: Real-time leaderboards with streak bonuses
- **Smart Questions**: Multiple categories with difficulty levels
- **Responsive UI**: Works on desktop and mobile devices
- **Auto-reconnection**: Handles network disconnections gracefully

## Quick Start

### Prerequisites

- Go 1.21 or higher
- PostgreSQL (optional, for production)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd buildprize-game
```

2. Install dependencies:
```bash
go mod tidy
```

3. Run the server:
```bash
go run main.go
```

4. The server will start on `http://localhost:8080`

## API Endpoints

### HTTP API

- `POST /api/v1/lobbies` - Create a new lobby
- `GET /api/v1/lobbies` - List available lobbies
- `POST /api/v1/lobbies/:id/join` - Join a lobby
- `POST /api/v1/lobbies/:id/leave` - Leave a lobby
- `POST /api/v1/lobbies/:id/start` - Start the game
- `POST /api/v1/lobbies/:id/answer` - Submit an answer

### WebSocket Events

- `join_lobby` - Join a lobby via WebSocket
- `leave_lobby` - Leave a lobby
- `start_game` - Start the game
- `submit_answer` - Submit an answer

## Game Flow

1. **Create/Join Lobby**: Players create or join a lobby
2. **Wait for Players**: Lobby waits for minimum 2 players
3. **Start Game**: Host starts the game
4. **Questions**: Server sends questions with time limits
5. **Scoring**: Points awarded for correct answers and speed
6. **Leaderboard**: Real-time leaderboard updates
7. **Game End**: Final results and winner announcement

## Scoring System

- **Base Score**: 100 points for correct answer
- **Speed Bonus**: Up to 50 points for fast responses
- **Accuracy Bonus**: 25 points for correct answers
- **Streak Bonus**: Multiplier for consecutive correct answers

## Architecture

The application uses several design patterns:

- **Event-Driven Architecture**: Real-time updates via WebSocket events
- **Repository Pattern**: Abstracted data persistence layer
- **Observer Pattern**: Hub system for client notifications
- **Command Pattern**: WebSocket message handling
- **Singleton Pattern**: Global game state management

## Scaling Strategy

### Horizontal Scaling
- Load balancing across multiple server instances
- PostgreSQL for shared session state
- Regional deployment for reduced latency

### Performance Optimizations
- Connection pooling
- Message batching
- Database caching
- CDN for static assets

## Development

### Project Structure

```
buildprize-game/
├── main.go                 # Application entry point
├── internal/
│   ├── config/            # Configuration management
│   ├── models/            # Data models
│   ├── hub/               # WebSocket hub system
│   ├── services/          # Business logic
│   ├── repository/        # Data persistence
│   └── server/            # HTTP/WebSocket server
└── ARCHITECTURE_FLOW.md   # Detailed architecture docs
```

### Adding New Features

1. **New Game Modes**: Extend the `GameService` with new game logic
2. **Question Categories**: Add to `QuestionDatabase` in `services/questions.go`
3. **Scoring Rules**: Modify `calculateScore` in `services/game_service.go`

## Testing

### Manual Testing
1. Open multiple browser tabs
2. Create a lobby in one tab
3. Join the lobby from other tabs
4. Start the game and test the flow

### Automated Testing
```bash
# Run Go tests
go test ./internal/testing -v

# Or use Makefile
make test
```

## Production Deployment

### Docker
```bash
# Build image
docker build -t buildprize-game .

# Run container
docker run -p 8080:8080 buildprize-game
```

### Railway.app
The application is configured for Railway deployment with automatic PostgreSQL database provisioning.

## Environment Variables

- `PORT`: Server port (default: 8080)
- `DATABASE_URL`: PostgreSQL connection URL (required)
- `MAX_LOBBY_SIZE`: Maximum players per lobby (default: 8)
- `QUESTION_TIME`: Time per question in seconds (default: 30)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details
