#!/usr/bin/env bash
# 文件作用：校验正式小程序 API 域名与 AppID，再生成不含本地测试入口的发布构建。
set -euo pipefail
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
export VITE_API_BASE="${VITE_API_BASE:-https://game.ai.58it.cn/api}"
python3 - "$PROJECT_DIR" <<'PY'
"""确认正式构建只连接 HTTPS 域名，并拒绝游客 AppID。"""
import json, os, pathlib, sys
from urllib.parse import urlparse
url = urlparse(os.environ['VITE_API_BASE'])
if url.scheme != 'https' or not url.hostname or url.hostname in ('localhost', '127.0.0.1') or url.path != '/api':
    raise SystemExit('发布构建需要 https://正式域名/api')
config = json.loads((pathlib.Path(sys.argv[1]) / 'frontend/src/manifest.json').read_text())
appid = config['mp-weixin']['appid']
if not appid.startswith('wx') or len(appid) != 18:
    raise SystemExit('请在 frontend/src/manifest.json 配置实际微信 AppID')
print('正式 API：' + os.environ['VITE_API_BASE'])
print('小程序 AppID：' + appid)
PY
cd "$PROJECT_DIR/frontend"
npm run build:mp-weixin
node - "$PROJECT_DIR" <<'JS'
// 正式产物启用微信合法域名校验，避免沿用本地调试的关闭状态。
const fs = require('node:fs')
const path = require('node:path')
const outputDir = path.resolve(process.argv[2], 'frontend', process.env.UNI_OUTPUT_DIR || 'dist/build/mp-weixin')
const configPath = path.join(outputDir, 'project.config.json')
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'))
config.setting = { ...config.setting, urlCheck: true }
fs.writeFileSync(configPath, JSON.stringify(config, null, 2) + '\n')
// 发布包不得携带本地开发者配置，避免其 urlCheck:false 覆盖正式设置。
fs.rmSync(path.join(path.dirname(configPath), 'project.private.config.json'), { force: true })
if (!config.appid || config.setting.urlCheck !== true) throw new Error('正式小程序配置校验失败')
console.log('正式产物已启用合法域名校验。')
console.log('发布目录：' + outputDir)
JS
echo '请导入上述发布目录，核验合法域名后上传。'
