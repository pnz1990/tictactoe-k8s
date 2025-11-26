# Tic Tac Toe - Kubernetes Reference Application

A production-ready reference application demonstrating modern DevOps and Kubernetes best practices. This simple Tic Tac Toe game serves as a template for building secure, scalable, and maintainable containerized applications.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GitHub Repository                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  app/                          │  k8s/                                      │
│  ├── index.html (game)         │  ├── base/ (shared manifests)             │
│  └── Dockerfile                │  └── overlays/                            │
│                                │      ├── dev/                              │
│                                │      ├── staging/                          │
│                                │      └── prod/                             │
└─────────────────────────────────────────────────────────────────────────────┘
                │                                    │
                ▼                                    ▼
┌───────────────────────────┐         ┌───────────────────────────────────────┐
│   GitHub Actions CI/CD    │         │            ArgoCD (GitOps)            │
│  ┌─────────────────────┐  │         │  ┌─────────────────────────────────┐  │
│  │ Build & Push Image  │  │         │  │ Watches k8s/ for changes        │  │
│  │ Trivy Scan          │  │         │  │ Auto-syncs to Kubernetes        │  │
│  │ Cosign Sign         │  │         │  │ Self-healing enabled            │  │
│  │ SBOM Generation     │  │         │  └─────────────────────────────────┘  │
│  └─────────────────────┘  │         └───────────────────────────────────────┘
└───────────────────────────┘                          │
                │                                      ▼
                ▼                         ┌───────────────────────────────────┐
┌───────────────────────────┐             │        Kubernetes Cluster         │
│  GitHub Container Registry │             │  ┌─────────────────────────────┐  │
│  (ghcr.io)                │             │  │ Namespace: tictactoe-dev    │  │
│  ┌─────────────────────┐  │             │  │  ├── Deployment (1 replica) │  │
│  │ Signed Images       │  │────────────▶│  │  ├── Service               │  │
│  │ SBOM Attached       │  │             │  │  ├── NetworkPolicy         │  │
│  │ Vulnerability Scanned│  │             │  │  └── PodDisruptionBudget   │  │
│  └─────────────────────┘  │             │  └─────────────────────────────┘  │
└───────────────────────────┘             └───────────────────────────────────┘
```

## Features

### Application
- **Tic Tac Toe Game**: Browser-based two-player game with neon cyberpunk styling
- **Static Site**: Pure HTML/CSS/JavaScript, no backend required
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

### Kubernetes Manifests

| Feature | Description |
|---------|-------------|
| **Resource Limits** | CPU/memory requests and limits defined |
| **Health Probes** | Liveness and readiness probes configured |
| **Security Context** | Non-root, read-only filesystem, dropped capabilities |
| **Network Policy** | Ingress restricted to port 8080, egress to DNS only |
| **Pod Disruption Budget** | Ensures availability during cluster maintenance |
| **Kustomize Overlays** | Environment-specific configs (dev/staging/prod) |

## Directory Structure

```
tictactoe-k8s/
├── app/
│   ├── index.html          # Tic Tac Toe game
│   └── Dockerfile          # Container build instructions
├── k8s/
│   ├── base/               # Shared Kubernetes manifests
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── networkpolicy.yaml
│   │   ├── pdb.yaml
│   │   └── kustomization.yaml
│   └── overlays/           # Environment-specific overrides
│       ├── dev/            # 1 replica, minimal resources
│       ├── staging/        # 2 replicas, moderate resources
│       └── prod/           # 3 replicas, full resources
├── .github/
│   └── workflows/
│       └── build.yaml      # CI/CD pipeline
└── README.md
```

## Environment Configurations

| Environment | Replicas | CPU Request | CPU Limit | Memory Request | Memory Limit | PDB Min Available |
|-------------|----------|-------------|-----------|----------------|--------------|-------------------|
| dev         | 1        | 10m         | 100m      | 16Mi           | 64Mi         | 1                 |
| staging     | 2        | 50m         | 200m      | 16Mi           | 64Mi         | 1                 |
| prod        | 3        | 100m        | 500m      | 32Mi           | 128Mi        | 2                 |

## Branch-Based Promotion Workflow

This project uses a branch-based GitOps promotion strategy:

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
git checkout staging
git merge main
git push

# Promote staging → prod
git checkout prod
git merge staging
git push
```

### ArgoCD Applications

| App Name | Branch | Overlay | Namespace | Replicas |
|----------|--------|---------|-----------|----------|
| tictactoe | main | dev | tictactoe-dev | 1 |
| tictactoe-staging | staging | staging | tictactoe-staging | 2 |
| tictactoe-prod | prod | prod | tictactoe-prod | 3 |

## Quick Start

### Prerequisites
- Kubernetes cluster
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

```bash
kubectl port-forward -n tictactoe-dev svc/tictactoe 8080:80
# Open http://localhost:8080
```

## CI/CD Workflow

### Automatic Triggers
- **Push to `main`** with changes in `app/` → Builds and pushes `latest` + SHA tag
- **Push tag `v*`** → Builds and pushes semantic version tags

### Manual Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

This creates image tags: `v1.0.0`, `1.0.0`, `1.0`, `latest`

## Security Features

### Supply Chain Security
```bash
# Verify image signature
cosign verify ghcr.io/pnz1990/tictactoe-k8s:latest \
  --certificate-identity-regexp=".*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

### Container Security
- ✅ Distroless base image (no shell, no package manager)
- ✅ Non-root user (UID 65532)
- ✅ Read-only root filesystem
- ✅ All capabilities dropped
- ✅ No privilege escalation

### Kubernetes Security
- ✅ Pod Security Context enforced
- ✅ Network Policy restricts traffic
- ✅ Resource limits prevent DoS
- ✅ Health probes ensure availability

## Monitoring & Observability

### Prometheus Metrics

The application exposes metrics via nginx-prometheus-exporter sidecar on port 9113.

```bash
# Port-forward to access metrics
kubectl port-forward -n tictactoe-dev svc/tictactoe 9113:9113

# View metrics
curl localhost:9113/metrics
```

**Available Metrics:**
- `nginx_connections_active` - Active client connections
- `nginx_connections_accepted` - Total accepted connections
- `nginx_connections_handled` - Total handled connections
- `nginx_http_requests_total` - Total HTTP requests
- `nginx_up` - Nginx health status

**Prometheus Annotations** (auto-discovery):
```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "9113"
prometheus.io/path: "/metrics"
```

### Structured Logging

Logs are output in JSON format for easy parsing by log aggregators (Fluentd, Loki, CloudWatch, etc.):

```json
{
  "time": "2025-11-26T00:30:51+00:00",
  "remote_addr": "172.31.13.88",
  "method": "GET",
  "uri": "/index.html",
  "status": 200,
  "body_bytes_sent": 3241,
  "request_time": 0.000,
  "http_referer": "",
  "http_user_agent": "Mozilla/5.0..."
}
```

```bash
# View logs
kubectl logs -n tictactoe-dev deploy/tictactoe -c tictactoe
```

### Health Endpoints
- **Liveness**: `GET /` on port 8080
- **Readiness**: `GET /` on port 8080
- **Health check**: `GET /healthz` on port 8080

## Development

### Local Testing

```bash
# Build locally
cd app
docker build -t tictactoe:local .

# Run locally
docker run -p 8080:8080 tictactoe:local

# Open http://localhost:8080
```

### Preview Kustomize Output

```bash
# View rendered manifests
kubectl kustomize k8s/overlays/dev
kubectl kustomize k8s/overlays/prod
```

## Best Practices Implemented

### CI/CD & Image
- [x] Multi-stage Dockerfile with non-root user
- [x] Image vulnerability scanning (Trivy)
- [x] Semantic versioning with git tags
- [x] Sign images with cosign
- [x] SBOM generation

### Kubernetes Manifests
- [x] Resource limits/requests
- [x] Liveness/readiness probes
- [x] Security context (non-root, read-only filesystem)
- [x] Network policies
- [x] Pod disruption budget
- [x] Kustomize overlays (dev/staging/prod)

### Observability
- [x] Prometheus metrics endpoint (nginx-prometheus-exporter sidecar)
- [x] Structured JSON logging
- [x] Health endpoints (/healthz)

### GitOps
- [x] ArgoCD auto-sync
- [x] Self-healing enabled
- [x] Pruning enabled
- [x] Branch-based promotion (main → staging → prod)

## License

MIT
