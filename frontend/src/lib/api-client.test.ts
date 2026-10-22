/** 文件作用：验证真实统一请求工具的认证头、HTTP错误、上传解析及业务查询编码。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { request, upload, queryString, errorMessage, errorStatus, errorCode } from '../api/client'
import { performOrderAction, listMessages } from '../api/orders.api'
import { verifyDevPhone } from '../api/auth.api'

/** 创建可控的 uni 平台传输接口，测试实际 API 层而不替代业务方法。 */
function setup() {
  const requests: UniApp.RequestOptions[] = [], uploads: UniApp.UploadFileOption[] = []
  const runtime = {
    getStorageSync: () => 'token-current',
    request: (options: UniApp.RequestOptions) => { requests.push(options) },
    uploadFile: (options: UniApp.UploadFileOption) => { uploads.push(options) },
  }
  const platform = globalThis as unknown as { uni: Partial<typeof uni> }
  const original = platform.uni
  platform.uni = runtime as unknown as Partial<typeof uni>
  return { requests, uploads, restore() { platform.uni = original } }
}

test('请求统一附带当前令牌并只解包业务data', async () => {
  const subject = setup()
  try {
    const pending = request<{ id: string }>('/me')
    assert.equal(subject.requests[0].header?.Authorization, 'Bearer token-current')
    assert.equal(subject.requests[0].timeout, 15000)
    subject.requests[0].success?.({ statusCode: 200, data: { data: { id: 'current-user' } }, header: {}, cookies: [] })
    assert.deepEqual(await pending, { id: 'current-user' })
  } finally { subject.restore() }
})

test('HTTP401保留状态和服务端消息，网络错误不伪装身份失效', async () => {
  const subject = setup()
  try {
    const pending = request('/me')
    subject.requests[0].success?.({ statusCode: 401, data: { error: '请重新登录' }, header: {}, cookies: [] })
    await assert.rejects(pending, (error: unknown) => errorStatus(error) === 401 && errorMessage(error) === '请重新登录')
    const offline = request('/games')
    subject.requests[1].fail?.({ errMsg: 'request:fail timeout' })
    await assert.rejects(offline, (error: unknown) => errorStatus(error) === undefined && /连接不上/.test(errorMessage(error)))
  } finally { subject.restore() }
})

test('上传错误响应不冒充有效媒体地址', async () => {
  const subject = setup()
  try {
    const pending = upload('/media/images', '/local/avatar.jpg')
    assert.equal(subject.uploads[0].header?.Authorization, 'Bearer token-current')
    subject.uploads[0].success?.({ statusCode: 413, data: JSON.stringify({ error: '图片过大' }) })
    await assert.rejects(pending, (error: unknown) => errorStatus(error) === 413 && errorMessage(error) === '图片过大')
  } finally { subject.restore() }
})

test('接单API保留当前挂靠选择，消息游标独立编码', async () => {
  const subject = setup()
  try {
    const claiming = performOrderAction('order/a', 'claim', '', 'club-B')
    assert.match(String(subject.requests[0].url), /orders\/order%2Fa\/actions$/)
    assert.deepEqual(subject.requests[0].data, { action: 'claim', note: '', club_id: 'club-B' })
    subject.requests[0].success?.({ statusCode: 200, data: { data: { id: 'order/a' } }, header: {}, cookies: [] })
    await claiming
    const messages = listMessages('order/a', { before_id: 'cursor&a', limit: 100 })
    assert.match(String(subject.requests[1].url), /before_id=cursor%26a&limit=100$/)
    subject.requests[1].success?.({ statusCode: 200, data: { data: [] }, header: {}, cookies: [] })
    assert.deepEqual(await messages, [])
  } finally { subject.restore() }
})

test('非法业务响应拒绝解析，微信原生errMsg保留可读诊断', async () => {
  const subject = setup()
  try {
    const pending = request('/games')
    subject.requests[0].success?.({ statusCode: 200, data: '<html>Bad Gateway</html>', header: {}, cookies: [] })
    await assert.rejects(pending, /服务器返回格式异常/)
    assert.equal(errorMessage({ errMsg: 'login:fail INVALID_LOGIN' }), 'login:fail INVALID_LOGIN')
    assert.equal(queryString({ q: '三角洲 & 王者', offset: 0, optional: undefined }), '?q=%E4%B8%89%E8%A7%92%E6%B4%B2%20%26%20%E7%8E%8B%E8%80%85&offset=0')
  } finally { subject.restore() }
})

test('开发手机号API发送与Go一致的phone_number字段', async () => {
  const subject = setup()
  try {
    const verifying = verifyDevPhone('13800000000')
    assert.deepEqual(subject.requests[0].data, { phone_number: '13800000000' })
    subject.requests[0].success?.({ statusCode: 200, data: { data: { phone_verified: true } }, header: {}, cookies: [] })
    assert.equal((await verifying).phone_verified, true)
  } finally { subject.restore() }
})

test('文字及图片审核拒绝保留稳定错误码和完整理由', async () => {
  const subject = setup()
  try {
    const reason = '内容审核未通过：包含现实违法交易信息，请修改后重新提交。'
    const text = request('/services', 'POST', { title: '示例' })
    subject.requests[0].success?.({ statusCode: 422, data: { error: reason, code: 'moderation_rejected' }, header: {}, cookies: [] })
    await assert.rejects(text, (error: unknown) => errorCode(error) === 'moderation_rejected' && errorMessage(error) === reason && errorStatus(error) === 422)
    const image = upload('/media/images', '/local/avatar.jpg')
    subject.uploads[0].success?.({ statusCode: 503, data: JSON.stringify({ error: '内容审核暂不可用，请保留草稿后重试', code: 'moderation_unavailable' }) })
    await assert.rejects(image, (error: unknown) => errorCode(error) === 'moderation_unavailable' && errorStatus(error) === 503)
    assert.equal(errorCode({ code: 42 }), undefined)
  } finally { subject.restore() }
})
