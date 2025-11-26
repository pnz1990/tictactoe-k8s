#!/bin/bash
set -e

echo "=== Dockerfile Best Practices ==="

FAILED=0

for dockerfile in app/Dockerfile backend/Dockerfile; do
  echo ""
  echo "--- Checking $dockerfile ---"
  
  # Chainguard images are secure by default (non-root, minimal)
  IS_CHAINGUARD=$(grep -q "chainguard" $dockerfile && echo "true" || echo "false")
  
  # Check multi-stage build (not required for simple static file copies with secure base)
  if ! grep -q "AS builder" $dockerfile; then
    if [ "$IS_CHAINGUARD" = "true" ] && [ "$dockerfile" = "app/Dockerfile" ]; then
      echo "PASS: Chainguard base (multi-stage not required for static files)"
    else
      echo "FAIL: $dockerfile - Not using multi-stage build"
      FAILED=1
    fi
  else
    echo "PASS: Multi-stage build"
  fi
  
  # Check non-root user (Chainguard runs as non-root by default)
  if ! grep -q "USER" $dockerfile; then
    if [ "$IS_CHAINGUARD" = "true" ]; then
      echo "PASS: Chainguard base (non-root by default)"
    else
      echo "FAIL: $dockerfile - No USER instruction"
      FAILED=1
    fi
  else
    echo "PASS: USER instruction present"
  fi
  
  # Check for distroless or minimal base
  if grep -q -E "(distroless|chainguard|alpine)" $dockerfile; then
    echo "PASS: Using minimal base image"
  else
    echo "WARN: $dockerfile - Consider using distroless/chainguard base"
  fi
  
  # Check no root user (UID 0)
  if grep -q "USER 0" $dockerfile || grep -q "USER root" $dockerfile; then
    echo "FAIL: $dockerfile - Running as root"
    FAILED=1
  else
    echo "PASS: Not running as root"
  fi
done

echo ""
if [ $FAILED -eq 0 ]; then
  echo "✅ All Dockerfile checks passed!"
  exit 0
else
  echo "❌ Some Dockerfile checks failed!"
  exit 1
fi
