// K6 Load Testing Script
// Run with: k6 run load_test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const gameSubmissionTime = new Trend('game_submission_duration');

// Test configuration - lenient thresholds for CI
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up to 10 users
    { duration: '1m', target: 50 },   // Ramp up to 50 users
    { duration: '2m', target: 50 },   // Stay at 50 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'], // 95% < 1s, 99% < 2s
    http_req_failed: ['rate<0.30'],                   // Error rate < 30%
    errors: ['rate<0.30'],                            // Custom error rate < 30%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';

const players = ['Alice', 'Bob', 'Charlie', 'Diana', 'Eve', 'Frank'];
const patterns = ['row1', 'row2', 'row3', 'col1', 'col2', 'col3', 'diag1', 'diag2'];

function getRandomPlayer() {
  return players[Math.floor(Math.random() * players.length)];
}

function getRandomPattern() {
  return patterns[Math.floor(Math.random() * patterns.length)];
}

export default function () {
  const player1 = getRandomPlayer();
  let player2 = getRandomPlayer();
  while (player2 === player1) {
    player2 = getRandomPlayer();
  }

  const isTie = Math.random() < 0.2; // 20% chance of tie
  const winner = isTie ? '' : (Math.random() < 0.5 ? player1 : player2);
  const pattern = isTie ? '' : getRandomPattern();

  const payload = JSON.stringify({
    player1: player1,
    player2: player2,
    winner: winner,
    pattern: pattern,
    isTie: isTie,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const start = new Date();
  const res = http.post(`${BASE_URL}/api/game`, payload, params);
  const duration = new Date() - start;

  gameSubmissionTime.add(duration);

  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'response time < 1000ms': (r) => r.timings.duration < 1000,
  });

  errorRate.add(!success);

  sleep(1); // Think time between requests
}

// Spike test scenario
export function spikeTest() {
  const payload = JSON.stringify({
    player1: 'Alice',
    player2: 'Bob',
    winner: 'Alice',
    pattern: 'row1',
    isTie: false,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.post(`${BASE_URL}/api/game`, payload, params);
  
  check(res, {
    'spike test - status is 200': (r) => r.status === 200,
    'spike test - response time < 2000ms': (r) => r.timings.duration < 2000,
  });
}

// Stress test configuration
export const stressOptions = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 200 },  // Ramp up to 200 users
    { duration: '5m', target: 200 },  // Stay at 200 users
    { duration: '2m', target: 0 },    // Ramp down
  ],
};
