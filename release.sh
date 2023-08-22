#!/bin/bash

# Name for our Docker images
STALE_IMAGE="stale-image"

# Create a local k3d cluster
k3d cluster create mycluster

# Build Docker image for the stale test
docker build -t $STALE_IMAGE -f StaleDockerfile .

# Import images to k3d
k3d image import $STALE_IMAGE -c mycluster

# Deploy a sample test stale pod
kubectl apply -f stale-deployment.yaml

# Use application to detect stale pods
go build -o InspectorKoti
./InspectorKoti --kubeconfig ~/.kube/config --dry-run true

# Clean up
k3d cluster delete mycluster