# 🧪 BuildPrize Quiz Backend Test Scripts

This directory contains test scripts to verify that your Go backend is working correctly.

## 🚀 Quick Start

### Option 1: Bash Script (Recommended)
```bash
# Make executable and run
chmod +x test_quiz.sh
./test_quiz.sh
```

### Option 2: Python Script
```bash
# Run Python test
python3 test_quiz.py
```

## 📋 What the Tests Do

The test scripts will automatically:

1. **✅ Health Check** - Verify server is running
2. **✅ Create Lobby** - Test lobby creation API
3. **✅ List Lobbies** - Test lobby listing API
4. **✅ Join Players** - Test player joining functionality
5. **✅ Start Game** - Test game start functionality
6. **✅ Submit Answers** - Test answer submission with scoring
7. **✅ Check State** - Verify lobby state updates
8. **✅ Leave Lobby** - Test player leaving functionality

## 🎯 Test Scenarios

### Lobby Management
- Creates a test lobby with 3 rounds
- Joins 2 players (Player1, Player2)
- Verifies player list updates

### Game Flow
- Starts the game when 2+ players are ready
- Submits answers with different response times
- Tests scoring system (fast correct vs slow wrong)

### API Endpoints Tested
- `GET /health` - Health check
- `POST /api/v1/lobbies` - Create lobby
- `GET /api/v1/lobbies` - List lobbies
- `GET /api/v1/lobbies/:id` - Get lobby details
- `POST /api/v1/lobbies/:id/join` - Join lobby
- `POST /api/v1/lobbies/:id/start` - Start game
- `POST /api/v1/lobbies/:id/answer` - Submit answer
- `POST /api/v1/lobbies/:id/leave` - Leave lobby

## 🔧 Prerequisites

### For Bash Script:
- `curl` command available
- `jq` command available (optional, for pretty JSON)

### For Python Script:
- Python 3.6+
- `requests` library: `pip install requests`

## 🚀 Usage Examples

### Test with Server Auto-Start
```bash
./test_quiz.sh
```

### Test with Existing Server
```bash
# Start server in another terminal
go run main.go

# Run tests
./test_quiz.sh --no-server
```

### Python Version
```bash
# Install dependencies
pip install requests

# Run tests
python3 test_quiz.py
```

## 📊 Expected Output

```
[INFO] 🚀 Starting BuildPrize Quiz Backend Tests
==================================================
[INFO] Testing health endpoint...
[SUCCESS] Health check passed
[INFO] Testing lobby creation...
[SUCCESS] Lobby created with ID: abc123-def456-...
[INFO] Testing lobby listing...
[SUCCESS] Lobby listing works
[INFO] Testing lobby joining...
[SUCCESS] Player1 joined lobby
[INFO] Testing second player joining...
[SUCCESS] Player2 joined lobby
[INFO] Testing game start...
[SUCCESS] Game started successfully
[INFO] Testing answer submission...
[SUCCESS] Player1 answer submitted
[SUCCESS] Player2 answer submitted
[INFO] Waiting for question timeout...
[INFO] Testing lobby state retrieval...
[SUCCESS] Lobby state retrieved
[INFO] Testing player leaving lobby...
[SUCCESS] Player1 left lobby
[SUCCESS] 🎉 All tests passed successfully!
==================================================
[INFO] Backend is working correctly!
```

## 🐛 Troubleshooting

### Server Not Running
```
[ERROR] Server is not running on port 8080
[INFO] Please start the server first: go run main.go
```

**Solution:** Start the server in another terminal:
```bash
go run main.go
```

### Port Already in Use
```
[ERROR] Cannot start server - make sure port 8080 is free
```

**Solution:** Kill any process using port 8080:
```bash
lsof -ti:8080 | xargs kill -9
```

### Dependencies Missing
```bash
# Install jq for JSON formatting
brew install jq  # macOS
sudo apt install jq  # Ubuntu

# Install Python requests
pip install requests
```

## 🎮 After Tests Pass

Once all tests pass, your backend is ready for:

- **Real-time multiplayer games**
- **WebSocket connections**
- **Database persistence** (PostgreSQL)
- **Production deployment** (Railway, Render, etc.)

## 📝 Notes

- Tests use in-memory storage by default
- For PostgreSQL testing, set `DATABASE_URL` environment variable
- WebSocket functionality is tested implicitly through API calls
- All test data is cleaned up automatically

Happy testing! 🎉
