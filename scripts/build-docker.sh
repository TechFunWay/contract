#!/bin/bash
set -e

# 构建本机 docker 镜像，tag 与家族发行约定一致：
#   techfunways/contract:<VERSION>（如 v0.1.0）与 techfunways/contract:latest
IMAGE="techfunways/contract"
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Building Docker image..."
docker build \
  --build-arg VERSION=${VERSION} \
  --build-arg BUILD_TIME=${BUILD_TIME} \
  --build-arg GIT_COMMIT=${GIT_COMMIT} \
  -t ${IMAGE}:${VERSION} \
  -t ${IMAGE}:latest \
  .

echo "Docker image built: ${IMAGE}:${VERSION}"
