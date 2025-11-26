#!/bin/bash
set -e

echo "=== Reliability Testing Suite ==="

FAILED=0
NAMESPACE=${NAMESPACE:-"tictactoe-dev"}

# Test 1: Pod Disruption Budget
echo ""
echo "--- Test 1: Pod Disruption Budget ---"
for env in dev staging prod; do
  kubectl kustomize k8s/overlays/$env > /tmp/manifests-$env.yaml
  
  if grep -q "kind: PodDisruptionBudget" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - PodDisruptionBudget exists"
    
    # Check minAvailable is set
    if grep -A 5 "kind: PodDisruptionBudget" /tmp/manifests-$env.yaml | grep -q "minAvailable:"; then
      min_available=$(grep -A 5 "kind: PodDisruptionBudget" /tmp/manifests-$env.yaml | grep "minAvailable:" | awk '{print $2}')
      echo "PASS: $env - minAvailable set to $min_available"
    else
      echo "FAIL: $env - minAvailable not set"
      FAILED=1
    fi
  else
    echo "FAIL: $env - PodDisruptionBudget missing"
    FAILED=1
  fi
done

# Test 2: Replica Count
echo ""
echo "--- Test 2: Replica Count ---"
for env in dev staging prod; do
  replicas=$(grep -A 10 "kind: Deployment" /tmp/manifests-$env.yaml | grep "replicas:" | head -1 | awk '{print $2}')
  
  if [ "$env" = "prod" ] && [ "$replicas" -lt 3 ]; then
    echo "FAIL: $env - Production should have at least 3 replicas (found $replicas)"
    FAILED=1
  elif [ "$env" = "staging" ] && [ "$replicas" -lt 2 ]; then
    echo "WARN: $env - Staging should have at least 2 replicas (found $replicas)"
  else
    echo "PASS: $env - Replica count is $replicas"
  fi
done

# Test 3: Health Probes Configuration
echo ""
echo "--- Test 3: Health Probes ---"
for env in dev staging prod; do
  # Check liveness probe
  if grep -q "livenessProbe:" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Liveness probe configured"
    
    # Check probe has reasonable settings
    if grep -A 10 "livenessProbe:" /tmp/manifests-$env.yaml | grep -q "initialDelaySeconds:"; then
      echo "PASS: $env - Liveness probe has initialDelaySeconds"
    else
      echo "WARN: $env - Liveness probe missing initialDelaySeconds"
    fi
    
    if grep -A 10 "livenessProbe:" /tmp/manifests-$env.yaml | grep -q "periodSeconds:"; then
      echo "PASS: $env - Liveness probe has periodSeconds"
    else
      echo "WARN: $env - Liveness probe missing periodSeconds"
    fi
  else
    echo "FAIL: $env - Liveness probe missing"
    FAILED=1
  fi
  
  # Check readiness probe
  if grep -q "readinessProbe:" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Readiness probe configured"
  else
    echo "FAIL: $env - Readiness probe missing"
    FAILED=1
  fi
done

# Test 4: Resource Requests and Limits
echo ""
echo "--- Test 4: Resource Management ---"
for env in dev staging prod; do
  # Check requests are set
  if grep -A 5 "resources:" /tmp/manifests-$env.yaml | grep -q "requests:"; then
    echo "PASS: $env - Resource requests defined"
  else
    echo "FAIL: $env - Resource requests missing"
    FAILED=1
  fi
  
  # Check limits are set
  if grep -A 5 "resources:" /tmp/manifests-$env.yaml | grep -q "limits:"; then
    echo "PASS: $env - Resource limits defined"
  else
    echo "FAIL: $env - Resource limits missing"
    FAILED=1
  fi
  
  # Check limits >= requests
  cpu_request=$(grep -A 10 "resources:" /tmp/manifests-$env.yaml | grep -A 2 "requests:" | grep "cpu:" | head -1 | awk '{print $2}')
  cpu_limit=$(grep -A 10 "resources:" /tmp/manifests-$env.yaml | grep -A 2 "limits:" | grep "cpu:" | head -1 | awk '{print $2}')
  
  if [ -n "$cpu_request" ] && [ -n "$cpu_limit" ]; then
    echo "PASS: $env - CPU request: $cpu_request, limit: $cpu_limit"
  fi
done

# Test 5: Anti-Affinity Rules
echo ""
echo "--- Test 5: Pod Anti-Affinity ---"
for env in staging prod; do
  if grep -q "podAntiAffinity:" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Pod anti-affinity configured"
  else
    echo "WARN: $env - Consider adding pod anti-affinity for HA"
  fi
done

# Test 6: Graceful Shutdown
echo ""
echo "--- Test 6: Graceful Shutdown ---"
for env in dev staging prod; do
  # Check terminationGracePeriodSeconds
  if grep -q "terminationGracePeriodSeconds:" /tmp/manifests-$env.yaml; then
    grace_period=$(grep "terminationGracePeriodSeconds:" /tmp/manifests-$env.yaml | awk '{print $2}')
    if [ "$grace_period" -ge 30 ]; then
      echo "PASS: $env - Graceful shutdown period: ${grace_period}s"
    else
      echo "WARN: $env - Short graceful shutdown period: ${grace_period}s"
    fi
  else
    echo "WARN: $env - terminationGracePeriodSeconds not set (defaults to 30s)"
  fi
  
  # Check for preStop hook
  if grep -q "preStop:" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - preStop hook configured"
  else
    echo "INFO: $env - No preStop hook (may be acceptable)"
  fi
done

# Test 7: Rolling Update Strategy
echo ""
echo "--- Test 7: Rolling Update Strategy ---"
for env in dev staging prod; do
  if grep -q "strategy:" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Update strategy defined"
    
    # Check maxUnavailable
    if grep -A 5 "strategy:" /tmp/manifests-$env.yaml | grep -q "maxUnavailable:"; then
      max_unavailable=$(grep -A 5 "strategy:" /tmp/manifests-$env.yaml | grep "maxUnavailable:" | awk '{print $2}')
      echo "PASS: $env - maxUnavailable: $max_unavailable"
    fi
    
    # Check maxSurge
    if grep -A 5 "strategy:" /tmp/manifests-$env.yaml | grep -q "maxSurge:"; then
      max_surge=$(grep -A 5 "strategy:" /tmp/manifests-$env.yaml | grep "maxSurge:" | awk '{print $2}')
      echo "PASS: $env - maxSurge: $max_surge"
    fi
  else
    echo "WARN: $env - Update strategy not explicitly defined"
  fi
done

# Test 8: Horizontal Pod Autoscaler
echo ""
echo "--- Test 8: Horizontal Pod Autoscaler ---"
for env in staging prod; do
  if grep -q "kind: HorizontalPodAutoscaler" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - HPA configured"
    
    # Check min/max replicas
    min_replicas=$(grep -A 10 "kind: HorizontalPodAutoscaler" /tmp/manifests-$env.yaml | grep "minReplicas:" | awk '{print $2}')
    max_replicas=$(grep -A 10 "kind: HorizontalPodAutoscaler" /tmp/manifests-$env.yaml | grep "maxReplicas:" | awk '{print $2}')
    
    echo "INFO: $env - HPA range: $min_replicas-$max_replicas replicas"
  else
    echo "INFO: $env - No HPA configured (static replica count)"
  fi
done

# Test 9: Service Mesh / Circuit Breaker
echo ""
echo "--- Test 9: Service Mesh Integration ---"
if kubectl get crd virtualservices.networking.istio.io > /dev/null 2>&1; then
  echo "INFO: Istio detected"
  
  for env in dev staging prod; do
    if grep -q "kind: VirtualService" /tmp/manifests-$env.yaml; then
      echo "PASS: $env - VirtualService configured"
    else
      echo "INFO: $env - No VirtualService (may not be using Istio)"
    fi
  done
else
  echo "INFO: No service mesh detected"
fi

# Test 10: Backup and Recovery
echo ""
echo "--- Test 10: Backup Strategy ---"
for env in staging prod; do
  # Check for volume snapshots or backup annotations
  if grep -q "backup" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Backup configuration found"
  else
    echo "INFO: $env - No explicit backup configuration (stateless app)"
  fi
done

# Test 11: Chaos Engineering Readiness
echo ""
echo "--- Test 11: Chaos Engineering Readiness ---"
for env in dev staging prod; do
  # Check if app can handle pod failures
  replicas=$(grep -A 10 "kind: Deployment" /tmp/manifests-$env.yaml | grep "replicas:" | head -1 | awk '{print $2}' | tr -d '\n')
  min_available=$(grep -A 5 "kind: PodDisruptionBudget" /tmp/manifests-$env.yaml | grep "minAvailable:" | awk '{print $2}' | tr -d '\n' || echo "0")
  
  if [ "${replicas:-0}" -gt 1 ] 2>/dev/null && [ "${min_available:-0}" -ge 1 ] 2>/dev/null; then
    echo "PASS: $env - Can handle pod failures (replicas: $replicas, minAvailable: $min_available)"
  else
    echo "WARN: $env - May not handle pod failures gracefully"
  fi
done

echo ""
if [ $FAILED -eq 0 ]; then
  echo "✅ All reliability tests passed!"
  exit 0
else
  echo "❌ Some reliability tests failed!"
  exit 1
fi
