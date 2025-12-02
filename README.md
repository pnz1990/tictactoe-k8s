# Tic Tac Toe - Kubernetes Reference Application

[![Comprehensive Testing & Best Practices](https://github.com/pnz1990/tictactoe-k8s/actions/workflows/comprehensive-tests.yaml/badge.svg)](https://github.com/pnz1990/tictactoe-k8s/actions/workflows/comprehensive-tests.yaml)
[![Build and Push](https://github.com/pnz1990/tictactoe-k8s/actions/workflows/build.yaml/badge.svg)](https://github.com/pnz1990/tictactoe-k8s/actions/workflows/build.yaml)

A production-ready reference application demonstrating modern DevOps and Kubernetes best practices. This Tic Tac Toe game serves as a template for building secure, scalable, and maintainable containerized applications with business metrics.

## Managed AWS Services

This project uses the following AWS managed services:

| Service | ID/Endpoint | Purpose |
|---------|-------------|---------|
| **Amazon Managed Grafana (AMG)** | `g-8f648e108c` | Dashboard visualization |
| **Amazon Managed Prometheus (AMP)** | `ws-62f6ab4b-6a1c-4971-806e-dee13a1e1e95` | Metrics storage |
| **Amazon EKS** | `unique-lofi-goose` | Kubernetes cluster |
| **AWS-managed ArgoCD** | Managed service | GitOps continuous deployment |
| **Amazon DynamoDB** | `tictactoe-games-{env}` | Game state persistence |

**Note:** ArgoCD, Grafana, and Prometheus are AWS-managed services. You cannot access their logs, configurations, or internal resources directly. All management is done through:
- ArgoCD: Application CRs in the cluster
- Grafana: Web UI at https://g-8f648e108c.grafana-workspace.ap-northeast-2.amazonaws.com
- Prometheus: Queried through Grafana datasources

**ArgoCD Auto-Sync:** Changes are automatically picked up within ~3 minutes (default polling interval). No manual sync required.

## Environment URLs

| Environment | URL |
|-------------|-----|
| **Dev** | http://k8s-tictacto-tictacto-74ef9eee48-1677920840.ap-northeast-2.elb.amazonaws.com |
| **Staging** | http://k8s-tictacto-tictacto-56b83df6f7-1529768459.ap-northeast-2.elb.amazonaws.com |
| **Production** | http://k8s-tictacto-tictacto-07fc5efb78-1765375059.ap-northeast-2.elb.amazonaws.com |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GitHub Repository                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  app/                    │  backend/              │  k8s/kro/               │
│  ├── index.html          │  ├── main.go           │  ├── tictactoe-rgd.yaml │
│  └── Dockerfile          │  ├── go.mod            │  ├── dev/dev.yaml       │
│                          │  └── Dockerfile        │  ├── staging/staging.yaml│
│                          │                        │  └── prod/prod.yaml     │
└─────────────────────────────────────────────────────────────────────────────┘
                │                                    │
                ▼                                    ▼
┌───────────────────────────┐         ┌───────────────────────────────────────┐
│   GitHub Actions CI/CD    │         │            ArgoCD (GitOps)            │
│  ┌─────────────────────┐  │         │  ┌─────────────────────────────────┐  │
│  │ Build Frontend      │  │         │  │ Watches k8s/kro/ for changes    │  │
│  │ Build Backend       │  │         │  │ Deploys KRO instances           │  │
│  │ Trivy Scan          │  │         │  │ Self-healing enabled            │  │
│  │ Cosign Sign         │  │         │  └─────────────────────────────────┘  │
│  └─────────────────────┘  │         └───────────────────────────────────────┘
└───────────────────────────┘                          │
                                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Kubernetes Cluster                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                    KRO (Kube Resource Orchestrator)                     ││
│  │         TicTacToeApp instances → Managed Resources                      ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│           │                                              │                   │
│           ▼                                              ▼                   │
│  ┌─────────────────────┐                    ┌─────────────────────┐         │
│  │   Backend (Go)      │                    │   Frontend (nginx)  │         │
│  │   Port 8081         │◄───── API ─────────│   Port 8080         │         │
│  │   /metrics          │                    │                     │         │
│  └─────────────────────┘                    └─────────────────────┘         │
│           │                                                                  │
│           ▼                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Grafana Alloy → Amazon Managed Prometheus (AMP)                        ││
│  │  Grafana Operator → Amazon Managed Grafana (AMG)                        ││
│  └─────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────┘
```

## Features

### Application
- **Tic Tac Toe Game**: Browser-based two-player game with neon cyberpunk styling
- **Player Names**: Enter player names before starting the game
- **Game Recording**: All game results sent to backend API
- **Lightweight**: ~3KB total size

### CI/CD Pipeline (GitHub Actions)

| Feature | Description |
|---------|-------------|
| **Multi-arch Build** | Builds for linux/amd64 using Docker Buildx |
| **Vulnerability Scanning** | Trivy scans for CVEs, results in GitHub Security tab |
| **Image Signing** | Cosign keyless signing via GitHub OIDC |
| **SBOM Generation** | Software Bill of Materials attached to image |
| **Semantic Versioning** | Tag `v1.0.0` creates `1.0.0`, `1.0`, `latest` image tags |
| **Build Cache** | GitHub Actions cache for faster builds |

### Container Image

| Feature | Description |
|---------|-------------|
| **Base Image** | Chainguard nginx (distroless, zero CVEs) |
| **Non-root User** | Runs as UID 65532 |
| **Minimal Attack Surface** | No shell, no package manager |
| **Signed & Verified** | Cosign signature for supply chain security |

### Kubernetes Manifests (KRO)

| Feature | Description |
|---------|-------------|
| **KRO ResourceGraphDefinition** | Custom TicTacToeApp API for standardized deployments |
| **DynamoDB Table (ACK)** | Game persistence table created via AWS Controllers for Kubernetes |
| **IAM Role & Policy (ACK)** | Per-environment IAM resources for DynamoDB access |
| **EKS Pod Identity** | Secure pod-to-AWS authentication without static credentials |
| **Synthetic Monitoring** | Continuous health checks with Prometheus metrics |
| **ArgoCD PostSync Hook** | Smoke tests run after each deployment, blocks bad deploys |
| **Resource Limits** | CPU/memory requests and limits defined |
| **Health Probes** | Liveness and readiness probes configured |
| **Security Context** | Non-root, read-only filesystem, dropped capabilities |
| **Pod Disruption Budget** | Ensures availability during cluster maintenance |
| **ALB Ingress** | EKS Auto Mode Application Load Balancer per environment |
| **Grafana Dashboards** | Ops and Business dashboards auto-created per environment |

## Directory Structure

```
tictactoe-k8s/
├── app/
│   ├── index.html          # Tic Tac Toe game
│   └── Dockerfile          # Multi-stage build with Chainguard nginx
├── backend/
│   ├── main.go             # Go backend with Prometheus metrics
│   └── Dockerfile          # Multi-stage build
├── synthetic-monitor/
│   ├── main.go             # Synthetic monitoring tests
│   └── Dockerfile          # Distroless container
├── k8s/kro/
│   ├── tictactoe-rgd.yaml  # KRO ResourceGraphDefinition
│   ├── dev/dev.yaml        # Dev instance (1 replica)
│   ├── staging/staging.yaml # Staging instance (2 replicas)
│   └── prod/prod.yaml      # Prod instance (3 replicas)
├── .github/workflows/
│   ├── build.yaml          # CI/CD pipeline
│   └── comprehensive-tests.yaml
├── tests/                  # Test suites
└── README.md
```

## Environment Configurations

| Environment | Replicas | CPU Request | CPU Limit | Memory Request | Memory Limit | PDB Min Available |
|-------------|----------|-------------|-----------|----------------|--------------|-------------------|
| dev         | 1        | 10m         | 100m      | 16Mi           | 64Mi         | 1                 |
| staging     | 2        | 50m         | 200m      | 16Mi           | 64Mi         | 1                 |
| prod        | 3        | 100m        | 500m      | 32Mi           | 128Mi        | 2                 |

## Branch-Based Promotion Workflow

```
┌─────────────┐     merge      ┌─────────────┐     merge      ┌─────────────┐
│    main     │ ─────────────▶ │   staging   │ ─────────────▶ │    prod     │
│   (dev)     │                │             │                │             │
└─────────────┘                └─────────────┘                └─────────────┘
       │                              │                              │
       ▼                              ▼                              ▼
┌─────────────┐                ┌─────────────┐                ┌─────────────┐
│ tictactoe   │                │ tictactoe-  │                │ tictactoe-  │
│ (ArgoCD)    │                │ staging     │                │ prod        │
└─────────────┘                └─────────────┘                └─────────────┘
       │                              │                              │
       ▼                              ▼                              ▼
┌─────────────┐                ┌─────────────┐                ┌─────────────┐
│ tictactoe-  │                │ tictactoe-  │                │ tictactoe-  │
│ dev (ns)    │                │ staging(ns) │                │ prod (ns)   │
│ 1 replica   │                │ 2 replicas  │                │ 3 replicas  │
└─────────────┘                └─────────────┘                └─────────────┘
```

### Promotion Commands

```bash
# Promote dev → staging
git checkout staging && git merge main && git push

# Promote staging → prod
git checkout prod && git merge staging && git push
```

### ArgoCD Applications

| App Name | Branch | Overlay | Namespace | Replicas |
|----------|--------|---------|-----------|----------|
| tictactoe | main | dev | tictactoe-dev | 1 |
| tictactoe-staging | staging | staging | tictactoe-staging | 2 |
| tictactoe-prod | prod | prod | tictactoe-prod | 3 |

## Quick Start

### Prerequisites
- Kubernetes cluster (EKS recommended)
- ArgoCD installed
- `kubectl` configured

### Deploy All Environments

```bash
CLUSTER_SERVER="https://kubernetes.default.svc"  # or your cluster ARN

# Dev (tracks main branch)
kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tictactoe
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/pnz1990/tictactoe-k8s.git
    targetRevision: main
    path: k8s/overlays/dev
  destination:
    server: ${CLUSTER_SERVER}
    namespace: tictactoe-dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF

# Staging (tracks staging branch)
kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tictactoe-staging
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/pnz1990/tictactoe-k8s.git
    targetRevision: staging
    path: k8s/overlays/staging
  destination:
    server: ${CLUSTER_SERVER}
    namespace: tictactoe-staging
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF

# Prod (tracks prod branch)
kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tictactoe-prod
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/pnz1990/tictactoe-k8s.git
    targetRevision: prod
    path: k8s/overlays/prod
  destination:
    server: ${CLUSTER_SERVER}
    namespace: tictactoe-prod
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF
```

### Access the Application

Each environment has its own ALB ingress (EKS Auto Mode):

```bash
# Get ALB URLs
kubectl get ingress -A -o custom-columns='ENV:.metadata.namespace,URL:.status.loadBalancer.ingress[0].hostname'
```

Or use port-forward:
```bash
kubectl port-forward -n tictactoe-dev svc/tictactoe 8080:80
```

## CI/CD Workflow

### Automatic Triggers
- **Push to `main`** with changes in `app/` → Builds and pushes `latest` + SHA tag
- **Push tag `v*`** → Builds and pushes semantic version tags

### Required Secrets

| Secret | Purpose |
|--------|---------|
| `PNZ_PAT` | Personal Access Token for promotion workflows to bypass branch protection on staging/prod |

**Important:** The `PNZ_PAT` secret is required for the promotion workflows (`promote-staging.yaml`, `promote-prod.yaml`) to push image tag updates to protected branches. Without this PAT, the sync-image-tag jobs will fail with "protected branch hook declined" errors.

### Manual Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

This creates image tags: `v1.0.0`, `1.0.0`, `1.0`, `latest`

## Security Features

### Supply Chain Security

**Image Signing (Cosign)**
```bash
# Verify image signature
cosign verify ghcr.io/pnz1990/tictactoe-k8s:latest \
  --certificate-identity-regexp=".*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

**SBOM (Software Bill of Materials)**
```bash
# View SBOM attestation
cosign download attestation ghcr.io/pnz1990/tictactoe-k8s:latest | head -1 | jq -r '.payload' | base64 -d | jq -r '.predicate' | jq
```

### Automated Dependency Updates (Renovate)

Renovate automatically creates PRs for dependency updates:

| Dependency | Auto-merge | Notes |
|------------|------------|-------|
| Chainguard nginx | ❌ | Manual review required |
| busybox | ✅ | Auto-merged |
| GitHub Actions | ✅ | Auto-merged |
| Prometheus exporter | ❌ | Manual review required |

### Container Security
- ✅ Distroless base image (no shell, no package manager)
- ✅ Non-root user (UID 65532)
- ✅ Read-only root filesystem
- ✅ All capabilities dropped
- ✅ Seccomp profile (RuntimeDefault)
- ✅ No privilege escalation

### Kubernetes Security
- ✅ Pod Security Context enforced
- ✅ Network Policy restricts traffic
- ✅ Resource limits prevent DoS
- ✅ Health probes ensure availability

## Observability

### Prometheus Metrics

Metrics exposed via nginx-prometheus-exporter sidecar on port 9113:

```bash
kubectl port-forward -n tictactoe-dev svc/tictactoe 9113:9113
curl localhost:9113/metrics
```

**Available Metrics:**
- `nginx_connections_active` - Active client connections
- `nginx_connections_accepted` - Total accepted connections
- `nginx_connections_handled` - Total handled connections
- `nginx_http_requests_total` - Total HTTP requests
- `nginx_up` - Nginx health status

**Pod Annotations** (auto-discovery):
```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "9113"
prometheus.io/path: "/metrics"
```

### Metrics Collection (Grafana Alloy → AMP)

Metrics are collected by Grafana Alloy and sent to Amazon Managed Prometheus:
- **AMP Workspace**: `ws-62f6ab4b-6a1c-4971-806e-dee13a1e1e95`
- **Region**: ap-northeast-2
- **Labels**: namespace, pod, app automatically added

### Log Collection (Fluent Bit → CloudWatch)

Logs are collected by Fluent Bit DaemonSet and sent to CloudWatch Logs:
- **Log Group**: `/eks/tictactoe`
- **Log Streams**: Per namespace (tictactoe-dev, tictactoe-staging, tictactoe-prod)

### Grafana Dashboards (Amazon Managed Grafana)

Pre-built dashboards available at AMG workspace `g-8f648e108c`:

| Dashboard | Path | Description |
|-----------|------|-------------|
| Tic Tac Toe - DEV | `/d/ff58o5r40bl6oa` | Dev environment metrics |
| Tic Tac Toe - STAGING | `/d/bf58o6pd8jf9cd` | Staging environment metrics |
| Tic Tac Toe - PROD | `/d/df58o7xz29hq8f` | Production environment metrics |
| Tic Tac Toe - Logs | `/d/af58pcjfql7nkc` | CloudWatch logs viewer |

### Structured Logging

Logs output in JSON format:
```json
{
  "time": "2025-11-26T00:30:51+00:00",
  "remote_addr": "172.31.13.88",
  "method": "GET",
  "uri": "/index.html",
  "status": 200,
  "body_bytes_sent": 3241,
  "request_time": 0.000
}
```

### Health Endpoints
- **Liveness**: `GET /` on port 8080
- **Readiness**: `GET /` on port 8080
- **Health check**: `GET /healthz` on port 8080

## Development

### Local Testing

```bash
cd app
docker build -t tictactoe:local .
docker run -p 8080:8080 tictactoe:local
# Open http://localhost:8080
```

### Preview Kustomize Output

```bash
kubectl kustomize k8s/overlays/dev
kubectl kustomize k8s/overlays/prod
```

## Testing

The project includes a comprehensive test suite that runs automatically on every push via GitHub Actions.

### Test Suite Overview

| Test | Description | Location |
|------|-------------|----------|
| Unit Tests | Go backend business logic, metrics, CORS, health endpoints | `backend/main_test.go` |
| Integration Tests | Concurrent requests, API response time, load handling | `tests/integration/` |
| Dockerfile Tests | Multi-stage builds, non-root user, minimal base images | `tests/policies/test-dockerfiles.sh` |
| K8s Manifest Validation | Security contexts, resource limits, probes, NetworkPolicy | `tests/policies/test-policies.sh` |
| Security Tests | Pod security standards, secrets, network policies, Trivy scans | `tests/security/security_test.sh` |
| Reliability Tests | PDB, replicas, health probes, rolling updates | `tests/reliability/reliability_test.sh` |
| Observability Tests | Prometheus annotations, metrics endpoints | `tests/observability/observability_test.sh` |
| Performance Tests | K6 load testing with thresholds | `tests/performance/load_test.js` |
| E2E Tests | Full deployment flow, connectivity, resilience | `tests/e2e/e2e_test.sh` |

### Running Tests Locally

```bash
# Unit tests
cd backend && go test -v ./...

# Integration tests
cd tests/integration && go test -v ./...

# Policy tests
./tests/policies/test-dockerfiles.sh
./tests/policies/test-policies.sh

# Security tests
./tests/security/security_test.sh

# Reliability tests
./tests/reliability/reliability_test.sh
```

### CI/CD Test Pipeline

The GitHub Actions workflow runs on every push to main/staging/prod branches:

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Unit Tests    │  │Integration Tests│  │ Dockerfile Tests│
└────────┬────────┘  └────────┬────────┘  └────────┬────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ Security Tests  │  │Reliability Tests│  │  Code Quality   │
└────────┬────────┘  └────────┬────────┘  └────────┬────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                              ▼
                    ┌─────────────────┐
                    │  Test Summary   │
                    └─────────────────┘
```

## Best Practices Checklist

### CI/CD & Image
- [x] Multi-stage Dockerfile with non-root user
- [x] Image vulnerability scanning (Trivy → GitHub Security tab)
- [x] Semantic versioning with git tags
- [x] Image signing (Cosign keyless via GitHub OIDC)
- [x] SBOM generation (attached to image)
- [x] Build caching

### Testing
- [x] Unit tests with coverage reporting
- [x] Integration tests (concurrency, response time)
- [x] Dockerfile policy validation
- [x] Kubernetes manifest validation
- [x] Security policy tests
- [x] Reliability tests (PDB, probes, resources)
- [x] Performance tests (K6 load testing)
- [x] Automated CI/CD test pipeline

### Kubernetes Manifests
- [x] Resource limits/requests
- [x] Liveness/readiness probes
- [x] Security context (non-root, read-only filesystem, seccomp)
- [x] Network policies
- [x] Pod disruption budget
- [x] Kustomize overlays (dev/staging/prod)

### Security
- [x] Renovate for automated dependency updates
- [x] Distroless base image (Chainguard)
- [x] All capabilities dropped
- [x] No privilege escalation

### Observability
- [x] Prometheus metrics (nginx-prometheus-exporter sidecar)
- [x] Business metrics (Go backend with Prometheus client)
- [x] Metrics collection (Grafana Alloy → AMP)
- [x] Log collection (Fluent Bit → CloudWatch)
- [x] Infrastructure dashboards (per environment)
- [x] Business dashboards (leaderboard, patterns, streaks)
- [x] Grafana dashboards as code (GrafanaDashboard CRDs)
- [x] Structured JSON logging
- [x] Health endpoints

### Business Metrics
The backend exposes the following Prometheus metrics:

| Metric | Labels | Description |
|--------|--------|-------------|
| `tictactoe_games_total` | result, mode | Total games (win/tie) by mode |
| `tictactoe_wins_total` | player, pattern, mode | Wins by player, pattern, and mode |
| `tictactoe_player_games_total` | player, mode | Games per player by mode |
| `tictactoe_ties_total` | mode | Total tied games by mode |
| `tictactoe_current_win_streak` | player | Current win streak |
| `tictactoe_dynamodb_operations_total` | operation, status | DynamoDB operations (PutItem success/error) |
| `tictactoe_online_games_active` | - | Currently active online games |
| `tictactoe_online_games_created_total` | - | Total online games created |
| `tictactoe_websocket_connections_active` | - | Active WebSocket connections |
| `tictactoe_websocket_messages_total` | type, direction | WebSocket messages (in/out) |

**Game Modes**: `local` (same device), `online` (multiplayer via WebSocket)

**Winning Patterns**: row1, row2, row3, col1, col2, col3, diag1, diag2

### Online Multiplayer (v3.0)

The application supports real-time online multiplayer via WebSocket:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/game/create` | POST | Create new online game, returns game ID |
| `/api/game/join` | POST | Join existing game by ID |
| `/api/game/get` | GET | Get game state by ID |
| `/api/game/ws` | WS | WebSocket for real-time game updates |

**Features:**
- Create game and share link/code with opponent
- Real-time board sync via WebSocket
- Turn-based play enforcement
- Game state persisted to DynamoDB on completion

### Leaderboard API (v3.1)

REST API for player statistics and game history, backed by DynamoDB:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/leaderboard` | GET | Top 20 players by wins with W/L/T stats |
| `/api/stats` | GET | Global stats: total games, wins, ties, patterns |
| `/api/recent` | GET | Last 20 games played |
| `/api/player?player=NAME` | GET | Individual player statistics |

**DynamoDB Schema:**
- Table: `tictactoe-games-{env}`
- Primary Key: `gameId` (HASH), `timestamp` (RANGE)
- GSI: `winner-timestamp-index` for leaderboard queries

### GitOps
- [x] ArgoCD auto-sync with self-healing
- [x] Pruning enabled
- [x] Branch-based promotion (main → staging → prod)
- [x] Grafana Operator managing AMG dashboards
- [x] PostSync hooks for deployment validation

### Synthetic Monitoring & Smoke Tests

| Component | Type | Purpose |
|-----------|------|---------|
| **Synthetic Monitor** | Deployment | Continuous health checks every 5 minutes |
| **PostSync Smoke Test** | ArgoCD Hook | Validates deployment after each sync |

**Synthetic Monitor Metrics:**
- `synthetic_test_success{test, environment}` - Test result (1=pass, 0=fail)
- `synthetic_test_duration_seconds{test, environment}` - Test duration

**PostSync Smoke Test:**
- Runs automatically after ArgoCD sync
- Tests frontend health, backend health, API endpoints, and online game creation
- Failed tests mark sync as "Degraded"
- Job auto-deletes on success

**Business Dashboard Filtering:**
- Synthetic test results (player names starting with "Synthetic") are filtered from business metrics
- Ops dashboard shows all data including synthetic tests

## Related Repositories

- **[eks-infrastructure](https://github.com/pnz1990/eks-infrastructure)** - AWS managed services (AMP, Grafana Alloy, Fluent Bit) provisioned via GitOps

## License

MIT

### ALB Metrics (CloudWatch)

ALB metrics are automatically collected by AWS and available in Grafana dashboards:

| Metric | Description |
|--------|-------------|
| RequestCount | Total requests through ALB |
| TargetResponseTime | Backend response latency (p99) |
| HTTPCode_ELB_5XX_Count | ALB 5xx errors |
| HealthyHostCount | Number of healthy targets |

These metrics are queried directly from CloudWatch in the Grafana dashboards.
