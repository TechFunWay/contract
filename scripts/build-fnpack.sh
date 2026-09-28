#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME=$(python3 -c "import json; print(json.load(open('app.json'))['appname'])")
FNOS_PKG_NAME=$(awk -F'=' '/^appname/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2}' fnpack/manifest)
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=${APP_NAME}"

# 桌面可见主入口必须留在统一网关域（gatewayPrefix / gatewaySocket / url 三件套）：
# 网关域上飞牛会话同源，服务端用 X-Trim-Userid 解析已绑定应用账号。
# type 用 url（2026-09-28 用户要求）——应用中心点开在浏览器新标签页里打开，
# 不要 fnOS 桌面窗口；窗口只留给选飞牛 NAS 账号登录时的授权弹窗（隐藏入口
# FnOSLogin → fnos-entry.html，由 window.open 以页面窗口打开）。
# 注意：2026-09-27 曾因飞牛手机 App 外部 webview 报 ERR_CONNECTION_CLOSED 把主入口
# 改成 iframe，现按用户要求改回 url；App 若再现该问题，按记忆
# fnos-nas-environment.md 的三态结论（内网 fn Connect 登录才是故障态）排查。
python3 - <<'PY'
import json

with open("fnpack/app/ui/config", encoding="utf-8") as source:
    config = json.load(source)
entries = config[".url"]
main = entries.get("techfunway-contract.main")
login = entries.get("techfunway-contract.FnOSLogin")
if not main or main.get("type") != "url":
    raise SystemExit("fnOS desktop main entry must use type=url (opens in a browser tab, not an fnOS desktop window)")
if main.get("type") != "url" or main.get("protocol") != "" or not main.get("gatewayPrefix") or main.get("gatewayPrefix") != "/app/techfunway-contract" or main.get("gatewaySocket") != "app.sock" or main.get("url") != "/app/techfunway-contract":
    raise SystemExit("fnOS main entry must be type=url on the unified gateway (gatewayPrefix=/app/<pkg>, url=/app/<pkg>)")
if not login or login.get("gatewayPrefix") != "/app/techfunway-contract" or login.get("gatewaySocket") != "app.sock":
    raise SystemExit("fnOS hidden entry must use the unified gateway")
if not login.get("noDisplay") or login.get("url") != "/app/techfunway-contract/fnos-entry.html":
    raise SystemExit("fnOS hidden entry must point to the login trampoline and stay hidden")
PY

# Verify Docker is available (required for CGO cross-compilation)
if ! command -v docker &>/dev/null; then
  echo "Error: Docker is required for fnOS package builds (CGO cross-compilation)."
  echo "Install Docker Desktop and try again."
  exit 1
fi

echo "Building frontend..."
VITE_FNOS_APP=true VITE_BASE_PATH="/app/${FNOS_PKG_NAME}/" npm --prefix web ci
VITE_FNOS_APP=true VITE_BASE_PATH="/app/${FNOS_PKG_NAME}/" npm --prefix web run build

echo "Copying frontend..."
rm -rf server/static/dist
cp -r web/dist server/static/dist

BUILD_DIR="release/${VERSION}"
mkdir -p ${BUILD_DIR}

# Save original manifest
cp fnpack/manifest fnpack/manifest.bak

for ARCH in "amd64" "arm64"; do
  echo "Building fnOS package for ${ARCH}..."

  echo "  Compiling Go binary via Docker (CGO_ENABLED=1, linux/${ARCH})..."
  docker run --rm \
    -v "${ROOT_DIR}/server:/src" \
    -v "go-build-cache:/root/.cache/go-build" \
    -v "go-mod-cache:/go/pkg/mod" \
    -w /src \
    --platform "linux/${ARCH}" \
    -e "LDFLAGS=${LDFLAGS}" \
    -e "ARCH=${ARCH}" \
    golang:1.26-alpine \
    sh -c "sed -i 's#dl-cdn.alpinelinux.org/alpine#mirrors.aliyun.com/alpine#g' /etc/apk/repositories && apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go build -ldflags \"$LDFLAGS -extldflags -static\" -o \"contract-linux-${ARCH}\" ."

  # Prepare build directory
  BUILD_PACK="${BUILD_DIR}/${APP_NAME}_${ARCH}"
  rm -rf "${BUILD_PACK}"
  mkdir -p "${BUILD_PACK}"

  # Copy fnpack template (only essential directories)
  cp -r fnpack/cmd "${BUILD_PACK}/"
  cp -r fnpack/config "${BUILD_PACK}/"
  cp -r fnpack/wizard "${BUILD_PACK}/"
  mkdir -p "${BUILD_PACK}/app"
  cp -r fnpack/app/ui "${BUILD_PACK}/app/"
  cp fnpack/ICON.PNG "${BUILD_PACK}/"
  cp fnpack/ICON_256.PNG "${BUILD_PACK}/"

  # Copy binary
  cp server/contract-linux-${ARCH} "${BUILD_PACK}/app/contract"
  chmod +x "${BUILD_PACK}/app/contract"
  rm server/contract-linux-${ARCH}

  # Copy frontend to app/ui
  cp -r server/static/dist/* "${BUILD_PACK}/app/ui/"

  # Generate manifest with correct platform
  if [ "$ARCH" = "amd64" ]; then
    FNOS_PLATFORM="x86"
  else
    FNOS_PLATFORM="arm"
  fi
  sed "s/^platform.*/platform              = ${FNOS_PLATFORM}/" fnpack/manifest > "${BUILD_PACK}/manifest"

  # Update version in manifest
  sed -i '' "s/^version.*/version               = ${VERSION#v}/" "${BUILD_PACK}/manifest" 2>/dev/null || \
  sed -i "s/^version.*/version               = ${VERSION#v}/" "${BUILD_PACK}/manifest"

  # Strip macOS metadata so it never ships inside the package
  find "${BUILD_PACK}" -name '.DS_Store' -delete

  # Build with fnpack
  cd "${BUILD_PACK}"
  fnpack build
  cd "$ROOT_DIR"

  # Move the built fpk to release directory（文件名带版本号；2026-09-21 约定
  # x86 平台的架构标写 x86 不写 amd64，与飞牛生态叫法一致，arm64 不变）
  if [ "$ARCH" = "amd64" ]; then
    ARCH_LABEL="x86"
  else
    ARCH_LABEL="$ARCH"
  fi
  if [ -f "${BUILD_PACK}/${FNOS_PKG_NAME}.fpk" ]; then
    mv "${BUILD_PACK}/${FNOS_PKG_NAME}.fpk" "${BUILD_DIR}/${FNOS_PKG_NAME}_${VERSION}_${ARCH_LABEL}.fpk"
  elif [ -f "${BUILD_PACK}/../${FNOS_PKG_NAME}.fpk" ]; then
    mv "${BUILD_PACK}/../${FNOS_PKG_NAME}.fpk" "${BUILD_DIR}/${FNOS_PKG_NAME}_${VERSION}_${ARCH_LABEL}.fpk"
  fi

  # Clean up
  rm -rf "${BUILD_PACK}"

  echo "Built ${FNOS_PKG_NAME}_${VERSION}_${ARCH_LABEL}.fpk"
done

# 截图、更新日志与 compose 随发行目录分发，版本目录自包含
# （build-all.sh 已放过一次也无妨，重复拷贝幂等）
if [ -d images/screenshots ]; then
  mkdir -p "${BUILD_DIR}/screenshots"
  cp images/screenshots/* "${BUILD_DIR}/screenshots/"
  (cd "${BUILD_DIR}" && zip -qr "screenshots-${VERSION}.zip" screenshots)
  echo "Copied screenshots and screenshots-${VERSION}.zip"
fi
[ -f CHANGELOG.md ] && cp CHANGELOG.md "${BUILD_DIR}/CHANGELOG.md"
sed "s|techfunways/contract:latest|techfunways/contract:${VERSION}|" deploy/docker-compose.yml > "${BUILD_DIR}/docker-compose.yml"
cp deploy/docker-compose.latest.yml "${BUILD_DIR}/docker-compose.latest.yml"
echo "Copied docker-compose.yml and docker-compose.latest.yml"

# Restore original manifest
mv fnpack/manifest.bak fnpack/manifest

# fnpack 构建用的是网关前缀版前端（VITE_BASE_PATH=/app/<包名>/），打完后恢复
# 标准构建，避免直连端口（make dev / 普通二进制）的静态资源被替换成前缀版。
echo "Restoring standard frontend build..."
VITE_FNOS_APP=false npm --prefix web run build
rm -rf server/static/dist
mkdir -p server/static/dist
cp -R web/dist/. server/static/dist/

echo "fnOS packages completed in ${BUILD_DIR}/"
