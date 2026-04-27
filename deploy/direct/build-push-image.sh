#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"

load_env_file_if_exists() {
  local file="$1"
  local line key value
  [[ -f "${file}" ]] || return 0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    # 跳过空行与注释
    [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || continue

    key="${line%%=*}"
    value="${line#*=}"

    # 若外部已显式设置同名变量，优先外部变量
    if [[ -z "${!key:-}" ]]; then
      export "${key}=${value}"
    fi
  done < "${file}"
}

load_env_file_if_exists "${ENV_FILE}"

IMAGE="${NEW_API_IMAGE:-}"
PLATFORM="${NEW_API_PLATFORM:-linux/amd64}"
BUILDER="${NEW_API_BUILDER:-new-api-buildx}"
PUSH_MODE="${NEW_API_PUSH_MODE:-push}" # push | load
APP_VERSION="${APP_VERSION:-}"

if [[ -z "${IMAGE}" ]]; then
  cat <<'EOF'
缺少 NEW_API_IMAGE。
请在 deploy/direct/.env 中设置，或在命令行中传入，例如：

NEW_API_IMAGE=ghcr.io/your-org/new-api:20260424 NEW_API_PLATFORM=linux/amd64 ./build-push-image.sh
EOF
  exit 1
fi

if [[ -z "${APP_VERSION}" ]]; then
  IMAGE_TAG="${IMAGE##*:}"
  if [[ -n "${IMAGE_TAG}" && "${IMAGE_TAG}" != "${IMAGE}" ]]; then
    APP_VERSION="${IMAGE_TAG}"
  else
    APP_VERSION="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || true)"
    APP_VERSION="${APP_VERSION:-v0.0.0-dev}"
  fi
fi

if ! docker buildx version >/dev/null 2>&1; then
  echo "未检测到 docker buildx，请先安装/启用 Docker Buildx。"
  exit 1
fi

if ! docker buildx inspect "${BUILDER}" >/dev/null 2>&1; then
  docker buildx create --name "${BUILDER}" --driver docker-container >/dev/null
fi

docker buildx use "${BUILDER}" >/dev/null
docker buildx inspect --bootstrap >/dev/null

BUILD_CMD=(
  docker buildx build
  --platform "${PLATFORM}"
  -t "${IMAGE}"
  --build-arg "APP_VERSION=${APP_VERSION}"
  -f "${REPO_ROOT}/Dockerfile"
  "${REPO_ROOT}"
)

case "${PUSH_MODE}" in
  push)
    BUILD_CMD+=(--push)
    ;;
  load)
    if [[ "${PLATFORM}" == *","* ]]; then
      echo "NEW_API_PUSH_MODE=load 时，NEW_API_PLATFORM 不能是多平台。"
      exit 1
    fi
    BUILD_CMD+=(--load)
    ;;
  *)
    echo "NEW_API_PUSH_MODE 仅支持 push 或 load，当前值：${PUSH_MODE}"
    exit 1
    ;;
esac

if [[ $# -gt 0 ]]; then
  BUILD_CMD+=("$@")
fi

echo "开始构建镜像：${IMAGE}"
echo "应用版本：${APP_VERSION}"
echo "目标平台：${PLATFORM}"
echo "推送模式：${PUSH_MODE}"
"${BUILD_CMD[@]}"
echo "完成。"
