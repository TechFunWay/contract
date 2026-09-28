#!/bin/bash
set -euo pipefail

# 多平台（linux/amd64 + linux/arm64）合并 manifest 的镜像，两种产物：
#   默认        —— 本地 OCI 归档 release/<VERSION>/techfunway-contract-<VERSION>-multiarch.oci.tar
#   PUSH=1      —— 同一份合并 manifest 推到 Docker Hub（techfunways/contract 的
#                 :<VERSION> 与 :latest），需先 docker login 且账号能写该命名空间
#   SKIP_OCI=1  —— 跳过本地归档（只推送时用，不重打已随发行目录分发的归档）
# 也支持 --push 作为 PUSH=1 的等价参数。两个开关都不开时与历史行为完全一致：
# 只导出本地归档，不碰任何远端（未经用户确认不推送）。

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

PUSH="${PUSH:-0}"
SKIP_OCI="${SKIP_OCI:-0}"
for arg in "$@"; do
  [ "$arg" = "--push" ] && PUSH=1
done

IMAGE="techfunways/contract"
VERSION="$(cat VERSION | tr -d '\n')"
BUILD_TIME="$(date +%Y-%m-%dT%H:%M:%S)"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
OUTPUT_DIR="release/${VERSION}"
OUTPUT_FILE="${OUTPUT_DIR}/techfunway-contract-${VERSION}-multiarch.oci.tar"

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: Docker is required to build the multi-platform image."
  exit 1
fi

mkdir -p "$OUTPUT_DIR"
if [ "${SKIP_OCI}" != "1" ]; then
  rm -f "$OUTPUT_FILE"
fi

# 先用 daemon（其拉取走 daemon 代理配置）把两架构的基础镜像内容全部物化到
# 本地：buildkit 的 blob/token 请求不走代理，网络上 Docker Hub 直连被污染时
# 会取不到内容导致整个 solve 失败；内容齐了 kit 就只用本地，不再要 token。
# 网络畅通的机器上这一步等价于预热，无副作用。
echo "Materializing base images for linux/amd64 + linux/arm64..."
for ARCH in amd64 arm64; do
  for BASE_IMAGE in node:20-alpine golang:1.26-alpine alpine:3.20; do
    docker run --platform "linux/${ARCH}" --rm "${BASE_IMAGE}" true >/dev/null 2>&1 \
      || docker pull --platform "linux/${ARCH}" "${BASE_IMAGE}" >/dev/null
  done
done

# 用默认 builder（daemon 内置 buildkit）直接导出 OCI 归档：
# 一个归档同时携带两种架构的合并 manifest。
if [ "${SKIP_OCI}" != "1" ]; then
  echo "Building ${IMAGE}:${VERSION} for linux/amd64,linux/arm64..."
  echo "Writing local OCI image archive (no registry push): ${OUTPUT_FILE}"
  docker buildx build \
    --builder default \
    --platform linux/amd64,linux/arm64 \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "BUILD_TIME=${BUILD_TIME}" \
    --build-arg "GIT_COMMIT=${GIT_COMMIT}" \
    -t "${IMAGE}:${VERSION}" \
    -t "${IMAGE}:latest" \
    --output "type=oci,dest=${OUTPUT_FILE}" \
    .
  echo "Multi-platform OCI image archive completed: ${OUTPUT_FILE}"
  echo "Load it on a target with: docker load -i ${OUTPUT_FILE}"
fi

# PUSH=1：把同一份 amd64+arm64 合并 manifest 推到 Docker Hub
if [ "${PUSH}" = "1" ]; then
  echo "Pushing ${IMAGE}:${VERSION} and ${IMAGE}:latest (linux/amd64 + linux/arm64)..."
  docker buildx build \
    --builder default \
    --platform linux/amd64,linux/arm64 \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "BUILD_TIME=${BUILD_TIME}" \
    --build-arg "GIT_COMMIT=${GIT_COMMIT}" \
    -t "${IMAGE}:${VERSION}" \
    -t "${IMAGE}:latest" \
    --push \
    .
  echo "Pushed to Docker Hub: ${IMAGE}:${VERSION} / ${IMAGE}:latest"
fi
