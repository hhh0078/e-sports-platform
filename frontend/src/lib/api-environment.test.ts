/** 文件作用：验证开发、正式与显式本地模拟构建的 API 环境边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveAPIBase } from './api-environment'

test('开发默认本机，正式默认项目 HTTPS 域名', () => {
  assert.equal(resolveAPIBase(false), 'http://127.0.0.1:8090/api')
  assert.equal(resolveAPIBase(false, 'http://127.0.0.1:18091/api'), 'http://127.0.0.1:18091/api')
  assert.equal(resolveAPIBase(true), 'https://game.ai.58it.cn/api')
  assert.equal(resolveAPIBase(true, 'https://game.ai.58it.cn/api/'), 'https://game.ai.58it.cn/api')
})

test('正式构建拒绝本机、IP、HTTP及含秘密或查询的地址', () => {
  for (const value of ['http://game.ai.58it.cn/api', 'http://127.0.0.1:18091/api', 'https://127.0.0.1/api', 'https://localhost/api', 'https://a.localhost/api', 'https://[::1]/api', 'https://user:secret@game.ai.58it.cn/api', 'https://game.ai.58it.cn/api?token=x', 'https://game.ai.58it.cn/api#x', 'https://game.ai.58it.cn/']) assert.throws(() => resolveAPIBase(true, value))
})
