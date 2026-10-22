#!/usr/bin/env bash
# 文件作用：启动或复用本目录的本地 API 与 H5，验证数据库和 Redis，不操作其它项目。
set -euo pipefail
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RUN_DIR="$PROJECT_DIR/.run"
mkdir -p "$RUN_DIR" "$PROJECT_DIR/backend/bin"
cd "$PROJECT_DIR"

if [[ -f backend/.env ]]; then
  set -a
  source backend/.env
  set +a
fi
export DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@127.0.0.1:5432/jijie_dev?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:56379}"
export REDIS_PREFIX="${REDIS_PREFIX:-jijie:}"
export ADDR="${ADDR:-127.0.0.1:8090}"
export APP_ENV="${APP_ENV:-development}"
export DEV_MODE="${DEV_MODE:-true}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
if [[ "$APP_ENV" != development || "$DEV_MODE" != true || "$ADDR" != 127.0.0.1:8090 ]]; then
  echo '此脚本仅启动本地 development / DEV_MODE=true / 127.0.0.1:8090。正式服务请单独配置。' >&2
  exit 1
fi
for command_name in go node npm python3 curl lsof; do
  command -v "$command_name" >/dev/null || { echo "缺少命令：$command_name" >&2; exit 1; }
done
if [[ ! -f frontend/node_modules/@dcloudio/vite-plugin-uni/bin/uni.js ]]; then
  echo '前端依赖尚未安装。先确认 npm config get registry 为国内镜像，再在 frontend 执行 npm ci。' >&2
  exit 1
fi
python3 - <<'PY'
"""检查专用开发库名称与依赖端口，连接失败不修改数据库或启动其它容器。"""
import os
import socket
from urllib.parse import urlparse

database = urlparse(os.environ['DATABASE_URL'])
if database.path != '/jijie_dev' or database.hostname not in ('localhost', '127.0.0.1'):
    raise SystemExit('本地启动仅允许 localhost/127.0.0.1 的 jijie_dev 数据库；未触碰其它数据库。')
redis_host, redis_port = os.environ['REDIS_ADDR'].rsplit(':', 1)
for name, host, port in [('PostgreSQL', database.hostname, database.port or 5432), ('Redis', redis_host, int(redis_port))]:
    try:
        with socket.create_connection((host, port), timeout=2):
            pass
    except OSError:
        raise SystemExit(f'{name} 未就绪（{host}:{port}）。请启动现有 Docker 服务；本脚本不会更改其它容器。')
PY

# record_process 记录 PID 和启动时间，供停止脚本防止误杀已复用的 PID。
record_process() {
  local label="$1" pid="$2" started
  started="$(ps -p "$pid" -o lstart= | xargs)"
  [[ -n "$started" ]] || { echo "$label 进程已退出，请查看 .run 日志。" >&2; exit 1; }
  printf '%s\n%s\n' "$pid" "$started" > "$RUN_DIR/$label.pid"
}

# existing_process 只复用工作目录和命令均属于当前仓库的监听进程。
existing_process() {
  local port="$1" label="$2" pid command_line process_dir
  pid="$(lsof -nP -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | head -n 1 || true)"
  [[ -n "$pid" ]] || return 1
  command_line="$(ps -p "$pid" -o command=)"
  process_dir="$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p')"
  if [[ "$label" == api && ( "$process_dir" == "$PROJECT_DIR" || "$process_dir" == "$PROJECT_DIR/backend" ) && "$command_line" == *jijie-api* ]] ||
     [[ "$label" == h5 && "$process_dir" == "$PROJECT_DIR/frontend" && "$command_line" == *"$PROJECT_DIR/frontend/node_modules/"* && "$command_line" == *uni* ]]; then
    record_process "$label" "$pid"
    echo "复用本项目 ${label}（PID ${pid}）。"
    return 0
  fi
  echo "端口 $port 被其它进程占用（PID ${pid}），未停止或修改该进程。" >&2
  exit 1
}

if ! existing_process 8090 api; then
  (cd backend && go build -o bin/jijie-api ./cmd/server)
  api_pid="$(python3 - "$PROJECT_DIR" <<'PY'
"""创建脱离当前终端会话的 API 进程，日志和 stdin 均不依赖调用终端。"""
import pathlib
import subprocess
import sys
project = pathlib.Path(sys.argv[1])
with (project / '.run/api.log').open('ab') as log:
    process = subprocess.Popen([str(project / 'backend/bin/jijie-api')], cwd=project, stdin=subprocess.DEVNULL, stdout=log, stderr=log, start_new_session=True)
print(process.pid)
PY
)"
  record_process api "$api_pid"
fi
if ! existing_process 5173 h5; then
  h5_pid="$(python3 - "$PROJECT_DIR" <<'PY'
"""直接启动 uni CLI 的 Node 进程，避免 npm 包装层留下难以管理的子进程。"""
import pathlib
import subprocess
import sys
project = pathlib.Path(sys.argv[1])
with (project / '.run/h5.log').open('ab') as log:
    process = subprocess.Popen(['node', str(project / 'frontend/node_modules/@dcloudio/vite-plugin-uni/bin/uni.js'), '--host', '0.0.0.0', '--port', '5173'], cwd=project / 'frontend', stdin=subprocess.DEVNULL, stdout=log, stderr=log, start_new_session=True)
print(process.pid)
PY
)"
  record_process h5 "$h5_pid"
fi
for attempt in {1..40}; do
  if curl --fail --silent --max-time 3 http://127.0.0.1:8090/health > "$RUN_DIR/health.json" && curl --fail --silent --max-time 3 http://127.0.0.1:5173/ > /dev/null; then
    echo '开机电竞已就绪：http://127.0.0.1:5173/'
    echo 'API：http://127.0.0.1:8090/health；日志位于 .run/api.log 与 .run/h5.log。'
    exit 0
  fi
  sleep 0.5
done
echo '启动未通过健康检查，请查看 .run/api.log 和 .run/h5.log。' >&2
exit 1
