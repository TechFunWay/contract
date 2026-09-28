#!/bin/bash
set -e

# 全平台编译打包——产物格式对齐家族 release/<VERSION>/ 发行目录规范：
#   techfunway-contract-<VERSION>-{linux,darwin}-{amd64,arm64}.tar.gz
#   techfunway-contract-<VERSION>-windows-amd64.zip
#   techfunway-contract_<VERSION>_{amd64,arm64}.fpk（由 build-fnpack.sh 产出）
#   docker-compose.yml（镜像 tag 钉死为当前版本）/ docker-compose.latest.yml
#   CHANGELOG.md / images/screenshots → screenshots/ + screenshots-<VERSION>.zip

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="contract"
PACKAGE_PREFIX="techfunway-contract"
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=${APP_NAME}"

echo "Building frontend..."
cd web && npm ci && npm run build && cd ..

echo "Copying frontend..."
rm -rf server/static/dist
cp -r web/dist server/static/dist

BUILD_DIR="release/${VERSION}"
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"
  OUTPUT_NAME="${PACKAGE_PREFIX}-${VERSION}-${GOOS}-${GOARCH}"

  echo "Building ${OUTPUT_NAME}..."

  if [ "$GOOS" = "linux" ]; then
    # Use Docker for Linux targets (CGO cross-compilation)
    docker run --rm \
      -v "${ROOT_DIR}/server:/src" \
      -v "go-build-cache:/root/.cache/go-build" \
      -v "go-mod-cache:/go/pkg/mod" \
      -w /src \
      --platform "linux/${GOARCH}" \
      -e "LDFLAGS=${LDFLAGS}" \
      -e "GOARCH=${GOARCH}" \
      -e "APP_NAME=${APP_NAME}" \
      golang:1.26-alpine \
      sh -c 'apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go build -ldflags "$LDFLAGS -extldflags -static" -o "${APP_NAME}-linux-${GOARCH}" .'
    cd "$ROOT_DIR"
    mv "server/${APP_NAME}-linux-${GOARCH}" "server/${OUTPUT_NAME}"
  elif [ "$GOOS" = "windows" ]; then
    # Windows 也必须开 CGO：SQLite 驱动是 mattn/go-sqlite3，没有纯 Go 实现。交叉编译
    # 时 CGO_ENABLED 默认是 0，编出来的是官方 stub——程序能启动，一碰数据库就报
    # "go-sqlite3 requires cgo to work"。改用 mingw-w64 编真正的 CGO 二进制。
    if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
      echo "错误：未找到 x86_64-w64-mingw32-gcc，请先安装：brew install mingw-w64"
      exit 1
    fi
    cd server
    CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH CC=x86_64-w64-mingw32-gcc \
      go build -ldflags "${LDFLAGS} -extldflags -static" -o ${OUTPUT_NAME}.exe .
    cd "$ROOT_DIR"
  else
    cd server
    CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "${LDFLAGS}" -o ${OUTPUT_NAME} .
    cd "$ROOT_DIR"
  fi

  mkdir -p ${BUILD_DIR}/${OUTPUT_NAME}

  # 包内可执行文件统一叫 contract / contract.exe，前端静态资源放 www/
  if [ "$GOOS" = "windows" ]; then
    cp server/${OUTPUT_NAME}.exe ${BUILD_DIR}/${OUTPUT_NAME}/contract.exe
    rm server/${OUTPUT_NAME}.exe
  else
    cp server/${OUTPUT_NAME} ${BUILD_DIR}/${OUTPUT_NAME}/contract
    rm server/${OUTPUT_NAME}
  fi

  cp -r server/static/dist ${BUILD_DIR}/${OUTPUT_NAME}/www

  cd "${ROOT_DIR}/${BUILD_DIR}"
  if [ "$GOOS" = "windows" ]; then
    zip -qr ${OUTPUT_NAME}.zip ${OUTPUT_NAME}
  else
    COPYFILE_DISABLE=1 tar czf ${OUTPUT_NAME}.tar.gz ${OUTPUT_NAME}
  fi
  cd "$ROOT_DIR"

  rm -rf ${BUILD_DIR}/${OUTPUT_NAME}

  echo "Built ${OUTPUT_NAME}"
done

# 截图与更新日志随发行目录分发，版本目录自包含
if [ -d images/screenshots ]; then
  mkdir -p "${BUILD_DIR}/screenshots"
  cp images/screenshots/* "${BUILD_DIR}/screenshots/"
  (cd "${BUILD_DIR}" && zip -qr "screenshots-${VERSION}.zip" screenshots)
  echo "Copied screenshots and screenshots-${VERSION}.zip"
fi
[ -f CHANGELOG.md ] && cp CHANGELOG.md "${BUILD_DIR}/CHANGELOG.md"

# 部署 compose 文件随发行产物分发；版本文件中的镜像 tag 替换为当前版本，
# 拷贝到目标主机后 docker compose up -d 即可运行；latest 文件原样分发。
sed "s|techfunways/contract:latest|techfunways/contract:${VERSION}|" deploy/docker-compose.yml > "${BUILD_DIR}/docker-compose.yml"
cp deploy/docker-compose.latest.yml "${BUILD_DIR}/docker-compose.latest.yml"
echo "Copied docker-compose.yml and docker-compose.latest.yml"

echo "All builds completed in ${BUILD_DIR}/"
