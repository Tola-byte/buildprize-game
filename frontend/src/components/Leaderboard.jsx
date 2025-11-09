import './Leaderboard.css';

export default function Leaderboard({ players = [] }) {
  // Sort players by score (descending)
  const sortedPlayers = [...players].sort((a, b) => b.score - a.score);

  return (
    <div className="leaderboard">
      <h2>Leaderboard</h2>
      <div className="leaderboard-list">
        {sortedPlayers.map((player, index) => (
          <div key={player.id} className="leaderboard-item">
            <div className="rank">#{index + 1}</div>
            <div className="player-info">
              <div className="player-name">{player.username}</div>
              <div className="player-stats">
                <span>Score: {player.score}</span>
                {player.streak > 0 && <span className="streak">🔥 {player.streak}</span>}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}



