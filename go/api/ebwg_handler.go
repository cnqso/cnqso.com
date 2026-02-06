package api

import (
	"encoding/json"
	"net/http"
	"server/db"
	"server/logs"
	"strconv"
	"strings"
)

type EBWGUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Pin  string `json:"pin"`
}

type Game struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	CoverURL string `json:"cover_url"`
	UserID   int    `json:"user_id"`
}

type GameAPIResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Cover   *Cover `json:"cover,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type Cover struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

// EBWGHandler serves the React app for EBWG
func EBWGHandler(w http.ResponseWriter, r *http.Request) {
	filePath := "static/react/ebwg" + r.URL.Path[len("/ebwg"):]
	if r.URL.Path == "/ebwg" || r.URL.Path == "/ebwg/" {
		filePath = "static/react/ebwg/index.html"
	}
	http.ServeFile(w, r, filePath)
}

// EBWGAPIHandler handles API requests for EBWG functionality
func EBWGAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/ebwg")

	switch {
	case strings.HasPrefix(path, "/users"):
		handleUsers(w, r, path)
	case strings.HasPrefix(path, "/games"):
		handleGames(w, r, path)
	case strings.HasPrefix(path, "/search-games"):
		handleGameSearch(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func handleUsers(w http.ResponseWriter, r *http.Request, path string) {
	switch r.Method {
	case "GET":
		if path == "/users" {
			getAllUsers(w, r)
		} else {
			// Handle /users/{id} pattern
			userIDStr := strings.TrimPrefix(path, "/users/")
			if userIDStr != "" {
				getUserByID(w, r, userIDStr)
			} else {
				http.Error(w, "Invalid user ID", http.StatusBadRequest)
			}
		}
	case "POST":
		if path == "/users" {
			createUser(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "DELETE":
		userIDStr := strings.TrimPrefix(path, "/users/")
		if userIDStr != "" {
			deleteUser(w, r, userIDStr)
		} else {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGames(w http.ResponseWriter, r *http.Request, path string) {
	switch r.Method {
	case "GET":
		if strings.Contains(path, "/users/") {
			// Handle /games/users/{id} pattern
			parts := strings.Split(path, "/")
			if len(parts) >= 3 && parts[1] == "users" {
				getUserGames(w, r, parts[2])
			} else {
				http.Error(w, "Invalid path", http.StatusBadRequest)
			}
		} else {
			getAllGames(w, r)
		}
	case "POST":
		if path == "/games" {
			addGameToQueue(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "DELETE":
		gameIDStr := strings.TrimPrefix(path, "/games/")
		if gameIDStr != "" {
			removeGameFromQueue(w, r, gameIDStr)
		} else {
			http.Error(w, "Invalid game ID", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, name, pin FROM ebwg_users ORDER BY name")
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error fetching users")
		return
	}
	defer rows.Close()

	var users []EBWGUser
	for rows.Next() {
		var user EBWGUser
		err := rows.Scan(&user.ID, &user.Name, &user.Pin)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error scanning user")
			return
		}
		users = append(users, user)
	}

	json.NewEncoder(w).Encode(users)
}

func getUserByID(w http.ResponseWriter, r *http.Request, userIDStr string) {
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user EBWGUser
	err = db.DB.QueryRow("SELECT id, name, pin FROM ebwg_users WHERE id = ?", userID).Scan(&user.ID, &user.Name, &user.Pin)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error fetching user")
		}
		return
	}

	json.NewEncoder(w).Encode(user)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var user EBWGUser
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if user.Name == "" || user.Pin == "" {
		http.Error(w, "Name and pin are required", http.StatusBadRequest)
		return
	}

	if len(user.Pin) != 4 {
		http.Error(w, "Pin must be exactly 4 digits", http.StatusBadRequest)
		return
	}

	result, err := db.DB.Exec("INSERT INTO ebwg_users (name, pin) VALUES (?, ?)", user.Name, user.Pin)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error creating user")
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error getting user ID")
		return
	}

	user.ID = int(id)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func deleteUser(w http.ResponseWriter, r *http.Request, userIDStr string) {
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Also delete user's games
	_, err = db.DB.Exec("DELETE FROM ebwg_games WHERE user_id = ?", userID)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error deleting user games")
		return
	}

	result, err := db.DB.Exec("DELETE FROM ebwg_users WHERE id = ?", userID)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error deleting user")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error checking deletion")
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getAllGames(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(`
		SELECT g.id, g.title, g.cover_url, g.user_id, u.name
		FROM ebwg_games g
		JOIN ebwg_users u ON g.user_id = u.id
		ORDER BY u.name, g.id
	`)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error fetching games")
		return
	}
	defer rows.Close()

	var games []map[string]interface{}
	for rows.Next() {
		var game Game
		var userName string
		err := rows.Scan(&game.ID, &game.Title, &game.CoverURL, &game.UserID, &userName)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error scanning game")
			return
		}

		gameMap := map[string]interface{}{
			"id":        game.ID,
			"title":     game.Title,
			"cover_url": game.CoverURL,
			"user_id":   game.UserID,
			"user_name": userName,
		}
		games = append(games, gameMap)
	}

	json.NewEncoder(w).Encode(games)
}

func getUserGames(w http.ResponseWriter, r *http.Request, userIDStr string) {
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	rows, err := db.DB.Query("SELECT id, title, cover_url, user_id FROM ebwg_games WHERE user_id = ? ORDER BY id", userID)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error fetching user games")
		return
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		var game Game
		err := rows.Scan(&game.ID, &game.Title, &game.CoverURL, &game.UserID)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error scanning game")
			return
		}
		games = append(games, game)
	}

	json.NewEncoder(w).Encode(games)
}

func addGameToQueue(w http.ResponseWriter, r *http.Request) {
	var game Game
	if err := json.NewDecoder(r.Body).Decode(&game); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if game.Title == "" || game.UserID == 0 {
		http.Error(w, "Title and user_id are required", http.StatusBadRequest)
		return
	}

	// Verify user exists
	var exists bool
	err := db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM ebwg_users WHERE id = ?)", game.UserID).Scan(&exists)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error checking user")
		return
	}
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	result, err := db.DB.Exec("INSERT INTO ebwg_games (title, cover_url, user_id) VALUES (?, ?, ?)",
		game.Title, game.CoverURL, game.UserID)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error adding game")
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error getting game ID")
		return
	}

	game.ID = int(id)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(game)
}

func removeGameFromQueue(w http.ResponseWriter, r *http.Request, gameIDStr string) {
	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		http.Error(w, "Invalid game ID", http.StatusBadRequest)
		return
	}

	result, err := db.DB.Exec("DELETE FROM ebwg_games WHERE id = ?", gameID)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error deleting game")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error checking deletion")
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleGameSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	limit := r.URL.Query().Get("limit")
	searchLimit := 10 // default for button search
	if limit == "1" {
		searchLimit = 1 // for live search
	}

	games, err := searchGamesFromAPI(query, searchLimit)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error searching games")
		return
	}

	json.NewEncoder(w).Encode(games)
}

// searchGamesFromAPI searches for games using a curated database
// In production, you'd want to use IGDB API with proper authentication
func searchGamesFromAPI(query string, limit int) ([]GameAPIResponse, error) {
	// Curated list of popular games with real covers from IGDB
	allGames := []GameAPIResponse{
		{
			ID:   1,
			Name: "The Legend of Zelda: Breath of the Wild",
			Cover: &Cover{
				ID:  1,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1nqf.webp",
			},
			Summary: "An open-world action-adventure game featuring Link in the kingdom of Hyrule.",
		},
		{
			ID:   2,
			Name: "Grand Theft Auto V",
			Cover: &Cover{
				ID:  2,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co2lbd.webp",
			},
			Summary: "An action-adventure game played from either a third-person or first-person perspective.",
		},
		{
			ID:   3,
			Name: "The Witcher 3: Wild Hunt",
			Cover: &Cover{
				ID:  3,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1wyy.webp",
			},
			Summary: "A story-driven open world RPG set in a visually stunning fantasy universe.",
		},
		{
			ID:   4,
			Name: "Cyberpunk 2077",
			Cover: &Cover{
				ID:  4,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co2lc0.webp",
			},
			Summary: "An open-world, action-adventure story set in Night City.",
		},
		{
			ID:   5,
			Name: "Red Dead Redemption 2",
			Cover: &Cover{
				ID:  5,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1q1f.webp",
			},
			Summary: "An epic tale of life in America's unforgiving heartland.",
		},
		{
			ID:   6,
			Name: "Minecraft",
			Cover: &Cover{
				ID:  6,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1rkt.webp",
			},
			Summary: "A sandbox video game where players can build with a variety of different blocks.",
		},
		{
			ID:   7,
			Name: "Dark Souls III",
			Cover: &Cover{
				ID:  7,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1vcq.webp",
			},
			Summary: "An action RPG set in a universe full of decadent atmosphere and imagery.",
		},
		{
			ID:   8,
			Name: "Hades",
			Cover: &Cover{
				ID:  8,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co2a64.webp",
			},
			Summary: "A rogue-like dungeon crawler in which you defy the god of the dead.",
		},
		{
			ID:   9,
			Name: "Persona 5 Royal",
			Cover: &Cover{
				ID:  9,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1x7h.webp",
			},
			Summary: "A deep JRPG experience blending traditional RPG gameplay with simulation elements.",
		},
		{
			ID:   10,
			Name: "Elden Ring",
			Cover: &Cover{
				ID:  10,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co4jni.webp",
			},
			Summary: "A fantasy action-RPG adventure set within a world created by Hidetaka Miyazaki and George R.R. Martin.",
		},
		{
			ID:   11,
			Name: "Super Mario Odyssey",
			Cover: &Cover{
				ID:  11,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1p4a.webp",
			},
			Summary: "A 3D platform game where Mario explores kingdoms and collects Power Moons.",
		},
		{
			ID:   12,
			Name: "Sekiro: Shadows Die Twice",
			Cover: &Cover{
				ID:  12,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1rb9.webp",
			},
			Summary: "An action-adventure game focused on stealth, exploration and visceral combat.",
		},
		{
			ID:   13,
			Name: "God of War",
			Cover: &Cover{
				ID:  13,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1tmu.webp",
			},
			Summary: "Follow Kratos and his son Atreus on a deeply personal journey through Norse mythology.",
		},
		{
			ID:   14,
			Name: "Hollow Knight",
			Cover: &Cover{
				ID:  14,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1rgi.webp",
			},
			Summary: "A challenging 2D action-adventure through a vast interconnected world.",
		},
		{
			ID:   15,
			Name: "Animal Crossing: New Horizons",
			Cover: &Cover{
				ID:  15,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1x7z.webp",
			},
			Summary: "A social simulation game where you develop a deserted island into a thriving community.",
		},
		{
			ID:   16,
			Name: "Doom Eternal",
			Cover: &Cover{
				ID:  16,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1tyn.webp",
			},
			Summary: "Hell's armies have invaded Earth. Become the Slayer in an epic single-player campaign.",
		},
		{
			ID:   17,
			Name: "Call of Duty: Modern Warfare",
			Cover: &Cover{
				ID:  17,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1wyq.webp",
			},
			Summary: "The stakes have never been higher as players take on the role of lethal Tier One operators.",
		},
		{
			ID:   18,
			Name: "Final Fantasy XIV",
			Cover: &Cover{
				ID:  18,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co1sf8.webp",
			},
			Summary: "A massively multiplayer online role-playing game set in the fantasy world of Hydaelyn.",
		},
		{
			ID:   19,
			Name: "Overwatch 2",
			Cover: &Cover{
				ID:  19,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co4pzd.webp",
			},
			Summary: "A team-based multiplayer first-person shooter developed by Blizzard Entertainment.",
		},
		{
			ID:   20,
			Name: "Among Us",
			Cover: &Cover{
				ID:  20,
				URL: "https://images.igdb.com/igdb/image/upload/t_cover_big/co2437.webp",
			},
			Summary: "A social deduction game where players work together to complete tasks while identifying impostors.",
		},
	}

	// Filter games based on query (case-insensitive partial matching)
	var matchedGames []GameAPIResponse
	queryLower := strings.ToLower(query)

	for _, game := range allGames {
		if strings.Contains(strings.ToLower(game.Name), queryLower) {
			matchedGames = append(matchedGames, game)
			if len(matchedGames) >= limit {
				break
			}
		}
	}

	// If no matches found by name, return the first few games as suggestions
	if len(matchedGames) == 0 && limit > 1 {
		for i := 0; i < limit && i < len(allGames); i++ {
			matchedGames = append(matchedGames, allGames[i])
		}
	}

	return matchedGames, nil
}
