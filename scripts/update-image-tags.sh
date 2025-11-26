#!/bin/bash
set -e

COMMIT_SHA=$(git rev-parse HEAD)

echo "Updating image tags to commit SHA: $COMMIT_SHA"

# Update dev overlay
yq eval -i ".images[0].newTag = \"$COMMIT_SHA\"" k8s/overlays/dev/kustomization.yaml
yq eval -i ".images[1].newTag = \"$COMMIT_SHA\"" k8s/overlays/dev/kustomization.yaml

echo "✅ Updated dev overlay with SHA: $COMMIT_SHA"
