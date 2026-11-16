#!/usr/bin/env bash
# 文件作用：宝塔每两分钟调用的自动发布入口，串行同步main并发布Go后端与H5，不创建备份。
set -euo pipefail
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$PROJECT_DIR/scripts/auto_deploy.py" "$@"
