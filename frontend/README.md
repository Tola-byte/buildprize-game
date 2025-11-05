# Quiz Game Frontend

React + Vite frontend for the Quiz Game application.

## Features

- 🎮 Create and join lobbies
- ⚡ Real-time updates via WebSocket
- 🎯 Interactive quiz questions with timer
- 📊 Live leaderboard updates
- 🎨 Modern, responsive UI

## Setup

```bash
cd frontend
npm install
npm run dev
```

The frontend will run on `http://localhost:3000`

## Usage

1. **Start the Go backend** (from project root):
   ```bash
   DATABASE_URL="postgres://quizuser:quizpass@localhost:5432/quizdb?sslmode=disable" go run main.go
   ```

2. **Start PostgreSQL** (if using Docker):
   ```bash
   docker-compose up -d postgres
   ```

3. **Start the frontend**:
   ```bash
   cd frontend
   npm run dev
   ```

4. Open `http://localhost:3000` in your browser

## How it Works

- **Lobby Screen**: Create or join game lobbies
- **Game Screen**: Play quiz questions with real-time updates
- **Leaderboard**: See scores update in real-time as players answer
- **WebSocket**: Connects to `ws://localhost:8080/ws` for real-time events

## API Endpoints Used

- `POST /api/v1/lobbies` - Create lobby
- `GET /api/v1/lobbies` - List lobbies
- `POST /api/v1/lobbies/:id/join` - Join lobby
- `POST /api/v1/lobbies/:id/start` - Start game
- `POST /api/v1/lobbies/:id/answer` - Submit answer

## WebSocket Events

- `player_joined` - New player joined
- `player_left` - Player left
- `new_question` - New question started
- `answer_received` - Answer submitted
- `question_results` - Question results
- `game_ended` - Game finished
