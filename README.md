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

## Quick Start

### Prerequisites
- Kubernetes cluster
- ArgoCD installed
- `kubectl` configured

### Deploy with ArgoCD

```bash
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
    targetRevision: HEAD
    path: k8s/overlays/dev  # or staging, prod
  destination:
    server: https://kubernetes.default.svc
    namespace: tictactoe-dev
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

### Health Endpoints
- **Liveness**: `GET /` on port 8080 (checks if nginx is responding)
- **Readiness**: `GET /` on port 8080 (checks if app is ready to serve)

### ArgoCD Dashboard
Monitor sync status, health, and history in the ArgoCD UI.

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

### GitOps
- [x] ArgoCD auto-sync
- [x] Self-healing enabled
- [x] Pruning enabled

## License

MIT
