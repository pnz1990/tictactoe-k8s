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
│  (ghcr.io)                │             │  │ tictactoe-dev (1 replica)   │  │
│  ┌─────────────────────┐  │             │  │ tictactoe-staging (2)       │  │
│  │ Signed Images       │  │────────────▶│  │ tictactoe-prod (3)          │  │
│  │ SBOM Attached       │  │             │  └─────────────────────────────┘  │
│  │ Vulnerability Scanned│  │             │                │                  │
│  └─────────────────────┘  │             │                ▼                  │
└───────────────────────────┘             │  ┌─────────────────────────────┐  │
                                          │  │ Grafana Alloy → AMP         │  │
                                          │  │ Fluent Bit → CloudWatch     │  │
                                          │  └─────────────────────────────┘  │
                                          └───────────────────────────────────┘
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
| **Security Context** | Non-root, read-only filesystem, dropped capabilities, seccomp |
| **Network Policy** | Ingress on 8080/9113, egress to DNS only |
| **Pod Disruption Budget** | Ensures availability during cluster maintenance |
| **Kustomize Overlays** | Environment-specific configs (dev/staging/prod) |

## Directory Structure

```
tictactoe-k8s/
├── app/
│   ├── index.html          # Tic Tac Toe game
│   └── Dockerfile          # Multi-stage build with Chainguard nginx
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
├── renovate.json           # Automated dependency updates
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
cosign download attestation ghcr.io/pnz1990/tictactoe-k8s:latest | jq -r '.payload' | base64 -d | jq '.predicate'
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

## Best Practices Checklist

### CI/CD & Image
- [x] Multi-stage Dockerfile with non-root user
- [x] Image vulnerability scanning (Trivy → GitHub Security tab)
- [x] Semantic versioning with git tags
- [x] Image signing (Cosign keyless via GitHub OIDC)
- [x] SBOM generation (attached to image)
- [x] Build caching

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
- [x] Metrics collection (Grafana Alloy → AMP)
- [x] Log collection (Fluent Bit → CloudWatch)
- [x] Grafana dashboards (per environment + logs)
- [x] Structured JSON logging
- [x] Health endpoints

### GitOps
- [x] ArgoCD auto-sync with self-healing
- [x] Pruning enabled
- [x] Branch-based promotion (main → staging → prod)

## Related Repositories

- **[eks-infrastructure](https://github.com/pnz1990/eks-infrastructure)** - AWS managed services (AMP, Grafana Alloy, Fluent Bit) provisioned via GitOps

## License

MIT
