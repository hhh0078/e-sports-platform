#!/usr/bin/env bash
# 文件作用：只停止启动脚本记录且仍属于本项目的进程，不停止数据库、Redis 或其它应用。
set -euo pipefail
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DRY_RUN="${1:-}"
[[ -z "$DRY_RUN" || "$DRY_RUN" == --dry-run ]] || { echo '用法：scripts/dev-stop.sh [--dry-run]' >&2; exit 1; }
for label in h5 api; do
  pid_file="$PROJECT_DIR/.run/$label.pid"
  [[ -f "$pid_file" ]] || continue
  pid="$(sed -n '1p' "$pid_file")"
  expected_start="$(sed -n '2p' "$pid_file")"
  [[ "$pid" =~ ^[0-9]+$ ]] || { echo "忽略无效的 $label PID 记录。" >&2; continue; }
  actual_start="$(ps -p "$pid" -o lstart= 2>/dev/null | xargs || true)"
  if [[ -z "$actual_start" ]]; then
    [[ "$DRY_RUN" == --dry-run ]] || rm -f "$pid_file"
    continue
  fi
  process_dir="$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p')"
  command_line="$(ps -p "$pid" -o command=)"
  belongs=false
  if [[ "$label" == api && ( "$process_dir" == "$PROJECT_DIR" || "$process_dir" == "$PROJECT_DIR/backend" ) && "$command_line" == *jijie-api* ]] ||
     [[ "$label" == h5 && "$process_dir" == "$PROJECT_DIR/frontend" && "$command_line" == *"$PROJECT_DIR/frontend/node_modules/"* && "$command_line" == *uni* ]]; then belongs=true; fi
  if [[ "$actual_start" != "$expected_start" || "$belongs" != true ]]; then
    echo "拒绝停止 ${label}（PID ${pid}）：启动时间或项目归属不匹配。" >&2
    continue
  fi
  if [[ "$DRY_RUN" == --dry-run ]]; then
    echo "已核验：可安全停止本项目 ${label}（PID ${pid}）；本次未发送信号。"
    continue
  fi
  kill -TERM "$pid"
  stopped=false
  for attempt in {1..30}; do
    if ! kill -0 "$pid" 2>/dev/null; then stopped=true; break; fi
    sleep 0.2
  done
  if [[ "$stopped" == true ]]; then
    rm -f "$pid_file"
    echo "已停止本项目 ${label}。"
  else
    echo "$label 仍在退出中，保留 PID 记录；未强制终止。" >&2
  fi
done
