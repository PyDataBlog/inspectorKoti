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
./InspectorKoti --kubeconfig ~/.kube/config --namespace default --deployment stale-deployment --dry-run --period 90 --threshold 100 --timeout 240 --debug

# Clean up
k3d cluster delete mycluster