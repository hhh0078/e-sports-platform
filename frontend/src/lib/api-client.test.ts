/** 文件作用：验证真实统一请求工具的认证头、HTTP错误、上传解析及业务查询编码。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { API_BASE, request, requestPublic, upload, download, queryString, errorMessage, errorStatus, errorCode } from '../api/client'
import { performOrderAction, listMessages } from '../api/orders.api'
import { verifyDevPhone } from '../api/auth.api'

/** 创建可控的 uni 平台传输接口，测试实际 API 层而不替代业务方法。 */
function setup() {
  const requests: UniApp.RequestOptions[] = [], uploads: UniApp.UploadFileOption[] = [], downloads: UniApp.DownloadFileOption[] = [], notices: UniApp.ShowToastOptions[] = [], removed: string[] = []
  let currentToken = 'token-current'
  const runtime = {
    getStorageSync: () => currentToken,
    showToast: (options: UniApp.ShowToastOptions) => { notices.push(options) },
    downloadFile: (options: UniApp.DownloadFileOption) => { downloads.push(options) },
    getFileSystemManager: () => ({ unlink: ({ filePath }: { filePath: string }) => { removed.push(filePath) } }),
    request: (options: UniApp.RequestOptions) => { requests.push(options) },
    uploadFile: (options: UniApp.UploadFileOption) => { uploads.push(options) },
  }
  const platform = globalThis as unknown as { uni: Partial<typeof uni> }
  const original = platform.uni
  platform.uni = runtime as unknown as Partial<typeof uni>
  return { requests, uploads, downloads, notices, removed, setToken(value: string) { currentToken = value }, restore() { platform.uni = original } }
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

test('朋友圈公共读取即使本机有会话也不携带Authorization且只使用GET',async()=>{
  const subject=setup()
  try{const pending=requestPublic<{id:string}>('/shares/public-id');assert.equal(subject.requests[0].header?.Authorization,undefined);assert.equal(subject.requests[0].method,'GET');subject.requests[0].success?.({statusCode:200,data:{data:{id:'public-id'}},header:{},cookies:[]});assert.deepEqual(await pending,{id:'public-id'})}finally{subject.restore()}
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


/** 等待当前队列的零延时轻提示执行完毕后才归还全局uni接口。 */
async function drainNotices(): Promise<void> { await new Promise(resolve => setTimeout(resolve, 0)) }

test('写入保留原data，AI和人工回执分别提示，图片使用上传文案', async () => {
  const subject = setup()
  try {
    const data = { id: 'service-A', state: 'active' }
    const writing = request<typeof data>('/services', 'POST', { title: '服务' })
    subject.requests[0].success?.({ statusCode: 201, data: { data, moderation: { status: 'ai_pending', pending_count: 3, estimated_seconds: 20 } }, header: {}, cookies: [] })
    assert.equal(await writing, data)
    assert.equal(subject.notices.length, 0, '审核轻提示不阻塞成功回调')
    await drainNotices()
    assert.match(String(subject.notices[0].title), /已保存.*图文审核中.*20秒/)
    const editing = request('/me/profile', 'PATCH', { nickname: '队友' })
    subject.requests[1].success?.({ statusCode: 200, data: { data: { id: 'user-A' }, moderation: { status: 'manual_pending', pending_count: 1 } }, header: {}, cookies: [] })
    await editing; await drainNotices()
    assert.equal(subject.notices[1].title, '已保存，等待人工审核')
    assert.equal(String(subject.notices[1].title).includes('20秒'), false)
    const uploading = upload<{ url: string }>('/media/images', '/tmp/avatar.jpg')
    subject.uploads[0].success?.({ statusCode: 200, data: JSON.stringify({ data: { url: '/api/media/image-A' }, moderation: { status: 'ai_pending', pending_count: 1 } }) })
    assert.deepEqual(await uploading, { url: '/api/media/image-A' }); await drainNotices()
    assert.match(String(subject.notices[2].title), /^图片已上传.*图文审核中/)
  } finally { await drainNotices(); subject.restore() }
})

test('GET、无回执、无任务、匿名和失败响应不产生审核提示', async () => {
  const subject = setup(), receipt = { status: 'ai_pending', pending_count: 1 }
  try {
    const reading = request('/services'); subject.requests[0].success?.({ statusCode: 200, data: { data: [], moderation: receipt }, header: {}, cookies: [] }); await reading
    const ordinary = request('/services', 'POST', {}); subject.requests[1].success?.({ statusCode: 200, data: { data: {} }, header: {}, cookies: [] }); await ordinary
    const empty = request('/services', 'PATCH', {}); subject.requests[2].success?.({ statusCode: 200, data: { data: {}, moderation: { ...receipt, pending_count: 0 } }, header: {}, cookies: [] }); await empty
    const failed = request('/services', 'POST', {}); subject.requests[3].success?.({ statusCode: 400, data: { error: '格式无效', moderation: receipt }, header: {}, cookies: [] }); await assert.rejects(failed)
    const anonymous = requestPublic('/bootstrap'); subject.requests[4].success?.({ statusCode: 200, data: { data: {}, moderation: receipt }, header: {}, cookies: [] }); await anonymous
    subject.setToken('')
    const unauthenticated = request('/services', 'POST', {}); subject.requests[5].success?.({ statusCode: 200, data: { data: {}, moderation: receipt }, header: {}, cookies: [] }); await unauthenticated
    const badUpload = upload('/media/images', '/tmp/avatar.png'); subject.uploads[0].success?.({ statusCode: 503, data: JSON.stringify({ error: '保存失败', moderation: receipt }) }); await assert.rejects(badUpload)
    await drainNotices(); assert.equal(subject.notices.length, 0)
  } finally { await drainNotices(); subject.restore() }
})

test('响应前换号与已排队提示展示前换号均丢弃旧身份的审核提示', async () => {
  const subject = setup(), receipt = { status: 'manual_pending', pending_count: 1 }
  try {
    const old = request('/me/profile', 'PATCH', {}); subject.setToken('token-new')
    subject.requests[0].success?.({ statusCode: 200, data: { data: { id: 'old-user' }, moderation: receipt }, header: {}, cookies: [] }); await old; await drainNotices(); assert.equal(subject.notices.length, 0)
    const queued = request('/me/profile', 'PATCH', {}); subject.requests[1].success?.({ statusCode: 200, data: { data: {}, moderation: receipt }, header: {}, cookies: [] }); await queued
    subject.setToken('token-third'); await drainNotices(); assert.equal(subject.notices.length, 0)
    const image = upload('/media/images', '/tmp/avatar.png', 'token-third'); subject.setToken('token-fourth')
    subject.uploads[0].success?.({ statusCode: 200, data: JSON.stringify({ data: { url: '/api/media/old' }, moderation: receipt }) }); await image; await drainNotices(); assert.equal(subject.notices.length, 0)
  } finally { await drainNotices(); subject.restore() }
})

test('受限图片下载只向固定同源路径发送Bearer请求头并返回200临时文件', async () => {
  const subject = setup()
  try {
    const pending = download('/admin/moderation/task-A/image')
    assert.equal(subject.downloads[0].url, API_BASE + '/admin/moderation/task-A/image')
    assert.equal(subject.downloads[0].header?.Authorization, 'Bearer token-current')
    assert.equal(subject.downloads[0].timeout, 15000)
    assert.equal(subject.downloads[0].url.includes('token-current'), false)
    subject.downloads[0].success?.({ statusCode: 200, tempFilePath: '/tmp/protected.png' })
    assert.equal(await pending, '/tmp/protected.png'); assert.deepEqual(subject.removed, [])
    for (const invalid of ['https://external.invalid/image', '//external.invalid/image', '/image\r\nInjected: value']) await assert.rejects(download(invalid), /接口地址无效/)
    assert.equal(subject.downloads.length, 1)
  } finally { subject.restore() }
})

test('未登录、HTTP失败、空路径和网络失败均不冒充有效审核图片', async () => {
  const subject = setup()
  try {
    subject.setToken(''); await assert.rejects(download('/admin/moderation/task-A/image'), error => errorStatus(error) === 401); assert.equal(subject.downloads.length, 0)
    subject.setToken('token-current')
    const forbidden = download('/admin/moderation/task-A/image'); subject.downloads[0].success?.({ statusCode: 403, tempFilePath: '/tmp/forbidden-response' }); await assert.rejects(forbidden, error => errorStatus(error) === 403); assert.deepEqual(subject.removed, ['/tmp/forbidden-response'])
    const empty = download('/admin/moderation/task-A/image'); subject.downloads[1].success?.({ statusCode: 200, tempFilePath: '' }); await assert.rejects(empty, /暂时无法读取审核图片/)
    const offline = download('/admin/moderation/task-A/image'); subject.downloads[2].fail?.({ errMsg: 'downloadFile:fail timeout' }); await assert.rejects(offline, /下载失败/)
  } finally { subject.restore() }
})

test('下载期间换号会删除旧临时文件并以取消错误拒绝交付', async () => {
  const subject = setup()
  try {
    const pending = download('/admin/moderation/task-A/image'); subject.setToken('token-another')
    subject.downloads[0].success?.({ statusCode: 200, tempFilePath: '/tmp/old-account.png' })
    await assert.rejects(pending, error => typeof error === 'object' && error !== null && 'cancelled' in error && error.cancelled === true)
    assert.deepEqual(subject.removed, ['/tmp/old-account.png'])
  } finally { subject.restore() }
})
