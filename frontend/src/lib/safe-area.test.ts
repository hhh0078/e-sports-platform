/** 文件作用：按uni-app真实条件编译分支执行安全区工具，验证H5零值、胶囊避让和原生异常回退。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { executable } from './sfc-test'
import type { SafeAreaMetrics } from './safe-area'

/** 只替换原生API和平台编译选择，测量公式仍来自真实业务源文件。 */
function measure(platform: 'H5' | 'MP-WEIXIN', window: unknown, capsule: unknown, throws = false): SafeAreaMetrics {
  const source = readFileSync(new URL('./safe-area.ts', import.meta.url), 'utf8').replace(/\/\/ #ifdef (H5|MP-WEIXIN)\n([\s\S]*?)\/\/ #endif/g, (_, target: string, code: string) => target === platform ? code : '')
  const context = vm.createContext({ uni: { getWindowInfo() { if (throws) throw new Error('原生异常'); return window }, getMenuButtonBoundingClientRect() { if (throws || platform === 'H5') throw new Error('不应访问胶囊'); return capsule } } })
  vm.runInContext(executable(source) + '\nglobalThis.metrics=readSafeArea()', context)
  return context.metrics as SafeAreaMetrics
}

test('H5安全区全部为零且不依赖原生接口', () => {
  const value = measure('H5', null, null, true)
  assert.deepEqual({ ...value }, { statusBarHeight: 0, capsuleSpace: 0, capsuleRightInset: 0, floatingTop: 0, contentTop: 0 })
})

test('微信读取独立水平胶囊避让宽度，悬浮按钮对齐胶囊顶部', () => {
  const value = measure('MP-WEIXIN', { statusBarHeight: 24, windowWidth: 390 }, { top: 28, height: 32, left: 292, right: 379 })
  assert.deepEqual({ ...value }, { statusBarHeight: 24, capsuleSpace: 40, capsuleRightInset: 110, floatingTop: 28, contentTop: 64 })
})

test('微信接口异常、非有限值及无效胶囊都有安全回退', () => {
  assert.deepEqual({ ...measure('MP-WEIXIN', null, null, true) }, { statusBarHeight: 20, capsuleSpace: 48, capsuleRightInset: 108, floatingTop: 28, contentTop: 68 })
  const value = measure('MP-WEIXIN', { statusBarHeight: Number.NaN, windowWidth: Number.POSITIVE_INFINITY }, { top: 0, height: 0, left: -1, right: 99999 })
  assert.equal(value.statusBarHeight, 20)
  assert.equal(value.floatingTop, 28)
  assert.equal(value.capsuleRightInset, 108)
  const validWindow = measure('MP-WEIXIN', { statusBarHeight: 44, windowWidth: 430 }, { top: 10, height: 32, left: 300, right: 420 })
  assert.equal(validWindow.statusBarHeight, 44)
  assert.equal(validWindow.floatingTop, 52)
  assert.equal(validWindow.contentTop, 92)
})
