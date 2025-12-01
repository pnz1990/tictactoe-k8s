package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

type OnlineGame struct {
	ID        string          `json:"id"`
	Board     [9]string       `json:"board"`
	Turn      string          `json:"turn"`
	Player1   string          `json:"player1"`
	Player2   string          `json:"player2"`
	Status    string          `json:"status"` // waiting, playing, finished
	Winner    string          `json:"winner,omitempty"`
	Pattern   string          `json:"pattern,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	Conns     []*websocket.Conn `json:"-"`
	mu        sync.Mutex      `json:"-"`
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
	game := &OnlineGame{
		ID:        uuid.New().String()[:8],
		Board:     [9]string{},
		Turn:      "X",
		Player1:   req.Player1,
		Status:    "waiting",
		CreatedAt: time.Now(),
	}
	gamesMu.Lock()
	games[game.ID] = game
	gamesMu.Unlock()
	onlineGamesCreated.Inc()
	onlineGamesActive.Inc()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"gameId": game.ID})
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
		"id": g.ID, "board": g.Board, "turn": g.Turn,
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
			result := GameResult{Player1: g.Player1, Player2: g.Player2, Winner: g.Winner, Pattern: g.Pattern, Mode: "online"}
			go saveGameToDynamoDB(result)
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
		result := GameResult{Player1: g.Player1, Player2: g.Player2, IsTie: true, Mode: "online"}
		go saveGameToDynamoDB(result)
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
	http.HandleFunc("/healthz", metricsMiddleware("/healthz", healthHandler))
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Backend starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
