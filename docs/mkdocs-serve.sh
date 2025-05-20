#!/usr/bin/env bash
# Script to serve MkDocs documentation using Docker/Podman
# 
# NOTE: This script is for local development only and is not used in CI/CD.
# It allows developers to preview documentation locally without installing MkDocs.

# Default port to 8000 if not set in environment
PORT=${PORT:-8000}

# Determine if we're using podman or docker
if command -v podman &> /dev/null; then
    CONTAINER_ENGINE="podman"
elif command -v docker &> /dev/null; then
    CONTAINER_ENGINE="docker"
else
    echo "Error: Neither podman nor docker found. Please install one of them to continue."
    exit 1
fi

echo "Using ${CONTAINER_ENGINE} to serve MkDocs documentation on port ${PORT}..."

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="${SCRIPT_DIR}"

# Print some info
echo "Documentation directory: ${DOCS_DIR}"
echo "This will be available at http://localhost:${PORT}"
echo "Press Ctrl+C to stop the server"

# Run the container
${CONTAINER_ENGINE} run --rm -it \
    -p ${PORT}:8000 \
    -v "${DOCS_DIR}:/docs" \
    -u "$(id -u):$(id -g)" \
    squidfunk/mkdocs-material:latest serve --dev-addr=0.0.0.0:8000

echo "MkDocs server stopped"