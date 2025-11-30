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
		prometheus.GaugeOpts{
			Name: "synthetic_test_success",
			Help: "Synthetic test result (1=success, 0=failure)",
		},
		[]string{"test", "environment"},
	)
	testDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synthetic_test_duration_seconds",
			Help: "Synthetic test duration in seconds",
		},
		[]string{"test", "environment"},
	)
	testTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synthetic_test_timestamp",
			Help: "Synthetic test last run timestamp",
		},
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

func testFrontendHealth(albURL string) error {
	resp, err := http.Get(albURL + "/healthz")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func testBackendHealth(albURL string) error {
	resp, err := http.Get(albURL + "/api/game")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Backend returns 405 for GET on /api/game which means it's alive
	if resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("unexpected status: %d (expected 405)", resp.StatusCode)
	}
	return nil
}

func testGameRecording(albURL string) error {
	game := GameResult{
		Player1: "SyntheticA",
		Player2: "SyntheticB",
		Winner:  "SyntheticA",
		Pattern: "row1",
		IsTie:   false,
	}

	body, _ := json.Marshal(game)
	resp, err := http.Post(albURL+"/api/game", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result["status"] != "recorded" {
		return fmt.Errorf("unexpected response: %v", result)
	}

	return nil
}

func runAllTests(frontendURL, backendURL, env string) {
	log.Printf("Running synthetic tests for %s", env)
	runTest("frontend_health", env, func() error { return testFrontendHealth(frontendURL) })
	runTest("backend_health", env, func() error { return testBackendHealth(backendURL) })
	runTest("game_recording", env, func() error { return testGameRecording(backendURL) })
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

	testInterval, err := time.ParseDuration(interval)
	if err != nil {
		testInterval = 60 * time.Second
	}

	log.Printf("Starting synthetic monitor for %s environment", env)
	log.Printf("Frontend URL: %s", frontendURL)
	log.Printf("Backend URL: %s", backendURL)
	log.Printf("Test interval: %s", testInterval)

	// Run tests immediately
	runAllTests(frontendURL, backendURL, env)

	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		log.Printf("Metrics server starting on :9091")
		log.Fatal(http.ListenAndServe(":9091", nil))
	}()

	// Run tests periodically
	ticker := time.NewTicker(testInterval)
	for range ticker.C {
		runAllTests(frontendURL, backendURL, env)
	}
}
