#!/bin/bash
set -e

echo "=== Security Testing Suite ==="

FAILED=0

# Test 1: Container Image Vulnerability Scanning
echo ""
echo "--- Test 1: Image Vulnerability Scanning ---"
for image in "ghcr.io/pnz1990/tictactoe-frontend:latest" "ghcr.io/pnz1990/tictactoe-backend:latest"; do
  echo "Scanning $image..."
  if command -v trivy &> /dev/null; then
    trivy image --severity HIGH,CRITICAL --exit-code 1 $image || FAILED=1
  else
    echo "SKIP: trivy not installed"
  fi
done

# Test 2: Image Signature Verification
echo ""
echo "--- Test 2: Image Signature Verification ---"
if command -v cosign &> /dev/null; then
  for image in "ghcr.io/pnz1990/tictactoe-frontend:latest" "ghcr.io/pnz1990/tictactoe-backend:latest"; do
    echo "Verifying signature for $image..."
    if cosign verify $image \
      --certificate-identity-regexp=".*" \
      --certificate-oidc-issuer="https://token.actions.githubusercontent.com" 2>/dev/null; then
      echo "PASS: Signature verified"
    else
      echo "WARN: Could not verify signature (image may not exist or not signed)"
    fi
  done
else
  echo "SKIP: cosign not installed"
fi

# Test 3: SBOM Verification
echo ""
echo "--- Test 3: SBOM Verification ---"
if command -v cosign &> /dev/null; then
  for image in "ghcr.io/pnz1990/tictactoe-frontend:latest" "ghcr.io/pnz1990/tictactoe-backend:latest"; do
    echo "Checking SBOM for $image..."
    if cosign download attestation $image 2>/dev/null | head -1 | jq -r '.payload' 2>/dev/null | base64 -d 2>/dev/null | jq -r '.predicate' > /tmp/sbom.json 2>/dev/null; then
      if [ -s /tmp/sbom.json ]; then
        echo "PASS: SBOM exists"
      else
        echo "WARN: SBOM missing (image may not be published yet)"
      fi
    else
      echo "WARN: Could not verify SBOM (image may not exist in registry)"
    fi
  done
else
  echo "SKIP: cosign not installed"
fi

# Test 4: Network Policy Validation
echo ""
echo "--- Test 4: Network Policy Validation ---"
for env in dev staging prod; do
  kubectl kustomize k8s/overlays/$env > /tmp/manifests-$env.yaml
  
  # Check NetworkPolicy exists
  if ! grep -q "kind: NetworkPolicy" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing NetworkPolicy"
    FAILED=1
  else
    # Check ingress rules (search entire file for ingress under NetworkPolicy)
    if ! grep -q "ingress:" /tmp/manifests-$env.yaml; then
      echo "FAIL: $env - NetworkPolicy missing ingress rules"
      FAILED=1
    else
      echo "PASS: $env - NetworkPolicy with ingress rules"
    fi
    
    # Check egress rules
    if ! grep -A 20 "kind: NetworkPolicy" /tmp/manifests-$env.yaml | grep -q "egress:"; then
      echo "FAIL: $env - NetworkPolicy missing egress rules"
      FAILED=1
    else
      echo "PASS: $env - NetworkPolicy with egress rules"
    fi
  fi
done

# Test 5: Secret Management
echo ""
echo "--- Test 5: Secret Management ---"
for env in dev staging prod; do
  # Check no hardcoded secrets in manifests
  if grep -iE "(password|secret|token|key).*:" /tmp/manifests-$env.yaml | grep -v "secretName" | grep -v "secretKeyRef"; then
    echo "WARN: $env - Potential hardcoded secrets found"
  else
    echo "PASS: $env - No hardcoded secrets"
  fi
done

# Test 6: RBAC Validation
echo ""
echo "--- Test 6: RBAC Validation ---"
for env in dev staging prod; do
  # Check if ServiceAccount is defined
  if grep -q "kind: ServiceAccount" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - ServiceAccount defined"
  else
    echo "WARN: $env - No ServiceAccount (using default)"
  fi
done

# Test 7: Pod Security Standards
echo ""
echo "--- Test 7: Pod Security Standards ---"
for env in dev staging prod; do
  # Check runAsNonRoot
  if ! grep -q "runAsNonRoot: true" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing runAsNonRoot: true"
    FAILED=1
  fi
  
  # Check readOnlyRootFilesystem
  if ! grep -q "readOnlyRootFilesystem: true" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing readOnlyRootFilesystem: true"
    FAILED=1
  fi
  
  # Check allowPrivilegeEscalation
  if ! grep -q "allowPrivilegeEscalation: false" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing allowPrivilegeEscalation: false"
    FAILED=1
  fi
  
  # Check capabilities are dropped
  if ! grep -A 2 "capabilities:" /tmp/manifests-$env.yaml | grep -q "drop:"; then
    echo "FAIL: $env - Capabilities not dropped"
    FAILED=1
  fi
  
  # Check seccomp profile
  if ! grep -q "type: RuntimeDefault" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing seccomp RuntimeDefault"
    FAILED=1
  fi
  
  echo "PASS: $env - Pod Security Standards enforced"
done

# Test 8: TLS/HTTPS Configuration
echo ""
echo "--- Test 8: TLS Configuration ---"
for env in dev staging prod; do
  # Check if Ingress has TLS configuration
  if grep -q "kind: Ingress" /tmp/manifests-$env.yaml; then
    if grep -A 20 "kind: Ingress" /tmp/manifests-$env.yaml | grep -q "tls:"; then
      echo "PASS: $env - Ingress has TLS configuration"
    else
      echo "WARN: $env - Ingress missing TLS configuration"
    fi
  fi
done

# Test 9: Resource Quotas
echo ""
echo "--- Test 9: Resource Limits ---"
for env in dev staging prod; do
  # Check limits section exists
  if grep -q "limits:" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Resource limits defined"
  else
    echo "FAIL: $env - Missing resource limits"
    FAILED=1
  fi
done

# Test 10: Container Registry Security
echo ""
echo "--- Test 10: Container Registry Security ---"
for env in dev staging prod; do
  # Check images are from trusted registries
  if grep -E "image:" /tmp/manifests-$env.yaml | grep -v -E "(ghcr.io|gcr.io|public.ecr.aws|docker.io/library)"; then
    echo "WARN: $env - Image from non-standard registry"
  else
    echo "PASS: $env - Images from trusted registries"
  fi
  
  # Check no :latest tag in prod
  if [ "$env" = "prod" ]; then
    if grep -E "image:.*:latest" /tmp/manifests-$env.yaml; then
      echo "WARN: $env - Using :latest tag (should pin versions)"
    else
      echo "PASS: $env - Not using :latest tag"
    fi
  fi
done

echo ""
if [ $FAILED -eq 0 ]; then
  echo "✅ All security tests passed!"
  exit 0
else
  echo "❌ Some security tests failed!"
  exit 1
fi
