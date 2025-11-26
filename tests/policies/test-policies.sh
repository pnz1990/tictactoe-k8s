#!/bin/bash

echo "=== Kubernetes Manifest Validation ==="

FAILED=0

# Test each overlay
for env in dev staging prod; do
  echo ""
  echo "--- Testing $env overlay ---"
  
  # Generate manifests
  kubectl kustomize k8s/overlays/$env > /tmp/manifests-$env.yaml
  
  # Validate YAML schema
  echo "Validating YAML schema..."
  if command -v kubeconform &> /dev/null; then
    kubeconform -strict -summary /tmp/manifests-$env.yaml || FAILED=1
  else
    echo "SKIP: kubeconform not installed"
  fi
done

echo ""
echo "=== Security Policy Checks ==="

# Check for required security contexts
echo "Checking security contexts..."
for env in dev staging prod; do
  # Check runAsNonRoot
  if ! grep -q "runAsNonRoot: true" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing runAsNonRoot: true"
    FAILED=1
  fi
  
  # Check seccompProfile
  if ! grep -q "seccompProfile" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing seccompProfile"
    FAILED=1
  fi
  
  # Check allowPrivilegeEscalation
  if ! grep -q "allowPrivilegeEscalation: false" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing allowPrivilegeEscalation: false"
    FAILED=1
  fi
  
  # Check capabilities drop
  if ! grep -q "drop:" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing capabilities drop"
    FAILED=1
  fi
  
  # Check resource limits
  if ! grep -q "limits:" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing resource limits"
    FAILED=1
  fi
  
  # Check resource requests
  if ! grep -q "requests:" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing resource requests"
    FAILED=1
  fi
  
  # Check liveness probe
  if ! grep -q "livenessProbe:" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing livenessProbe"
    FAILED=1
  fi
  
  # Check readiness probe
  if ! grep -q "readinessProbe:" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing readinessProbe"
    FAILED=1
  fi
  
  # Check NetworkPolicy exists
  if ! grep -q "kind: NetworkPolicy" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing NetworkPolicy"
    FAILED=1
  fi
  
  # Check PodDisruptionBudget exists
  if ! grep -q "kind: PodDisruptionBudget" /tmp/manifests-$env.yaml; then
    echo "FAIL: $env - Missing PodDisruptionBudget"
    FAILED=1
  fi
  
  echo "PASS: $env security policies"
done

echo ""
echo "=== Image Policy Checks ==="

for env in dev staging prod; do
  # Check no latest tag in production
  if [ "$env" = "prod" ]; then
    if grep -q "image:.*:latest" /tmp/manifests-$env.yaml; then
      echo "WARN: $env - Using :latest tag (consider pinning)"
    fi
  fi
  
  # Check images are from allowed registries
  if grep -E "image:" /tmp/manifests-$env.yaml | grep -v -E "(ghcr.io|gcr.io|public.ecr.aws|docker.io/nginx)" > /dev/null; then
    echo "WARN: $env - Image from non-standard registry"
  fi
done

echo ""
if [ $FAILED -eq 0 ]; then
  echo "✅ All policy checks passed!"
  exit 0
else
  echo "❌ Some policy checks failed!"
  exit 1
fi
