package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Business metrics
	gamesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_games_total", Help: "Total games played"},
		[]string{"result", "mode"},
	)
	winsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_wins_total", Help: "Wins by player and pattern"},
		[]string{"player", "pattern", "mode"},
	)
	playerGamesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_player_games_total", Help: "Games per player"},
		[]string{"player", "mode"},
	)
	tiesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_ties_total", Help: "Total tied games"},
		[]string{"mode"},
	)
	winStreakGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "tictactoe_current_win_streak", Help: "Current win streak"},
		[]string{"player"},
	)
	dynamoDBOps = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_dynamodb_operations_total", Help: "DynamoDB operations"},
		[]string{"operation", "status"},
	)
	onlineGamesActive = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "tictactoe_online_games_active", Help: "Active online games"},
	)
	onlineGamesCreated = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "tictactoe_online_games_created_total", Help: "Total online games created"},
	)
	wsConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "tictactoe_websocket_connections_active", Help: "Active WebSocket connections"},
	)
	wsMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_websocket_messages_total", Help: "WebSocket messages"},
		[]string{"type", "direction"},
	)

	// Ops metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"method", "endpoint"},
	)
	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "http_requests_in_flight", Help: "Current in-flight requests"},
	)
)

// Game structures
type GameResult struct {
	Player1 string `json:"player1"`
	Player2 string `json:"player2"`
	Winner  string `json:"winner"`
	Pattern string `json:"pattern"`
	IsTie   bool   `json:"isTie"`
	Mode    string `json:"mode"` // "local" or "online"
}

type Move struct {
	Index  int    `json:"index"`
	Player string `json:"player"`
	Time   int64  `json:"time"` // ms since game start
}

type OnlineGame struct {
	ID          string            `json:"id"`
	Board       [9]string         `json:"board"`
	Turn        string            `json:"turn"`
	FirstPlayer string            `json:"firstPlayer"`
	Player1     string            `json:"player1"`
	Player2     string            `json:"player2"`
	Status      string            `json:"status"` // waiting, playing, finished
	Winner      string            `json:"winner,omitempty"`
	Pattern     string            `json:"pattern,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	StartedAt   time.Time         `json:"startedAt"`
	Moves       []Move            `json:"moves"`
	Conns       []*websocket.Conn `json:"-"`
	mu          sync.Mutex        `json:"-"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

var (
	winStreaks   = make(map[string]int)
	dynamoClient *dynamodb.Client
	tableName    string
	games        = make(map[string]*OnlineGame)
	gamesMu      sync.RWMutex
	upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func init() {
	prometheus.MustRegister(gamesTotal, winsTotal, playerGamesTotal, tiesTotal, winStreakGauge, dynamoDBOps)
	prometheus.MustRegister(onlineGamesActive, onlineGamesCreated, wsConnectionsActive, wsMessagesTotal)
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpRequestsInFlight)
}

func initDynamoDB() {
	tableName = os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		log.Println("DYNAMODB_TABLE not set, game persistence disabled")
		return
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("Failed to load AWS config: %v", err)
		return
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	log.Printf("DynamoDB client initialized for table: %s", tableName)
}

func saveGameToDynamoDB(result GameResult) {
	if dynamoClient == nil {
		return
	}
	gameId := uuid.New().String()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	item := map[string]types.AttributeValue{
		"gameId":    &types.AttributeValueMemberS{Value: gameId},
		"timestamp": &types.AttributeValueMemberS{Value: timestamp},
		"player1":   &types.AttributeValueMemberS{Value: result.Player1},
		"player2":   &types.AttributeValueMemberS{Value: result.Player2},
		"isTie":     &types.AttributeValueMemberBOOL{Value: result.IsTie},
		"mode":      &types.AttributeValueMemberS{Value: result.Mode},
	}
	if !result.IsTie {
		item["winner"] = &types.AttributeValueMemberS{Value: result.Winner}
		item["pattern"] = &types.AttributeValueMemberS{Value: result.Pattern}
	}
	_, err := dynamoClient.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("Failed to save game to DynamoDB: %v", err)
		dynamoDBOps.WithLabelValues("PutItem", "error").Inc()
	} else {
		dynamoDBOps.WithLabelValues("PutItem", "success").Inc()
	}
}

func saveOnlineGameToDynamoDB(g *OnlineGame) {
	if dynamoClient == nil {
		return
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	duration := int64(0)
	if len(g.Moves) > 0 {
		duration = g.Moves[len(g.Moves)-1].Time
	}

	// Convert moves to DynamoDB list
	movesList := make([]types.AttributeValue, len(g.Moves))
	for i, m := range g.Moves {
		movesList[i] = &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"index":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", m.Index)},
			"player": &types.AttributeValueMemberS{Value: m.Player},
			"time":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", m.Time)},
		}}
	}

	item := map[string]types.AttributeValue{
		"gameId":    &types.AttributeValueMemberS{Value: g.ID},
		"timestamp": &types.AttributeValueMemberS{Value: timestamp},
		"player1":   &types.AttributeValueMemberS{Value: g.Player1},
		"player2":   &types.AttributeValueMemberS{Value: g.Player2},
		"isTie":     &types.AttributeValueMemberBOOL{Value: g.Winner == ""},
		"mode":      &types.AttributeValueMemberS{Value: "online"},
		"moves":     &types.AttributeValueMemberL{Value: movesList},
		"duration":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", duration)},
	}
	if g.Winner != "" {
		item["winner"] = &types.AttributeValueMemberS{Value: g.Winner}
		item["pattern"] = &types.AttributeValueMemberS{Value: g.Pattern}
	}
	_, err := dynamoClient.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("Failed to save online game to DynamoDB: %v", err)
		dynamoDBOps.WithLabelValues("PutItem", "error").Inc()
	} else {
		dynamoDBOps.WithLabelValues("PutItem", "success").Inc()
	}
}

func metricsMiddleware(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()
		rw := &responseWriter{w, http.StatusOK}
		next(rw, r)
		httpRequestsTotal.WithLabelValues(r.Method, endpoint, http.StatusText(rw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, endpoint).Observe(time.Since(start).Seconds())
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var result GameResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if result.Mode == "" {
		result.Mode = "local"
	}
	go saveGameToDynamoDB(result)
	recordMetrics(result)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
}

func recordMetrics(result GameResult) {
	playerGamesTotal.WithLabelValues(result.Player1, result.Mode).Inc()
	playerGamesTotal.WithLabelValues(result.Player2, result.Mode).Inc()
	if result.IsTie {
		gamesTotal.WithLabelValues("tie", result.Mode).Inc()
		tiesTotal.WithLabelValues(result.Mode).Inc()
		winStreaks[result.Player1] = 0
		winStreaks[result.Player2] = 0
		winStreakGauge.WithLabelValues(result.Player1).Set(0)
		winStreakGauge.WithLabelValues(result.Player2).Set(0)
	} else {
		gamesTotal.WithLabelValues("win", result.Mode).Inc()
		winsTotal.WithLabelValues(result.Winner, result.Pattern, result.Mode).Inc()
		loser := result.Player1
		if result.Winner == result.Player1 {
			loser = result.Player2
		}
		winStreaks[result.Winner]++
		winStreaks[loser] = 0
		winStreakGauge.WithLabelValues(result.Winner).Set(float64(winStreaks[result.Winner]))
		winStreakGauge.WithLabelValues(loser).Set(0)
	}
}

// Online game handlers
func createGameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Player1 string `json:"player1"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Player1 == "" {
		http.Error(w, "player1 required", http.StatusBadRequest)
		return
	}
	// Coin flip: random first player
	firstPlayer := "X"
	if rand.Intn(2) == 1 {
		firstPlayer = "O"
	}
	game := &OnlineGame{
		ID:          uuid.New().String()[:8],
		Board:       [9]string{},
		Turn:        firstPlayer,
		FirstPlayer: firstPlayer,
		Player1:     req.Player1,
		Status:      "waiting",
		CreatedAt:   time.Now(),
	}
	gamesMu.Lock()
	games[game.ID] = game
	gamesMu.Unlock()
	onlineGamesCreated.Inc()
	onlineGamesActive.Inc()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"gameId": game.ID, "firstPlayer": game.FirstPlayer})
}

func joinGameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GameID  string `json:"gameId"`
		Player2 string `json:"player2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	gamesMu.Lock()
	game, exists := games[req.GameID]
	if !exists {
		gamesMu.Unlock()
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	if game.Status != "waiting" {
		gamesMu.Unlock()
		http.Error(w, "Game already started", http.StatusBadRequest)
		return
	}
	game.Player2 = req.Player2
	game.Status = "playing"
	game.StartedAt = time.Now()
	gamesMu.Unlock()
	game.broadcast(WSMessage{Type: "game_start", Payload: game.toJSON()})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game.toJSON())
}

func getGameHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("id")
	gamesMu.RLock()
	game, exists := games[gameID]
	gamesMu.RUnlock()
	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game.toJSON())
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("id")
	gamesMu.RLock()
	game, exists := games[gameID]
	gamesMu.RUnlock()
	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	wsConnectionsActive.Inc()
	game.mu.Lock()
	game.Conns = append(game.Conns, conn)
	game.mu.Unlock()
	conn.WriteJSON(WSMessage{Type: "game_state", Payload: game.toJSON()})
	wsMessagesTotal.WithLabelValues("game_state", "out").Inc()
	defer func() {
		wsConnectionsActive.Dec()
		conn.Close()
		game.mu.Lock()
		for i, c := range game.Conns {
			if c == conn {
				game.Conns = append(game.Conns[:i], game.Conns[i+1:]...)
				break
			}
		}
		game.mu.Unlock()
	}()
	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		wsMessagesTotal.WithLabelValues(msg.Type, "in").Inc()
		game.handleMessage(msg)
	}
}

func (g *OnlineGame) toJSON() map[string]interface{} {
	return map[string]interface{}{
		"id": g.ID, "board": g.Board, "turn": g.Turn, "firstPlayer": g.FirstPlayer,
		"player1": g.Player1, "player2": g.Player2,
		"status": g.Status, "winner": g.Winner, "pattern": g.Pattern,
	}
}

func (g *OnlineGame) broadcast(msg WSMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	wsMessagesTotal.WithLabelValues(msg.Type, "out").Add(float64(len(g.Conns)))
	for _, conn := range g.Conns {
		conn.WriteJSON(msg)
	}
}

func (g *OnlineGame) handleMessage(msg WSMessage) {
	if msg.Type != "move" || g.Status != "playing" {
		return
	}
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}
	idx := int(payload["index"].(float64))
	player := payload["player"].(string)
	expectedPlayer := g.Player1
	if g.Turn == "O" {
		expectedPlayer = g.Player2
	}
	if player != expectedPlayer || idx < 0 || idx > 8 || g.Board[idx] != "" {
		return
	}
	g.Board[idx] = g.Turn

	// Record move with timestamp
	moveTime := int64(0)
	if !g.StartedAt.IsZero() {
		moveTime = time.Since(g.StartedAt).Milliseconds()
	}
	g.Moves = append(g.Moves, Move{Index: idx, Player: g.Turn, Time: moveTime})

	wins := [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {0, 3, 6}, {1, 4, 7}, {2, 5, 8}, {0, 4, 8}, {2, 4, 6}}
	patterns := map[string]string{
		"0,1,2": "row1", "3,4,5": "row2", "6,7,8": "row3",
		"0,3,6": "col1", "1,4,7": "col2", "2,5,8": "col3",
		"0,4,8": "diag1", "2,4,6": "diag2",
	}
	for _, w := range wins {
		if g.Board[w[0]] != "" && g.Board[w[0]] == g.Board[w[1]] && g.Board[w[1]] == g.Board[w[2]] {
			g.Status = "finished"
			g.Winner = player
			key := fmt.Sprintf("%d,%d,%d", w[0], w[1], w[2])
			g.Pattern = patterns[key]
			g.broadcast(WSMessage{Type: "game_state", Payload: g.toJSON()})
			go saveOnlineGameToDynamoDB(g)
			result := GameResult{Player1: g.Player1, Player2: g.Player2, Winner: g.Winner, Pattern: g.Pattern, Mode: "online"}
			recordMetrics(result)
			onlineGamesActive.Dec()
			return
		}
	}
	isFull := true
	for _, c := range g.Board {
		if c == "" {
			isFull = false
			break
		}
	}
	if isFull {
		g.Status = "finished"
		g.broadcast(WSMessage{Type: "game_state", Payload: g.toJSON()})
		go saveOnlineGameToDynamoDB(g)
		result := GameResult{Player1: g.Player1, Player2: g.Player2, IsTie: true, Mode: "online"}
		recordMetrics(result)
		onlineGamesActive.Dec()
		return
	}
	if g.Turn == "X" {
		g.Turn = "O"
	} else {
		g.Turn = "X"
	}
	g.broadcast(WSMessage{Type: "game_state", Payload: g.toJSON()})
}

// Leaderboard structures
type PlayerStats struct {
	Player      string  `json:"player"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	Ties        int     `json:"ties"`
	TotalGames  int     `json:"totalGames"`
	WinRate     float64 `json:"winRate"`
	WinStreak   int     `json:"winStreak"`
	BestPattern string  `json:"bestPattern,omitempty"`
}

type LeaderboardResponse struct {
	Players   []PlayerStats `json:"players"`
	UpdatedAt string        `json:"updatedAt"`
}

type RecentGame struct {
	GameID    string `json:"gameId"`
	Player1   string `json:"player1"`
	Player2   string `json:"player2"`
	Winner    string `json:"winner,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	IsTie     bool   `json:"isTie"`
	Mode      string `json:"mode"`
	Timestamp string `json:"timestamp"`
}

type StatsResponse struct {
	TotalGames      int            `json:"totalGames"`
	TotalWins       int            `json:"totalWins"`
	TotalTies       int            `json:"totalTies"`
	TopPatterns     map[string]int `json:"topPatterns"`
	UpdatedAt       string         `json:"updatedAt"`
	AvgMovesPerGame float64        `json:"avgMovesPerGame"`
	XWinRate        float64        `json:"xWinRate"`
	OWinRate        float64        `json:"oWinRate"`
	TieRate         float64        `json:"tieRate"`
	MostActiveHour  int            `json:"mostActiveHour"`
	LongestStreak   int            `json:"longestStreak"`
	StreakHolder    string         `json:"streakHolder"`
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if dynamoClient == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Scan all online games and aggregate stats
	playerStats := make(map[string]*PlayerStats)
	playerPatterns := make(map[string]map[string]int) // player -> pattern -> count
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:         aws.String(tableName),
			ExclusiveStartKey: lastKey,
		}
		result, err := dynamoClient.Scan(context.Background(), input)
		if err != nil {
			log.Printf("Scan error: %v", err)
			dynamoDBOps.WithLabelValues("Scan", "error").Inc()
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		dynamoDBOps.WithLabelValues("Scan", "success").Inc()

		for _, item := range result.Items {
			mode := getStringAttr(item, "mode")
			// Only count online games for leaderboard
			if mode != "online" {
				continue
			}

			p1 := getStringAttr(item, "player1")
			p2 := getStringAttr(item, "player2")
			winner := getStringAttr(item, "winner")
			pattern := getStringAttr(item, "pattern")
			isTie := getBoolAttr(item, "isTie")

			// Skip synthetic test data
			if len(p1) >= 9 && p1[:9] == "Synthetic" {
				continue
			}

			ensurePlayer(playerStats, p1)
			ensurePlayer(playerStats, p2)
			if playerPatterns[p1] == nil {
				playerPatterns[p1] = make(map[string]int)
			}
			if playerPatterns[p2] == nil {
				playerPatterns[p2] = make(map[string]int)
			}

			if isTie {
				playerStats[p1].Ties++
				playerStats[p2].Ties++
			} else if winner != "" {
				playerStats[winner].Wins++
				if pattern != "" {
					playerPatterns[winner][pattern]++
				}
				loser := p1
				if winner == p1 {
					loser = p2
				}
				playerStats[loser].Losses++
			}
			playerStats[p1].TotalGames++
			playerStats[p2].TotalGames++
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	// Convert to slice, calculate win rates and best patterns
	players := make([]PlayerStats, 0, len(playerStats))
	for name, ps := range playerStats {
		if ps.TotalGames > 0 {
			ps.WinRate = float64(ps.Wins) / float64(ps.TotalGames) * 100
		}
		// Find best pattern
		if patterns, ok := playerPatterns[name]; ok {
			maxCount := 0
			for p, c := range patterns {
				if c > maxCount {
					maxCount = c
					ps.BestPattern = p
				}
			}
		}
		// Get current streak from memory
		if streak, ok := winStreaks[name]; ok {
			ps.WinStreak = streak
		}
		players = append(players, *ps)
	}

	// Sort by wins descending
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			if players[j].Wins > players[i].Wins {
				players[i], players[j] = players[j], players[i]
			}
		}
	}

	// Limit to top 20
	if len(players) > 20 {
		players = players[:20]
	}

	resp := LeaderboardResponse{
		Players:   players,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if dynamoClient == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var totalGames, totalWins, totalTies, xWins, oWins int
	patterns := make(map[string]int)
	hourCounts := make(map[int]int)
	playerWinStreaks := make(map[string]int)
	longestStreak, streakHolder := 0, ""
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:         aws.String(tableName),
			ExclusiveStartKey: lastKey,
		}
		result, err := dynamoClient.Scan(context.Background(), input)
		if err != nil {
			dynamoDBOps.WithLabelValues("Scan", "error").Inc()
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		dynamoDBOps.WithLabelValues("Scan", "success").Inc()

		for _, item := range result.Items {
			mode := getStringAttr(item, "mode")
			if mode != "online" {
				continue
			}
			p1 := getStringAttr(item, "player1")
			if len(p1) >= 9 && p1[:9] == "Synthetic" {
				continue
			}
			totalGames++

			// Track hour of play
			ts := getStringAttr(item, "timestamp")
			if len(ts) >= 13 {
				hour := 0
				fmt.Sscanf(ts[11:13], "%d", &hour)
				hourCounts[hour]++
			}

			if getBoolAttr(item, "isTie") {
				totalTies++
			} else {
				totalWins++
				winner := getStringAttr(item, "winner")
				pattern := getStringAttr(item, "pattern")
				if pattern != "" {
					patterns[pattern]++
				}
				// X always goes first, so winner == p1 means X won
				if winner == p1 {
					xWins++
				} else {
					oWins++
				}
				// Track streaks
				playerWinStreaks[winner]++
				if playerWinStreaks[winner] > longestStreak {
					longestStreak = playerWinStreaks[winner]
					streakHolder = winner
				}
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	// Find most active hour
	mostActiveHour, maxHourCount := 0, 0
	for h, c := range hourCounts {
		if c > maxHourCount {
			maxHourCount = c
			mostActiveHour = h
		}
	}

	// Calculate rates
	var xRate, oRate, tieRate float64
	if totalGames > 0 {
		xRate = float64(xWins) / float64(totalGames) * 100
		oRate = float64(oWins) / float64(totalGames) * 100
		tieRate = float64(totalTies) / float64(totalGames) * 100
	}

	resp := StatsResponse{
		TotalGames:     totalGames,
		TotalWins:      totalWins,
		TotalTies:      totalTies,
		TopPatterns:    patterns,
		XWinRate:       xRate,
		OWinRate:       oRate,
		TieRate:        tieRate,
		MostActiveHour: mostActiveHour,
		LongestStreak:  longestStreak,
		StreakHolder:   streakHolder,
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func recentGamesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if dynamoClient == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(100),
	}
	result, err := dynamoClient.Scan(context.Background(), input)
	if err != nil {
		dynamoDBOps.WithLabelValues("Scan", "error").Inc()
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	dynamoDBOps.WithLabelValues("Scan", "success").Inc()

	games := make([]RecentGame, 0)
	for _, item := range result.Items {
		mode := getStringAttr(item, "mode")
		if mode != "online" {
			continue
		}
		p1 := getStringAttr(item, "player1")
		if len(p1) >= 9 && p1[:9] == "Synthetic" {
			continue
		}
		games = append(games, RecentGame{
			GameID:    getStringAttr(item, "gameId"),
			Player1:   p1,
			Player2:   getStringAttr(item, "player2"),
			Winner:    getStringAttr(item, "winner"),
			Pattern:   getStringAttr(item, "pattern"),
			IsTie:     getBoolAttr(item, "isTie"),
			Mode:      mode,
			Timestamp: getStringAttr(item, "timestamp"),
		})
	}

	// Sort by timestamp descending
	for i := 0; i < len(games); i++ {
		for j := i + 1; j < len(games); j++ {
			if games[j].Timestamp > games[i].Timestamp {
				games[i], games[j] = games[j], games[i]
			}
		}
	}

	if len(games) > 20 {
		games = games[:20]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

func playerStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	player := r.URL.Query().Get("player")
	if player == "" {
		http.Error(w, "player parameter required", http.StatusBadRequest)
		return
	}
	if dynamoClient == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	stats := &PlayerStats{Player: player}
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:         aws.String(tableName),
			ExclusiveStartKey: lastKey,
			FilterExpression:  aws.String("player1 = :p OR player2 = :p"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":p": &types.AttributeValueMemberS{Value: player},
			},
		}
		result, err := dynamoClient.Scan(context.Background(), input)
		if err != nil {
			dynamoDBOps.WithLabelValues("Scan", "error").Inc()
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		dynamoDBOps.WithLabelValues("Scan", "success").Inc()

		for _, item := range result.Items {
			stats.TotalGames++
			if getBoolAttr(item, "isTie") {
				stats.Ties++
			} else {
				winner := getStringAttr(item, "winner")
				if winner == player {
					stats.Wins++
				} else {
					stats.Losses++
				}
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func getStringAttr(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func getBoolAttr(item map[string]types.AttributeValue, key string) bool {
	if v, ok := item[key].(*types.AttributeValueMemberBOOL); ok {
		return v.Value
	}
	return false
}

func ensurePlayer(stats map[string]*PlayerStats, player string) {
	if _, ok := stats[player]; !ok {
		stats[player] = &PlayerStats{Player: player}
	}
}

// Game replay response
type GameReplay struct {
	GameID    string `json:"gameId"`
	Player1   string `json:"player1"`
	Player2   string `json:"player2"`
	Winner    string `json:"winner,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	IsTie     bool   `json:"isTie"`
	Timestamp string `json:"timestamp"`
	Duration  int64  `json:"duration"`
	Moves     []Move `json:"moves"`
}

func gameReplayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	gameID := r.URL.Query().Get("id")
	if gameID == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}
	if dynamoClient == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Query by gameId
	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("gameId = :gid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":gid": &types.AttributeValueMemberS{Value: gameID},
		},
		Limit: aws.Int32(1),
	}
	result, err := dynamoClient.Query(context.Background(), input)
	if err != nil {
		dynamoDBOps.WithLabelValues("Query", "error").Inc()
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	dynamoDBOps.WithLabelValues("Query", "success").Inc()

	if len(result.Items) == 0 {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	item := result.Items[0]
	replay := GameReplay{
		GameID:    getStringAttr(item, "gameId"),
		Player1:   getStringAttr(item, "player1"),
		Player2:   getStringAttr(item, "player2"),
		Winner:    getStringAttr(item, "winner"),
		Pattern:   getStringAttr(item, "pattern"),
		IsTie:     getBoolAttr(item, "isTie"),
		Timestamp: getStringAttr(item, "timestamp"),
		Duration:  getIntAttr(item, "duration"),
		Moves:     getMovesAttr(item, "moves"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(replay)
}

func playerGamesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	player := r.URL.Query().Get("player")
	if player == "" {
		http.Error(w, "player parameter required", http.StatusBadRequest)
		return
	}
	if dynamoClient == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	input := &dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("(player1 = :p OR player2 = :p) AND #m = :online"),
		ExpressionAttributeNames: map[string]string{
			"#m": "mode",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":p":      &types.AttributeValueMemberS{Value: player},
			":online": &types.AttributeValueMemberS{Value: "online"},
		},
	}
	result, err := dynamoClient.Scan(context.Background(), input)
	if err != nil {
		dynamoDBOps.WithLabelValues("Scan", "error").Inc()
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	dynamoDBOps.WithLabelValues("Scan", "success").Inc()

	games := make([]RecentGame, 0)
	for _, item := range result.Items {
		games = append(games, RecentGame{
			GameID:    getStringAttr(item, "gameId"),
			Player1:   getStringAttr(item, "player1"),
			Player2:   getStringAttr(item, "player2"),
			Winner:    getStringAttr(item, "winner"),
			Pattern:   getStringAttr(item, "pattern"),
			IsTie:     getBoolAttr(item, "isTie"),
			Mode:      "online",
			Timestamp: getStringAttr(item, "timestamp"),
		})
	}

	// Sort by timestamp descending
	for i := 0; i < len(games); i++ {
		for j := i + 1; j < len(games); j++ {
			if games[j].Timestamp > games[i].Timestamp {
				games[i], games[j] = games[j], games[i]
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

func getIntAttr(item map[string]types.AttributeValue, key string) int64 {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		var n int64
		fmt.Sscanf(v.Value, "%d", &n)
		return n
	}
	return 0
}

func getMovesAttr(item map[string]types.AttributeValue, key string) []Move {
	moves := make([]Move, 0)
	if v, ok := item[key].(*types.AttributeValueMemberL); ok {
		for _, m := range v.Value {
			if mv, ok := m.(*types.AttributeValueMemberM); ok {
				var idx int
				var t int64
				if n, ok := mv.Value["index"].(*types.AttributeValueMemberN); ok {
					fmt.Sscanf(n.Value, "%d", &idx)
				}
				if n, ok := mv.Value["time"].(*types.AttributeValueMemberN); ok {
					fmt.Sscanf(n.Value, "%d", &t)
				}
				player := ""
				if s, ok := mv.Value["player"].(*types.AttributeValueMemberS); ok {
					player = s.Value
				}
				moves = append(moves, Move{Index: idx, Player: player, Time: t})
			}
		}
	}
	return moves
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func main() {
	initDynamoDB()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	http.HandleFunc("/api/game", metricsMiddleware("/api/game", corsMiddleware(gameHandler)))
	http.HandleFunc("/api/game/create", metricsMiddleware("/api/game/create", corsMiddleware(createGameHandler)))
	http.HandleFunc("/api/game/join", metricsMiddleware("/api/game/join", corsMiddleware(joinGameHandler)))
	http.HandleFunc("/api/game/get", metricsMiddleware("/api/game/get", corsMiddleware(getGameHandler)))
	http.HandleFunc("/api/game/ws", wsHandler)
	http.HandleFunc("/api/leaderboard", metricsMiddleware("/api/leaderboard", corsMiddleware(leaderboardHandler)))
	http.HandleFunc("/api/stats", metricsMiddleware("/api/stats", corsMiddleware(statsHandler)))
	http.HandleFunc("/api/recent", metricsMiddleware("/api/recent", corsMiddleware(recentGamesHandler)))
	http.HandleFunc("/api/player", metricsMiddleware("/api/player", corsMiddleware(playerStatsHandler)))
	http.HandleFunc("/api/player/games", metricsMiddleware("/api/player/games", corsMiddleware(playerGamesHandler)))
	http.HandleFunc("/api/replay", metricsMiddleware("/api/replay", corsMiddleware(gameReplayHandler)))
	http.HandleFunc("/healthz", metricsMiddleware("/healthz", healthHandler))
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Backend starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
