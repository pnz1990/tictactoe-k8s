#!/bin/bash
set -e

echo "=== End-to-End Testing Suite ==="

FAILED=0
NAMESPACE=${NAMESPACE:-"tictactoe-dev"}
TIMEOUT=300

# Test 1: Full Deployment Flow
echo ""
echo "--- Test 1: Full Deployment Flow ---"
echo "Deploying application to $NAMESPACE..."

kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k k8s/overlays/dev -n $NAMESPACE

echo "Waiting for deployment to be ready..."
if kubectl wait --for=condition=available --timeout=${TIMEOUT}s deployment/tictactoe-frontend -n $NAMESPACE; then
  echo "PASS: Frontend deployment ready"
else
  echo "FAIL: Frontend deployment failed"
  FAILED=1
fi

if kubectl wait --for=condition=available --timeout=${TIMEOUT}s deployment/tictactoe-backend -n $NAMESPACE; then
  echo "PASS: Backend deployment ready"
else
  echo "FAIL: Backend deployment failed"
  FAILED=1
fi

# Test 2: Service Connectivity
echo ""
echo "--- Test 2: Service Connectivity ---"
FRONTEND_POD=$(kubectl get pods -n $NAMESPACE -l app=tictactoe-frontend -o jsonpath='{.items[0].metadata.name}')
BACKEND_POD=$(kubectl get pods -n $NAMESPACE -l app=tictactoe-backend -o jsonpath='{.items[0].metadata.name}')

if [ -n "$FRONTEND_POD" ]; then
  echo "Testing frontend service..."
  if kubectl exec -n $NAMESPACE $FRONTEND_POD -- wget -q -O- http://localhost:8080/ > /dev/null; then
    echo "PASS: Frontend service responding"
  else
    echo "FAIL: Frontend service not responding"
    FAILED=1
  fi
else
  echo "FAIL: Frontend pod not found"
  FAILED=1
fi

if [ -n "$BACKEND_POD" ]; then
  echo "Testing backend service..."
  if kubectl exec -n $NAMESPACE $BACKEND_POD -- wget -q -O- http://localhost:8081/healthz | grep -q "ok"; then
    echo "PASS: Backend service responding"
  else
    echo "FAIL: Backend service not responding"
    FAILED=1
  fi
else
  echo "FAIL: Backend pod not found"
  FAILED=1
fi

# Test 3: Frontend-Backend Communication
echo ""
echo "--- Test 3: Frontend-Backend Communication ---"
if [ -n "$FRONTEND_POD" ] && [ -n "$BACKEND_POD" ]; then
  # Get backend service ClusterIP
  BACKEND_SVC=$(kubectl get svc -n $NAMESPACE tictactoe-backend -o jsonpath='{.spec.clusterIP}')
  
  echo "Testing frontend -> backend communication..."
  if kubectl exec -n $NAMESPACE $FRONTEND_POD -- wget -q -O- http://$BACKEND_SVC:8081/healthz | grep -q "ok"; then
    echo "PASS: Frontend can reach backend"
  else
    echo "FAIL: Frontend cannot reach backend"
    FAILED=1
  fi
fi

# Test 4: Game Submission Flow
echo ""
echo "--- Test 4: Game Submission Flow ---"
if [ -n "$BACKEND_POD" ]; then
  echo "Submitting test game..."
  GAME_DATA='{"player1":"Alice","player2":"Bob","winner":"Alice","pattern":"row1","isTie":false}'
  
  if kubectl exec -n $NAMESPACE $BACKEND_POD -- wget -q -O- --post-data="$GAME_DATA" --header="Content-Type: application/json" http://localhost:8081/api/game > /dev/null; then
    echo "PASS: Game submission successful"
    
    # Verify metrics updated
    sleep 2
    kubectl exec -n $NAMESPACE $BACKEND_POD -- wget -q -O- http://localhost:8081/metrics > /tmp/metrics.txt
    
    if grep -q "tictactoe_games_total" /tmp/metrics.txt; then
      echo "PASS: Metrics updated after game submission"
    else
      echo "FAIL: Metrics not updated"
      FAILED=1
    fi
  else
    echo "FAIL: Game submission failed"
    FAILED=1
  fi
fi

# Test 5: Ingress Routing
echo ""
echo "--- Test 5: Ingress Routing ---"
if kubectl get ingress -n $NAMESPACE tictactoe > /dev/null 2>&1; then
  echo "PASS: Ingress exists"
  
  # Check ingress rules
  if kubectl get ingress -n $NAMESPACE tictactoe -o yaml | grep -q "path: /api"; then
    echo "PASS: Backend API route configured"
  else
    echo "FAIL: Backend API route missing"
    FAILED=1
  fi
  
  if kubectl get ingress -n $NAMESPACE tictactoe -o yaml | grep -q "path: /"; then
    echo "PASS: Frontend route configured"
  else
    echo "FAIL: Frontend route missing"
    FAILED=1
  fi
else
  echo "WARN: Ingress not found (may be using LoadBalancer)"
fi

# Test 6: Pod Restart Resilience
echo ""
echo "--- Test 6: Pod Restart Resilience ---"
if [ -n "$BACKEND_POD" ]; then
  echo "Deleting backend pod to test resilience..."
  kubectl delete pod -n $NAMESPACE $BACKEND_POD
  
  echo "Waiting for new pod to be ready..."
  if kubectl wait --for=condition=ready pod -l app=tictactoe-backend -n $NAMESPACE --timeout=120s; then
    echo "PASS: Pod recovered after deletion"
  else
    echo "FAIL: Pod did not recover"
    FAILED=1
  fi
fi

# Test 7: Network Policy Enforcement
echo ""
echo "--- Test 7: Network Policy Enforcement ---"
if kubectl get networkpolicy -n $NAMESPACE > /dev/null 2>&1; then
  echo "PASS: NetworkPolicy exists"
  
  # Test that pods can communicate within allowed rules
  if [ -n "$FRONTEND_POD" ] && [ -n "$BACKEND_POD" ]; then
    BACKEND_SVC=$(kubectl get svc -n $NAMESPACE tictactoe-backend -o jsonpath='{.spec.clusterIP}')
    
    # Frontend should be able to reach backend
    if kubectl exec -n $NAMESPACE $FRONTEND_POD -- timeout 5 wget -q -O- http://$BACKEND_SVC:8081/healthz > /dev/null 2>&1; then
      echo "PASS: Allowed traffic works"
    else
      echo "FAIL: Allowed traffic blocked"
      FAILED=1
    fi
  fi
else
  echo "WARN: NetworkPolicy not found"
fi

# Test 8: Metrics Scraping
echo ""
echo "--- Test 8: Metrics Scraping ---"
if [ -n "$BACKEND_POD" ]; then
  # Check if metrics are scrapeable
  kubectl exec -n $NAMESPACE $BACKEND_POD -- wget -q -O- http://localhost:8081/metrics > /tmp/metrics.txt
  
  if [ -s /tmp/metrics.txt ]; then
    echo "PASS: Metrics endpoint accessible"
    
    # Check Prometheus format
    if grep -q "# HELP" /tmp/metrics.txt && grep -q "# TYPE" /tmp/metrics.txt; then
      echo "PASS: Metrics in Prometheus format"
    else
      echo "FAIL: Metrics not in Prometheus format"
      FAILED=1
    fi
  else
    echo "FAIL: Metrics endpoint empty"
    FAILED=1
  fi
fi

# Test 9: Log Output
echo ""
echo "--- Test 9: Log Output ---"
if [ -n "$BACKEND_POD" ]; then
  echo "Checking log output..."
  kubectl logs -n $NAMESPACE $BACKEND_POD --tail=5 > /tmp/logs.txt
  
  if [ -s /tmp/logs.txt ]; then
    echo "PASS: Logs are being generated"
    
    # Check for errors in logs
    if grep -i "error\|panic\|fatal" /tmp/logs.txt; then
      echo "WARN: Errors found in logs"
    else
      echo "PASS: No errors in recent logs"
    fi
  else
    echo "WARN: No logs available"
  fi
fi

# Test 10: Resource Consumption
echo ""
echo "--- Test 10: Resource Consumption ---"
if [ -n "$BACKEND_POD" ]; then
  echo "Checking resource usage..."
  kubectl top pod -n $NAMESPACE $BACKEND_POD > /tmp/resources.txt 2>/dev/null || echo "SKIP: metrics-server not available"
  
  if [ -s /tmp/resources.txt ]; then
    cat /tmp/resources.txt
    echo "PASS: Resource metrics available"
  fi
fi

# Cleanup
echo ""
echo "--- Cleanup ---"
kubectl delete namespace $NAMESPACE --wait=false

echo ""
if [ $FAILED -eq 0 ]; then
  echo "✅ All E2E tests passed!"
  exit 0
else
  echo "❌ Some E2E tests failed!"
  exit 1
fi
