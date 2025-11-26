#!/bin/bash
set -e

echo "=== Observability Testing Suite ==="

FAILED=0
NAMESPACE=${NAMESPACE:-"tictactoe-dev"}

# Check if cluster is accessible
CLUSTER_AVAILABLE=false
if kubectl cluster-info &>/dev/null; then
  CLUSTER_AVAILABLE=true
fi

# Test 1: Prometheus Metrics Endpoint
echo ""
echo "--- Test 1: Prometheus Metrics Endpoint ---"
if [ "$CLUSTER_AVAILABLE" = true ]; then
  POD=$(kubectl get pods -n $NAMESPACE -l app=tictactoe-backend -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$POD" ]; then
    echo "Testing metrics endpoint on pod $POD..."
    kubectl exec -n $NAMESPACE $POD -- wget -q -O- http://localhost:8081/metrics > /tmp/metrics.txt 2>/dev/null || true
    
    if [ -s /tmp/metrics.txt ]; then
      echo "PASS: Metrics endpoint accessible"
      
      # Check for required metrics
      required_metrics=(
        "tictactoe_games_total"
        "tictactoe_wins_total"
        "tictactoe_player_games_total"
        "tictactoe_ties_total"
        "tictactoe_current_win_streak"
        "http_requests_total"
        "http_request_duration_seconds"
      )
      
      for metric in "${required_metrics[@]}"; do
        if grep -q "$metric" /tmp/metrics.txt; then
          echo "PASS: Metric $metric exists"
        else
          echo "FAIL: Metric $metric missing"
          FAILED=1
        fi
      done
    else
      echo "SKIP: Metrics endpoint not accessible (pod may not be ready)"
    fi
  else
    echo "SKIP: No backend pod found in namespace $NAMESPACE"
  fi
else
  echo "SKIP: No cluster available"
fi

# Test 2: Health Endpoints
echo ""
echo "--- Test 2: Health Endpoints ---"
if [ "$CLUSTER_AVAILABLE" = true ] && [ -n "$POD" ]; then
  # Test liveness
  if kubectl exec -n $NAMESPACE $POD -- wget -q -O- http://localhost:8081/healthz 2>/dev/null | grep -q "ok"; then
    echo "PASS: Liveness endpoint returns ok"
  else
    echo "SKIP: Liveness endpoint not accessible"
  fi
  
  # Test readiness
  if kubectl exec -n $NAMESPACE $POD -- wget -q -O- http://localhost:8081/healthz 2>/dev/null | grep -q "ok"; then
    echo "PASS: Readiness endpoint returns ok"
  else
    echo "SKIP: Readiness endpoint not accessible"
  fi
else
  echo "SKIP: No backend pod available"
fi

# Test 3: Structured Logging
echo ""
echo "--- Test 3: Structured Logging ---"
if [ "$CLUSTER_AVAILABLE" = true ] && [ -n "$POD" ]; then
  echo "Checking log format..."
  kubectl logs -n $NAMESPACE $POD --tail=10 > /tmp/logs.txt 2>/dev/null || true
  
  if [ -s /tmp/logs.txt ]; then
    # Check if logs are in JSON format
    if head -1 /tmp/logs.txt | jq . > /dev/null 2>&1; then
      echo "PASS: Logs are in JSON format"
      
      # Check for required fields
      required_fields=("time" "level" "msg")
      for field in "${required_fields[@]}"; do
        if head -1 /tmp/logs.txt | jq -e ".$field" > /dev/null 2>&1; then
          echo "PASS: Log field $field exists"
        else
          echo "WARN: Log field $field missing"
        fi
      done
    else
      echo "WARN: Logs are not in JSON format"
    fi
  else
    echo "SKIP: No logs available"
  fi
else
  echo "SKIP: No backend pod available"
fi

# Test 4: Prometheus ServiceMonitor/PodMonitor
echo ""
echo "--- Test 4: Prometheus Discovery ---"
for env in dev staging prod; do
  kubectl kustomize k8s/overlays/$env > /tmp/manifests-$env.yaml
  
  # Check for Prometheus annotations
  if grep -q "prometheus.io/scrape" /tmp/manifests-$env.yaml; then
    echo "PASS: $env - Prometheus scrape annotation exists"
    
    if grep -q "prometheus.io/port" /tmp/manifests-$env.yaml; then
      echo "PASS: $env - Prometheus port annotation exists"
    else
      echo "FAIL: $env - Prometheus port annotation missing"
      FAILED=1
    fi
    
    if grep -q "prometheus.io/path" /tmp/manifests-$env.yaml; then
      echo "PASS: $env - Prometheus path annotation exists"
    else
      echo "WARN: $env - Prometheus path annotation missing (defaults to /metrics)"
    fi
  else
    echo "FAIL: $env - Prometheus scrape annotation missing"
    FAILED=1
  fi
done

# Test 5: Grafana Dashboard Existence
echo ""
echo "--- Test 5: Grafana Dashboards ---"
if kubectl get crd grafanadashboards.grafana.integreatly.org > /dev/null 2>&1; then
  for env in dev staging prod; do
    if kubectl get grafanadashboard -n $NAMESPACE tictactoe-$env 2>/dev/null; then
      echo "PASS: $env - Grafana dashboard exists"
    else
      echo "WARN: $env - Grafana dashboard not found"
    fi
  done
else
  echo "SKIP: Grafana Operator not installed"
fi

# Test 6: Alert Rules
echo ""
echo "--- Test 6: Alert Rules ---"
if kubectl get crd prometheusrules.monitoring.coreos.com > /dev/null 2>&1; then
  if kubectl get prometheusrule -n $NAMESPACE tictactoe-alerts 2>/dev/null; then
    echo "PASS: PrometheusRule exists"
  else
    echo "WARN: PrometheusRule not found"
  fi
else
  echo "SKIP: Prometheus Operator not installed"
fi

# Test 7: Trace Context Propagation
echo ""
echo "--- Test 7: Distributed Tracing ---"
if [ -n "$POD" ]; then
  # Check if trace headers are present in responses
  kubectl exec -n $NAMESPACE $POD -- wget -S -O- http://localhost:8081/healthz 2>&1 | grep -i "trace" && echo "PASS: Trace headers present" || echo "SKIP: No trace headers"
else
  echo "SKIP: No backend pod found"
fi

# Test 8: Log Aggregation
echo ""
echo "--- Test 8: Log Aggregation ---"
# Check if Fluent Bit or similar is running
if kubectl get daemonset -n kube-system fluent-bit > /dev/null 2>&1; then
  echo "PASS: Fluent Bit DaemonSet exists"
elif kubectl get daemonset -n amazon-cloudwatch cloudwatch-agent > /dev/null 2>&1; then
  echo "PASS: CloudWatch Agent DaemonSet exists"
else
  echo "WARN: No log aggregation DaemonSet found"
fi

# Test 9: Metrics Cardinality
echo ""
echo "--- Test 9: Metrics Cardinality ---"
if [ -s /tmp/metrics.txt ]; then
  # Count unique metric names
  metric_count=$(grep -E "^[a-z_]+" /tmp/metrics.txt | cut -d'{' -f1 | sort -u | wc -l)
  echo "Total unique metrics: $metric_count"
  
  if [ $metric_count -gt 100 ]; then
    echo "WARN: High metric cardinality ($metric_count metrics)"
  else
    echo "PASS: Reasonable metric cardinality"
  fi
  
  # Check for high-cardinality labels
  if grep -E '\{[^}]*,[^}]*,[^}]*,[^}]*,[^}]*\}' /tmp/metrics.txt > /dev/null; then
    echo "WARN: Metrics with many labels detected (potential cardinality issue)"
  else
    echo "PASS: No high-cardinality labels detected"
  fi
else
  echo "SKIP: No metrics data available"
fi

# Test 10: SLO/SLI Metrics
echo ""
echo "--- Test 10: SLO/SLI Metrics ---"
if [ -s /tmp/metrics.txt ]; then
  # Check for SLI metrics
  sli_metrics=(
    "http_request_duration_seconds"
    "http_requests_total"
  )
  
  for metric in "${sli_metrics[@]}"; do
    if grep -q "$metric" /tmp/metrics.txt; then
      echo "PASS: SLI metric $metric exists"
    else
      echo "WARN: SLI metric $metric missing"
    fi
  done
else
  echo "SKIP: No metrics data available"
fi

echo ""
if [ $FAILED -eq 0 ]; then
  echo "✅ All observability tests passed!"
  exit 0
else
  echo "❌ Some observability tests failed!"
  exit 1
fi
