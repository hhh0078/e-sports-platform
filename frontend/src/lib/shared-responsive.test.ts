/** 文件作用：执行共享组件真实脚本，验证rpx图标、封面像素预留及可见图表实测尺寸的一致性。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'
import { executable } from './sfc-test'
import { designPixels, designRpx } from './design-units'
import { tradeChartGeometry, legacyCanvasColor } from './admin-chart'

/** 提取共享组件的生产脚本，测试不复制组件内部尺寸算法。 */
function script(name: string): string { const value = readFileSync(new URL('../components/' + name + '.vue', import.meta.url), 'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1]; assert.ok(value); return executable(value) }

test('数字与字符串数字图标共用rpx占位和官方字号，显式单位保持有效', () => {
  const props = reactive<{ size: number | string }>({ size: 23 })
  const context = vm.createContext({ computed, designRpx, defineProps: () => props, withDefaults: (value: unknown) => value })
  vm.runInContext(script('Icon') + '\nglobalThis.subject={edge}', context)
  for (const value of [23, '23', 10.5, '10.5']) { props.size = value; assert.equal(context.subject.edge.value, designRpx(Number(value))) }
  for (const value of ['32rpx', '1em', '100%']) { props.size = value; assert.equal(context.subject.edge.value, value) }
  const source = readFileSync(new URL('../components/Icon.vue', import.meta.url), 'utf8')
  assert.match(source, /<UniIcons[^>]*:size="edge"/)
})

test('服务卡片在320及390窗口用实际像素预留rpx文字与双角标，卸载移除监听', t => {
  const props = reactive({ service: { game_id: 'delta', provider_deposit_cents: 100, provider_club_deposit_cents: 100 }, interactive: false })
  const hooks: Record<string, () => void> = {}, resize = new Set<() => void>()
  let width = 390
  const context = vm.createContext({ computed, ref, designPixels, defineProps: () => props, withDefaults: (value: unknown) => value, onMounted: (hook: () => void) => { hooks.mounted = hook }, onShow: (hook: () => void) => { hooks.show = hook }, onBeforeUnmount: (hook: () => void) => { hooks.unmount = hook }, state: { games: [] }, uni: { getWindowInfo: () => ({ windowWidth: width }), onWindowResize: (hook: () => void) => resize.add(hook), offWindowResize: (hook: () => void) => resize.delete(hook) } })
  vm.runInContext(script('ServiceCard') + '\nglobalThis.subject={bottomReservation,topReservation}', context)
  hooks.mounted(); t.after(() => hooks.unmount())
  for (width of [320, 390]) {
    resize.forEach(hook => hook())
    assert.equal(context.subject.bottomReservation.value, 52 * width / 390)
    assert.equal(context.subject.topReservation.value, 44 * width / 390)
    props.service.provider_club_deposit_cents = 0
    assert.equal(context.subject.topReservation.value, 23 * width / 390)
    props.service.provider_deposit_cents = 0
    assert.equal(context.subject.topReservation.value, 0)
    props.service.provider_deposit_cents = 100; props.service.provider_club_deposit_cents = 100
  }
  hooks.unmount(); assert.equal(resize.size, 0)
})

test('交易图表使用真实rpx画布高度清理与计算，缺少测量或卸载后不绘制', async t => {
  const calls: unknown[][] = [], hooks: Record<string, () => void> = {}
  let rect: { width: number; height?: number } = { width: 288, height: 196 * 320 / 390 }
  const canvas = new Proxy({}, { get: (_, name) => (...args: unknown[]) => calls.push([name, ...args]) })
  const query = { in: () => query, select: () => query, boundingClientRect: (callback: (value: typeof rect) => void) => { callback(rect); return query }, exec: () => {} }
  const context = vm.createContext({ computed, nextTick, watch, tradeChartGeometry, legacyCanvasColor, defineProps: () => ({ days: [], color: '#8de8b6' }), withDefaults: (value: unknown) => value, getCurrentInstance: () => ({ uid: 1, proxy: {} }), onMounted: (hook: () => void) => { hooks.mounted = hook }, onBeforeUnmount: (hook: () => void) => { hooks.unmount = hook }, uni: { createSelectorQuery: () => query, createCanvasContext: () => canvas, onWindowResize: () => {}, offWindowResize: () => {} } })
  const scope = effectScope(); scope.run(() => vm.runInContext(script('TradeChart') + '\nglobalThis.subject={draw}', context)); t.after(() => scope.stop())
  await context.subject.draw()
  assert.deepEqual(calls.find(call => call[0] === 'clearRect'), ['clearRect', 0, 0, rect.width, rect.height])
  assert.ok((calls.find(call => call[0] === 'moveTo')?.[2] as number) < rect.height!)
  calls.length = 0; rect = { width: 288 }; await context.subject.draw(); assert.equal(calls.length, 0)
  hooks.unmount(); rect.height = 196; await context.subject.draw(); assert.equal(calls.length, 0)
})
