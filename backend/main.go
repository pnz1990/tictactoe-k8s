package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Business metrics
	gamesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_games_total", Help: "Total games played"},
		[]string{"result"},
	)
	winsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_wins_total", Help: "Wins by player and pattern"},
		[]string{"player", "pattern"},
	)
	playerGamesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_player_games_total", Help: "Games per player"},
		[]string{"player"},
	)
	tiesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "tictactoe_ties_total", Help: "Total tied games"},
	)
	winStreakGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "tictactoe_current_win_streak", Help: "Current win streak"},
		[]string{"player"},
	)
	dynamoDBOps = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "tictactoe_dynamodb_operations_total", Help: "DynamoDB operations"},
		[]string{"operation", "status"},
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

type GameResult struct {
	Player1 string `json:"player1"`
	Player2 string `json:"player2"`
	Winner  string `json:"winner"`
	Pattern string `json:"pattern"`
	IsTie   bool   `json:"isTie"`
}

var (
	winStreaks   = make(map[string]int)
	dynamoClient *dynamodb.Client
	tableName    string
)

func init() {
	prometheus.MustRegister(gamesTotal, winsTotal, playerGamesTotal, tiesTotal, winStreakGauge, dynamoDBOps)
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
		log.Printf("Game saved to DynamoDB: %s", gameId)
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
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
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

	// Save to DynamoDB (async)
	go saveGameToDynamoDB(result)

	playerGamesTotal.WithLabelValues(result.Player1).Inc()
	playerGamesTotal.WithLabelValues(result.Player2).Inc()

	if result.IsTie {
		gamesTotal.WithLabelValues("tie").Inc()
		tiesTotal.Inc()
		winStreaks[result.Player1] = 0
		winStreaks[result.Player2] = 0
		winStreakGauge.WithLabelValues(result.Player1).Set(0)
		winStreakGauge.WithLabelValues(result.Player2).Set(0)
	} else {
		gamesTotal.WithLabelValues("win").Inc()
		winsTotal.WithLabelValues(result.Winner, result.Pattern).Inc()
		loser := result.Player1
		if result.Winner == result.Player1 {
			loser = result.Player2
		}
		winStreaks[result.Winner]++
		winStreaks[loser] = 0
		winStreakGauge.WithLabelValues(result.Winner).Set(float64(winStreaks[result.Winner]))
		winStreakGauge.WithLabelValues(loser).Set(0)
	}

	log.Printf("Game recorded: %+v", result)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
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
	http.HandleFunc("/healthz", metricsMiddleware("/healthz", healthHandler))
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("Backend starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
