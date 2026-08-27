#!/usr/bin/env bash
# usage: bash build_benzhi_docker.sh <镜像名> <平台>
# 例：bash build_benzhi_docker.sh my-project linux/amd64
set -euo pipefail

IMAGE_NAME="${1:-my-project}"
PLATFORM="${2:-linux/amd64}"

docker buildx build --platform "${PLATFORM}" -f benzhi.Dockerfile -t "${IMAGE_NAME}" .
echo "built ${IMAGE_NAME} for ${PLATFORM}"
