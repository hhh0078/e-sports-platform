/** 文件作用：验证金额显示与异常时间不会破坏交易界面。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { money, time, statusLabel, assetUrl, go, notify } from './format.ts'
import { back } from './navigation'

test('金额按分准确展示，保留有效小数', () => {
  assert.equal(money(0), '0')
  assert.equal(money(1), '0.01')
  assert.equal(money(1099), '10.99')
  assert.equal(money(1050), '10.5')
  assert.equal(money(1000000), '10000')
})

test('空值和无效时间安全显示，未知状态保留原值', () => {
  assert.equal(money(null), '0')
  assert.equal(time(null), '')
  assert.equal(time('not-a-date'), '')
  assert.equal(statusLabel('pending_payment'), '待支付')
  assert.equal(statusLabel('refunded'), '已退款')
  assert.equal(statusLabel('future_status'), 'future_status')
})

test('上传封面和内置游戏图片采用当前后端地址', () => {
  assert.equal(assetUrl('/api/media/cover123'), 'http://127.0.0.1:8090/api/media/cover123')
  assert.equal(assetUrl('/static/covers/delta-1.jpg'), 'http://127.0.0.1:8090/static/covers/delta-1.jpg')
  assert.equal(assetUrl(null), '')
})

test('运营轮播、透明公告和文章封面解析为当前API域名，包内与外部图片保持原地址', () => {
  for(const name of ['platform-intro-v1','newcomer-rules-v1'])assert.equal(assetUrl('/static/operations/'+name+'.png'),'http://127.0.0.1:8090/static/operations/'+name+'.png')
  for(const value of ['/static/icons/game.png','https://images.example.com/image.png','wxfile://local.png'])assert.equal(assetUrl(value),value)
})

test('主导航归一化保留筛选，同一主页面切换不重建，二级详情保留返回路径', () => {
  type Options = { url?: string; title?: string; fail?: () => void }
  const calls: Array<{ type: string; options: Options }> = []
  let pages: Array<{ route: string }> = []
  const platform = globalThis as unknown as { getCurrentPages?: () => Array<{ route: string }>; uni?: object }
  platform.getCurrentPages = () => pages
  platform.uni = { reLaunch: (options: Options) => calls.push({ type: 'reLaunch', options }), navigateTo: (options: Options) => calls.push({ type: 'navigateTo', options }), showToast: (options: Options) => calls.push({ type: 'toast', options }) }
  go('/pages/orders/index?role=provider&status=working')
  assert.equal(calls[0].type, 'reLaunch')
  assert.equal(calls[0].options.url, '/pages/main/index?tab=orders&role=provider&status=working')
  go('/pages/order/index?id=example')
  assert.equal(calls[1].type, 'navigateTo')
  calls[1].options.fail?.()
  assert.equal(calls[2].type, 'toast')
  pages = [{ route: 'pages/main/index' }]
  go('/pages/me/index')
  assert.equal(calls.length, 3)
  delete platform.uni
  delete platform.getCurrentPages
})

test('共享返回优先沿历史返回，无历史或返回失败时进入唯一主页面', () => {
  type Options = { url?: string; delta?: number; fail?: () => void }
  const calls: Array<{ type: string; options: Options }> = []
  let pages = [{ route: 'pages/main/index' }, { route: 'pages/services/detail' }]
  const platform = globalThis as unknown as { getCurrentPages?: () => Array<{ route: string }>; uni?: object }
  platform.getCurrentPages = () => pages
  platform.uni = { navigateBack: (options: Options) => calls.push({ type: 'back', options }), reLaunch: (options: Options) => calls.push({ type: 'reLaunch', options }), showToast: () => {} }
  try {
    back()
    assert.equal(calls[0].type, 'back')
    assert.equal(calls[0].options.delta, 1)
    calls[0].options.fail?.()
    assert.equal(calls[1].type, 'reLaunch')
    assert.equal(calls[1].options.url, '/pages/main/index?tab=player')
    pages = [{ route: 'pages/community/clubs/detail' }]
    back()
    assert.equal(calls[2].type, 'reLaunch')
    assert.equal(calls[2].options.url, '/pages/main/index?tab=player')
  } finally { delete platform.uni; delete platform.getCurrentPages }
})

test('审核拒绝和故障展示可停留的完整原因，普通提示保持短消息', () => {
  const modals: UniApp.ShowModalOptions[] = [], toasts: UniApp.ShowToastOptions[] = []
  const platform = globalThis as unknown as { uni?: object }, previous = platform.uni
  platform.uni = { showModal: (options: UniApp.ShowModalOptions) => modals.push(options), showToast: (options: UniApp.ShowToastOptions) => toasts.push(options) }
  try {
    const reason = '内容审核未通过：' + '需要修改的具体原因。'.repeat(20)
    notify({ code: 'moderation_rejected', message: reason })
    assert.equal(modals[0].title, '内容需修改')
    assert.equal(modals[0].content, reason)
    assert.equal(modals[0].showCancel, false)
    notify({ code: 'moderation_unavailable', message: '审核超时，请稍后重新提交，填写内容仍保留。' })
    assert.equal(modals[1].title, '审核暂不可用')
    assert.equal(toasts.length, 0)
    notify('保存成功')
    assert.equal(toasts[0].title, '保存成功')
  } finally { platform.uni = previous }
})
