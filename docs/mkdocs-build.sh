#!/usr/bin/env bash
# Script to build MkDocs documentation using Docker/Podman
#
# NOTE: This script is for local development only and is not used in CI/CD.
# It allows developers to build documentation locally without installing MkDocs.

# Determine if we're using podman or docker
if command -v podman &> /dev/null; then
    CONTAINER_ENGINE="podman"
elif command -v docker &> /dev/null; then
    CONTAINER_ENGINE="docker"
else
    echo "Error: Neither podman nor docker found. Please install one of them to continue."
    exit 1
fi

echo "Using ${CONTAINER_ENGINE} to build MkDocs documentation..."

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="${SCRIPT_DIR}"
OUTPUT_DIR="${DOCS_DIR}/site"

# Print some info
echo "Documentation directory: ${DOCS_DIR}"
echo "Output directory: ${OUTPUT_DIR}"

# Make sure the output directory exists and is empty
mkdir -p "${OUTPUT_DIR}"
rm -rf "${OUTPUT_DIR:?}"/* 2>/dev/null || true

# Run the container to build the docs
${CONTAINER_ENGINE} run --rm -it \
    -v "${DOCS_DIR}:/docs" \
    -u "$(id -u):$(id -g)" \
    squidfunk/mkdocs-material:latest build

echo "Build complete! Documentation is available in: ${OUTPUT_DIR}"
echo "You can preview it by running: python3 -m http.server --directory ${OUTPUT_DIR}"
