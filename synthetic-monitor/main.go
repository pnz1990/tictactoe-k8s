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
	"github.com/prometheus/client_golang/prometheus/push"
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
	resp, err := http.Get(albURL + "/api/healthz")
	if err != nil {
		// Try alternative endpoint
		resp, err = http.Get(albURL + "/api/game")
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
	}
	defer resp.Body.Close()

	// Backend returns 405 for GET on /api/game which is expected
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
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

func testMetricsEndpoint(backendMetricsURL string) error {
	resp, err := http.Get(backendMetricsURL)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) < 100 {
		return fmt.Errorf("metrics response too short")
	}

	return nil
}

func main() {
	albURL := os.Getenv("ALB_URL")
	env := os.Getenv("ENVIRONMENT")
	pushgatewayURL := os.Getenv("PUSHGATEWAY_URL")
	backendMetricsURL := os.Getenv("BACKEND_METRICS_URL")

	if albURL == "" || env == "" {
		log.Fatal("ALB_URL and ENVIRONMENT must be set")
	}

	log.Printf("Starting synthetic tests for %s environment", env)
	log.Printf("ALB URL: %s", albURL)

	// Run tests
	runTest("frontend_health", env, func() error { return testFrontendHealth(albURL) })
	runTest("backend_health", env, func() error { return testBackendHealth(albURL) })
	runTest("game_recording", env, func() error { return testGameRecording(albURL) })

	if backendMetricsURL != "" {
		runTest("metrics_endpoint", env, func() error { return testMetricsEndpoint(backendMetricsURL) })
	}

	testTimestamp.WithLabelValues(env).Set(float64(time.Now().Unix()))

	// Push metrics if pushgateway is configured
	if pushgatewayURL != "" {
		pusher := push.New(pushgatewayURL, "synthetic_monitor").
			Grouping("environment", env).
			Gatherer(prometheus.DefaultGatherer)

		if err := pusher.Push(); err != nil {
			log.Printf("Failed to push metrics: %v", err)
		} else {
			log.Printf("Metrics pushed to %s", pushgatewayURL)
		}
	}

	log.Println("Synthetic tests completed")
}
