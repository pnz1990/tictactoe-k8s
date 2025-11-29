#!/bin/bash
set -e

echo "=== Policy Testing Suite (KRO) ==="

FAILED=0
RGD_FILE="k8s/kro/tictactoe-rgd.yaml"

# Test 1: Security Context
echo ""
echo "--- Test 1: Security Context ---"
if grep -q "securityContext:" "$RGD_FILE"; then
  echo "PASS: Security context defined"
  
  if grep -q "runAsNonRoot: true" "$RGD_FILE"; then
    echo "PASS: runAsNonRoot enabled"
  else
    echo "FAIL: runAsNonRoot not set"
    FAILED=1
  fi
  
  if grep -q "readOnlyRootFilesystem: true" "$RGD_FILE"; then
    echo "PASS: readOnlyRootFilesystem enabled"
  else
    echo "FAIL: readOnlyRootFilesystem not set"
    FAILED=1
  fi
else
  echo "FAIL: Security context missing"
  FAILED=1
fi

# Test 2: Resource Limits
echo ""
echo "--- Test 2: Resource Limits ---"
if grep -q "limits:" "$RGD_FILE" && grep -q "requests:" "$RGD_FILE"; then
  echo "PASS: Resource limits and requests defined"
else
  echo "FAIL: Resource limits/requests missing"
  FAILED=1
fi

# Test 3: Health Probes
echo ""
echo "--- Test 3: Health Probes ---"
if grep -q "livenessProbe:" "$RGD_FILE" && grep -q "readinessProbe:" "$RGD_FILE"; then
  echo "PASS: Health probes configured"
else
  echo "FAIL: Health probes missing"
  FAILED=1
fi

# Test 4: KRO Instance Files
echo ""
echo "--- Test 4: KRO Instance Validation ---"
for env in dev staging prod; do
  INSTANCE_FILE="k8s/kro/$env/$env.yaml"
  if [ -f "$INSTANCE_FILE" ]; then
    echo "PASS: $env instance file exists"
    
    # Validate YAML syntax
    if python3 -c "import yaml; yaml.safe_load(open('$INSTANCE_FILE'))" 2>/dev/null || \
       ruby -ryaml -e "YAML.load_file('$INSTANCE_FILE')" 2>/dev/null || \
       kubectl apply --dry-run=client -f "$INSTANCE_FILE" 2>/dev/null; then
      echo "PASS: $env instance YAML valid"
    else
      echo "WARN: Could not validate $env YAML syntax"
    fi
  else
    echo "FAIL: $env instance file missing"
    FAILED=1
  fi
done

# Test 5: RGD Structure
echo ""
echo "--- Test 5: RGD Structure ---"
if [ -f "$RGD_FILE" ]; then
  if grep -q "kind: ResourceGraphDefinition" "$RGD_FILE"; then
    echo "PASS: Valid RGD kind"
  else
    echo "FAIL: Invalid RGD kind"
    FAILED=1
  fi
  
  if grep -q "schema:" "$RGD_FILE" && grep -q "resources:" "$RGD_FILE"; then
    echo "PASS: RGD has schema and resources"
  else
    echo "FAIL: RGD missing schema or resources"
    FAILED=1
  fi
else
  echo "FAIL: RGD file not found"
  FAILED=1
fi

echo ""
echo "=== Policy Tests Complete ==="
if [ $FAILED -eq 0 ]; then
  echo "All policy tests passed!"
  exit 0
else
  echo "Some policy tests failed"
  exit 1
fi
