import React, { useState, useEffect } from 'react';
import './App.css';

interface User {
  id: number;
  name: string;
  pin: string;
}

interface Game {
  id: number;
  title: string;
  cover_url: string;
  user_id: number;
  user_name?: string;
}

interface GameSearchResult {
  id: number;
  name: string;
  cover?: {
    id: number;
    url: string;
  };
  summary?: string;
}

function App() {
  const [users, setUsers] = useState<User[]>([]);
  const [games, setGames] = useState<Game[]>([]);
  const [searchResults, setSearchResults] = useState<GameSearchResult[]>([]);
  const [liveSearchResult, setLiveSearchResult] = useState<GameSearchResult | null>(null);
  const [newUserName, setNewUserName] = useState('');
  const [newUserPin, setNewUserPin] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedUser, setSelectedUser] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [liveSearchLoading, setLiveSearchLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch users on component mount
  useEffect(() => {
    fetchUsers();
    fetchGames();
  }, []);

  // Live search with debounce
  useEffect(() => {
    if (!searchQuery.trim()) {
      setLiveSearchResult(null);
      return;
    }

    const timeoutId = setTimeout(() => {
      performLiveSearch(searchQuery);
    }, 300); // 300ms debounce

    return () => clearTimeout(timeoutId);
  }, [searchQuery]);

  const fetchUsers = async () => {
    try {
      const response = await fetch('/api/ebwg/users');
      if (response.ok) {
        const data = await response.json();
        setUsers(data || []);
      } else {
        setError('Failed to fetch users');
        setUsers([]);
      }
    } catch (err) {
      setError('Network error fetching users');
      setUsers([]);
    }
  };

  const fetchGames = async () => {
    try {
      const response = await fetch('/api/ebwg/games');
      if (response.ok) {
        const data = await response.json();
        setGames(data || []);
      } else {
        setError('Failed to fetch games');
        setGames([]);
      }
    } catch (err) {
      setError('Network error fetching games');
      setGames([]);
    }
  };

  const addUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUserName.trim() || !newUserPin.trim()) return;
    
    if (newUserPin.length !== 4) {
      setError('PIN must be exactly 4 digits');
      return;
    }

    try {
      setLoading(true);
      const response = await fetch('/api/ebwg/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: newUserName.trim(),
          pin: newUserPin.trim(),
        }),
      });

      if (response.ok) {
        setNewUserName('');
        setNewUserPin('');
        setError(null);
        await fetchUsers();
      } else {
        const errorText = await response.text();
        setError(`Failed to add user: ${errorText}`);
      }
    } catch (err) {
      setError('Network error adding user');
    } finally {
      setLoading(false);
    }
  };

  const removeUser = async (userId: number) => {
    try {
      setLoading(true);
      const response = await fetch(`/api/ebwg/users/${userId}`, {
        method: 'DELETE',
      });

      if (response.ok) {
        await fetchUsers();
        await fetchGames();
        setError(null);
      } else {
        setError('Failed to remove user');
      }
    } catch (err) {
      setError('Network error removing user');
    } finally {
      setLoading(false);
    }
  };

  const performLiveSearch = async (query: string) => {
    try {
      setLiveSearchLoading(true);
      const response = await fetch(`/api/ebwg/search-games?query=${encodeURIComponent(query)}&limit=1`);
      if (response.ok) {
        const data = await response.json();
        setLiveSearchResult(data && data.length > 0 ? data[0] : null);
      } else {
        setLiveSearchResult(null);
      }
    } catch (err) {
      setLiveSearchResult(null);
    } finally {
      setLiveSearchLoading(false);
    }
  };

  const searchGames = async () => {
    if (!searchQuery.trim()) return;

    try {
      setLoading(true);
      const response = await fetch(`/api/ebwg/search-games?query=${encodeURIComponent(searchQuery)}&limit=10`);
      if (response.ok) {
        const data = await response.json();
        setSearchResults(data || []);
        setError(null);
      } else {
        setError('Failed to search games');
        setSearchResults([]);
      }
    } catch (err) {
      setError('Network error searching games');
      setSearchResults([]);
    } finally {
      setLoading(false);
    }
  };

  const addGameToQueue = async (game: GameSearchResult) => {
    if (!selectedUser) {
      setError('Please select a user first');
      return;
    }

    try {
      setLoading(true);
      const response = await fetch('/api/ebwg/games', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          title: game.name,
          cover_url: game.cover?.url || '',
          user_id: selectedUser,
        }),
      });

      if (response.ok) {
        await fetchGames();
        setError(null);
      } else {
        setError('Failed to add game to queue');
      }
    } catch (err) {
      setError('Network error adding game');
    } finally {
      setLoading(false);
    }
  };

  const removeGameFromQueue = async (gameId: number) => {
    try {
      setLoading(true);
      const response = await fetch(`/api/ebwg/games/${gameId}`, {
        method: 'DELETE',
      });

      if (response.ok) {
        await fetchGames();
        setError(null);
      } else {
        setError('Failed to remove game from queue');
      }
    } catch (err) {
      setError('Network error removing game');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="App">
      <header className="App-header">
        <h1>🎮 EBWG - Kino Video Game Club</h1>
        <p>Elite Brotherhood of Wonderful Gamers</p>
      </header>

      {error && (
        <div className="error-banner">
          {error}
          <button onClick={() => setError(null)}>✕</button>
        </div>
      )}

      <main className="main-content">
        {/* User Management Section */}
        <section className="section">
          <h2>Club Members</h2>
          
          <form onSubmit={addUser} className="user-form">
            <input
              type="text"
              placeholder="Member name"
              value={newUserName}
              onChange={(e) => setNewUserName(e.target.value)}
              disabled={loading}
            />
            <input
              type="text"
              placeholder="4-digit PIN"
              value={newUserPin}
              onChange={(e) => setNewUserPin(e.target.value)}
              maxLength={4}
              pattern="[0-9]{4}"
              disabled={loading}
            />
            <button type="submit" disabled={loading || !newUserName.trim() || !newUserPin.trim()}>
              {loading ? 'Adding...' : 'Add Member'}
            </button>
          </form>

          <div className="users-grid">
            {users && users.map((user) => (
              <div key={user.id} className="user-card">
                <h3>{user.name}</h3>
                <p>PIN: {user.pin}</p>
                <div className="user-actions">
                  <button 
                    className={`select-btn ${selectedUser === user.id ? 'selected' : ''}`}
                    onClick={() => setSelectedUser(selectedUser === user.id ? null : user.id)}
                  >
                    {selectedUser === user.id ? 'Selected' : 'Select'}
                  </button>
                  <button 
                    className="remove-btn"
                    onClick={() => removeUser(user.id)}
                    disabled={loading}
                  >
                    Remove
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Game Search Section */}
        <section className="section">
          <h2>Search Games</h2>
          
          <div className="search-form">
            <input
              type="text"
              placeholder="Search for games..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              disabled={loading}
            />
            <button onClick={searchGames} disabled={loading || !searchQuery.trim()}>
              {loading ? 'Searching...' : 'Search More'}
            </button>
          </div>

          {/* Live Search Result */}
          {searchQuery.trim() && liveSearchResult && (
            <div className="live-search-result">
              <h3>
                Quick Result {liveSearchLoading && <span className="loading-indicator">...</span>}
              </h3>
              <div className="game-card live-result">
                {liveSearchResult.cover && (
                  <img 
                    src={liveSearchResult.cover.url} 
                    alt={liveSearchResult.name}
                    className="game-cover"
                  />
                )}
                <div className="game-info">
                  <h4>{liveSearchResult.name}</h4>
                  {liveSearchResult.summary && <p className="game-summary">{liveSearchResult.summary}</p>}
                  <button 
                    className="add-game-btn"
                    onClick={() => addGameToQueue(liveSearchResult)}
                    disabled={loading || !selectedUser}
                  >
                    Add to Queue
                  </button>
                </div>
              </div>
              <p className="search-hint">Click "Search More" to see more results</p>
            </div>
          )}

          {/* Full Search Results */}
          {searchResults.length > 0 && (
            <div className="search-results">
              <h3>All Results ({searchResults.length})</h3>
              <div className="games-grid">
                {searchResults && searchResults.map((game) => (
                  <div key={game.id} className="game-card">
                    {game.cover && (
                      <img 
                        src={game.cover.url} 
                        alt={game.name}
                        className="game-cover"
                      />
                    )}
                    <div className="game-info">
                      <h4>{game.name}</h4>
                      {game.summary && <p className="game-summary">{game.summary}</p>}
                      <button 
                        className="add-game-btn"
                        onClick={() => addGameToQueue(game)}
                        disabled={loading || !selectedUser}
                      >
                        Add to Queue
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* No results message */}
          {searchQuery.trim() && !liveSearchResult && !liveSearchLoading && searchResults.length === 0 && !loading && (
            <div className="no-results">
              <p>No games found for "{searchQuery}". Try a different search term.</p>
            </div>
          )}
        </section>

        {/* Game Queues Section */}
        <section className="section">
          <h2>Game Queues</h2>
          
          {users && users.map((user) => {
            const userGames = games ? games.filter(game => game.user_id === user.id) : [];
            return (
              <div key={user.id} className="user-queue">
                <h3>{user.name}'s Queue ({userGames.length} games)</h3>
                {userGames.length === 0 ? (
                  <p className="empty-queue">No games in queue</p>
                ) : (
                  <div className="queue-games">
                    {userGames && userGames.map((game) => (
                      <div key={game.id} className="queue-game">
                        {game.cover_url && (
                          <img 
                            src={game.cover_url} 
                            alt={game.title}
                            className="queue-game-cover"
                          />
                        )}
                        <div className="queue-game-info">
                          <h4>{game.title}</h4>
                          <button 
                            className="remove-game-btn"
                            onClick={() => removeGameFromQueue(game.id)}
                            disabled={loading}
                          >
                            Remove
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </section>
      </main>
    </div>
  );
}

export default App;
