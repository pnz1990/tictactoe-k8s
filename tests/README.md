# Test Suite

Comprehensive test suite for the Tic Tac Toe Kubernetes application.

## Test Categories

### Unit Tests (`backend/main_test.go`)
Tests for Go backend business logic:
- Game result recording (wins, ties)
- Win streak tracking
- Prometheus metrics updates
- CORS middleware
- Health endpoints
- All winning patterns (row1-3, col1-3, diag1-2)

```bash
cd backend && go test -v ./...
```

### Integration Tests (`tests/integration/`)
Tests for API behavior under load:
- Concurrent game submissions (50 goroutines)
- API response time (<100ms threshold)
- High load handling (1000 requests)
- Invalid request handling

```bash
cd tests/integration && go test -v ./...
```

### Dockerfile Tests (`tests/policies/test-dockerfiles.sh`)
Validates Dockerfile best practices:
- Multi-stage builds
- Non-root USER instruction
- Minimal base images (distroless/chainguard)
- No root user execution

```bash
./tests/policies/test-dockerfiles.sh
```

### K8s Manifest Tests (`tests/policies/test-policies.sh`)
Validates Kubernetes manifests:
- Security contexts (runAsNonRoot, seccompProfile)
- Resource limits and requests
- Health probes (liveness, readiness)
- NetworkPolicy presence
- PodDisruptionBudget configuration

```bash
./tests/policies/test-policies.sh
```

### Security Tests (`tests/security/security_test.sh`)
Security validation:
- Container image vulnerability scanning (Trivy)
- Image signature verification (Cosign)
- SBOM verification
- NetworkPolicy ingress/egress rules
- Secret management (no hardcoded secrets)
- Pod Security Standards enforcement
- Resource limits validation

```bash
./tests/security/security_test.sh
```

### Reliability Tests (`tests/reliability/reliability_test.sh`)
High availability validation:
- PodDisruptionBudget configuration
- Replica counts per environment
- Health probe configuration
- Resource management
- Graceful shutdown settings
- Rolling update strategy

```bash
./tests/reliability/reliability_test.sh
```

### Observability Tests (`tests/observability/observability_test.sh`)
Monitoring validation:
- Prometheus metrics endpoint
- Health endpoints
- Structured logging format
- Prometheus scrape annotations
- Grafana dashboard presence

```bash
./tests/observability/observability_test.sh
```

### Performance Tests (`tests/performance/load_test.js`)
K6 load testing:
- Ramp up to 50 concurrent users
- Response time thresholds (p95 < 1s)
- Error rate thresholds (< 30%)
- Game submission flow testing

```bash
k6 run tests/performance/load_test.js
```

### E2E Tests (`tests/e2e/e2e_test.sh`)
End-to-end deployment testing:
- Full deployment flow
- Service connectivity
- Frontend-backend communication
- Game submission flow
- Ingress routing
- Pod restart resilience
- Network policy enforcement
- Metrics scraping
- Log output validation

```bash
NAMESPACE=tictactoe-dev ./tests/e2e/e2e_test.sh
```

## CI/CD Integration

All tests run automatically via GitHub Actions on:
- Push to main/staging/prod branches
- Pull requests to main/staging/prod branches

See `.github/workflows/comprehensive-tests.yaml` for the full pipeline configuration.

## Test Results

View test results in the GitHub Actions tab:
https://github.com/pnz1990/tictactoe-k8s/actions
