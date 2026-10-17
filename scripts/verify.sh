#!/usr/bin/env bash
# 文件作用：运行真实 PG/Redis 集成测试、Go 静态检查、前端测试及双端构建，不重置演示数据。
set -euo pipefail
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"
if [[ -f backend/.env ]]; then
  set -a
  source backend/.env
  set +a
fi
export DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@127.0.0.1:5432/jijie_dev?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:56379}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
python3 - <<'PY'
"""阻止验收脚本误连非本地或非本项目数据库。"""
import os
from urllib.parse import urlparse
database = urlparse(os.environ['DATABASE_URL'])
if database.path != '/jijie_dev' or database.hostname not in ('localhost', '127.0.0.1'):
    raise SystemExit('验收仅允许本地 jijie_dev 数据库；测试在其中创建并清理独立 schema。')
PY
bash -n scripts/dev-start.sh scripts/dev-stop.sh scripts/verify.sh
(cd backend && TEST_INTEGRATION=true go test -race -count=1 -v ./... && go vet ./...)
(cd frontend && npm test && npm run build:h5 && npm run build:mp-weixin)
echo '自动验收完成：后端集成测试、竞态检查、静态检查、前端测试与 H5/微信构建均通过。'
echo '真实微信登录/支付及真机验收需要实际 AppID、商户和 HTTPS 域名，见 docs/WECHAT_SETUP.md。'
