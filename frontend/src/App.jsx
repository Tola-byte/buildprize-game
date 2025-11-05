import { BrowserRouter, Routes, Route } from 'react-router-dom';
import LobbyScreen from './components/LobbyScreen';
import GameScreen from './components/GameScreen';
import './App.css';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LobbyScreen />} />
        <Route path="/game/:lobbyId" element={<GameScreen />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
