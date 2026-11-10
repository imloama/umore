#!/usr/bin/env bash
# 启动单个后端进程：同时提供前端页面（./web）与 /api、/uploads。
# 数据与配置都相对于 backend 目录，先 cd 过去再启动，别在别的目录裸跑。
set -euo pipefail

cd "$(dirname "$0")/backend"
exec ./umore
