package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

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

var winStreaks = make(map[string]int)

func init() {
	prometheus.MustRegister(gamesTotal, winsTotal, playerGamesTotal, tiesTotal, winStreakGauge)
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpRequestsInFlight)
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
