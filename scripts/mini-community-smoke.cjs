/**
 * 文件作用：保留历史命令兼容入口，将旧页面验收统一委托给 TypeScript 界面回归脚本。
 * 当前入口：frontend/node_modules/.bin/tsx scripts/mini-ui-regression.ts。
 * 保留所有环境变量、命令行参数与终端输出，统一使用项目根目录和新页面路由。
 */
const { spawnSync } = require('node:child_process')
const path = require('node:path')
const projectRoot = path.resolve(__dirname, '..')
const result = spawnSync(
  path.join(projectRoot, 'frontend/node_modules/.bin/tsx'),
  [path.join(projectRoot, 'scripts/mini-ui-regression.ts'), ...process.argv.slice(2)],
  { cwd: projectRoot, env: process.env, stdio: 'inherit' }
)
if (result.error) console.error('统一界面回归脚本启动失败：' + result.error.message)
process.exit(result.status ?? 1)
