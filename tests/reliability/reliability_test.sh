#!/bin/bash
set -e

echo "=== Reliability Testing Suite (KRO) ==="

FAILED=0
RGD_FILE="k8s/kro/tictactoe-rgd.yaml"

# Test 1: Pod Disruption Budget in RGD
echo ""
echo "--- Test 1: Pod Disruption Budget ---"
if grep -q "kind: PodDisruptionBudget" "$RGD_FILE"; then
  echo "PASS: PodDisruptionBudget defined in RGD"
  if grep -A 5 "kind: PodDisruptionBudget" "$RGD_FILE" | grep -q "minAvailable:"; then
    echo "PASS: minAvailable configured"
  else
    echo "FAIL: minAvailable not set"
    FAILED=1
  fi
else
  echo "FAIL: PodDisruptionBudget missing from RGD"
  FAILED=1
fi

# Test 2: Replica Configuration
echo ""
echo "--- Test 2: Replica Configuration ---"
for env in dev staging prod; do
  INSTANCE_FILE="k8s/kro/$env/$env.yaml"
  if [ -f "$INSTANCE_FILE" ]; then
    replicas=$(grep "replicas:" "$INSTANCE_FILE" | head -1 | awk '{print $2}')
    backend_replicas=$(grep "backendReplicas:" "$INSTANCE_FILE" | awk '{print $2}')
    
    if [ "$env" = "prod" ]; then
      if [ "$replicas" -ge 3 ] && [ "$backend_replicas" -ge 3 ]; then
        echo "PASS: $env - Production has $replicas/$backend_replicas replicas"
      else
        echo "FAIL: $env - Production should have at least 3 replicas"
        FAILED=1
      fi
    elif [ "$env" = "staging" ]; then
      if [ "$replicas" -ge 2 ]; then
        echo "PASS: $env - Staging has $replicas/$backend_replicas replicas"
      else
        echo "WARN: $env - Staging should have at least 2 replicas"
      fi
    else
      echo "PASS: $env - Dev has $replicas/$backend_replicas replicas"
    fi
  else
    echo "FAIL: $env - Instance file not found"
    FAILED=1
  fi
done

# Test 3: Health Probes in RGD
echo ""
echo "--- Test 3: Health Probes ---"
if grep -q "livenessProbe:" "$RGD_FILE"; then
  echo "PASS: Liveness probe configured"
else
  echo "FAIL: Liveness probe missing"
  FAILED=1
fi

if grep -q "readinessProbe:" "$RGD_FILE"; then
  echo "PASS: Readiness probe configured"
else
  echo "FAIL: Readiness probe missing"
  FAILED=1
fi

# Test 4: Resource Limits
echo ""
echo "--- Test 4: Resource Limits ---"
if grep -q "resources:" "$RGD_FILE" && grep -q "limits:" "$RGD_FILE"; then
  echo "PASS: Resource limits defined in RGD"
else
  echo "FAIL: Resource limits missing"
  FAILED=1
fi

# Test 5: Instance Resource Configuration
echo ""
echo "--- Test 5: Instance Resource Configuration ---"
for env in dev staging prod; do
  INSTANCE_FILE="k8s/kro/$env/$env.yaml"
  if [ -f "$INSTANCE_FILE" ]; then
    if grep -q "cpuLimit:" "$INSTANCE_FILE" || grep -q "memoryLimit:" "$INSTANCE_FILE"; then
      echo "PASS: $env - Resource limits configured"
    else
      echo "INFO: $env - Using default resource limits"
    fi
  fi
done

# Test 6: Rolling Update Strategy
echo ""
echo "--- Test 6: Deployment Configuration ---"
if grep -q "kind: Deployment" "$RGD_FILE"; then
  echo "PASS: Deployment resource defined"
else
  echo "FAIL: Deployment missing from RGD"
  FAILED=1
fi

echo ""
echo "=== Reliability Tests Complete ==="
if [ $FAILED -eq 0 ]; then
  echo "All reliability tests passed!"
  exit 0
else
  echo "Some reliability tests failed"
  exit 1
fi
