package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	testResult = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "synthetic_test_success", Help: "Synthetic test result (1=success, 0=failure)"},
		[]string{"test", "environment"},
	)
	testDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "synthetic_test_duration_seconds", Help: "Synthetic test duration in seconds"},
		[]string{"test", "environment"},
	)
	testTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "synthetic_test_timestamp", Help: "Synthetic test last run timestamp"},
		[]string{"environment"},
	)
)

func init() {
	prometheus.MustRegister(testResult, testDuration, testTimestamp)
}

type GameResult struct {
	Player1 string `json:"player1"`
	Player2 string `json:"player2"`
	Winner  string `json:"winner"`
	Pattern string `json:"pattern"`
	IsTie   bool   `json:"isTie"`
	Mode    string `json:"mode"`
}

func runTest(name string, env string, testFunc func() error) {
	start := time.Now()
	err := testFunc()
	duration := time.Since(start).Seconds()
	if err != nil {
		log.Printf("❌ %s: FAILED - %v (%.2fs)", name, err, duration)
		testResult.WithLabelValues(name, env).Set(0)
	} else {
		log.Printf("✅ %s: PASSED (%.2fs)", name, duration)
		testResult.WithLabelValues(name, env).Set(1)
	}
	testDuration.WithLabelValues(name, env).Set(duration)
}

func testFrontendHealth(url string) error {
	resp, err := http.Get(url + "/healthz")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func testBackendHealth(url string) error {
	resp, err := http.Get(url + "/api/game")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("unexpected status: %d (expected 405)", resp.StatusCode)
	}
	return nil
}

func testLocalGameRecording(url string) error {
	game := GameResult{Player1: "SyntheticA", Player2: "SyntheticB", Winner: "SyntheticA", Pattern: "row1", Mode: "local"}
	body, _ := json.Marshal(game)
	resp, err := http.Post(url+"/api/game", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func testOnlineGameCreate(url string) error {
	body, _ := json.Marshal(map[string]string{"player1": "SyntheticOnline"})
	resp, err := http.Post(url+"/api/game/create", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode: %w", err)
	}
	if result["gameId"] == "" {
		return fmt.Errorf("no gameId returned")
	}
	return nil
}

func testOnlineGameFlow(url string) error {
	// Create game
	body, _ := json.Marshal(map[string]string{"player1": "SyntheticP1"})
	resp, err := http.Post(url+"/api/game/create", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	defer resp.Body.Close()
	var createRes map[string]string
	json.NewDecoder(resp.Body).Decode(&createRes)
	gameId := createRes["gameId"]
	
	// Join game
	joinBody, _ := json.Marshal(map[string]string{"gameId": gameId, "player2": "SyntheticP2"})
	resp2, err := http.Post(url+"/api/game/join", "application/json", bytes.NewReader(joinBody))
	if err != nil {
		return fmt.Errorf("join failed: %w", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("join status: %d", resp2.StatusCode)
	}
	
	// Get game state
	resp3, err := http.Get(url + "/api/game/get?id=" + gameId)
	if err != nil {
		return fmt.Errorf("get failed: %w", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		return fmt.Errorf("get status: %d", resp3.StatusCode)
	}
	return nil
}

func runAllTests(frontendURL, backendURL, env string) {
	log.Printf("Running synthetic tests for %s", env)
	runTest("frontend_health", env, func() error { return testFrontendHealth(frontendURL) })
	runTest("backend_health", env, func() error { return testBackendHealth(backendURL) })
	runTest("local_game_recording", env, func() error { return testLocalGameRecording(backendURL) })
	runTest("online_game_create", env, func() error { return testOnlineGameCreate(backendURL) })
	runTest("online_game_flow", env, func() error { return testOnlineGameFlow(backendURL) })
	testTimestamp.WithLabelValues(env).Set(float64(time.Now().Unix()))
	log.Printf("Synthetic tests completed")
}

func main() {
	frontendURL := os.Getenv("FRONTEND_URL")
	backendURL := os.Getenv("BACKEND_URL")
	env := os.Getenv("ENVIRONMENT")
	interval := os.Getenv("TEST_INTERVAL")
	if interval == "" {
		interval = "60s"
	}
	if frontendURL == "" || backendURL == "" || env == "" {
		log.Fatal("FRONTEND_URL, BACKEND_URL and ENVIRONMENT must be set")
	}
	testInterval, _ := time.ParseDuration(interval)
	if testInterval == 0 {
		testInterval = 60 * time.Second
	}
	log.Printf("Starting synthetic monitor for %s", env)
	runAllTests(frontendURL, backendURL, env)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
		log.Fatal(http.ListenAndServe(":9091", nil))
	}()
	ticker := time.NewTicker(testInterval)
	for range ticker.C {
		runAllTests(frontendURL, backendURL, env)
	}
}
