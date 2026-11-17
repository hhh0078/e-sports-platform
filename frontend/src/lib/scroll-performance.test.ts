/** 文件作用：验证共享窗口监听、常驻导航无滚动回写和图文图片占位的性能边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { nextTick, readonly, ref } from 'vue'
import { executable } from './sfc-test'
import { communityImageStyle } from './community-image'

test('一百张卡片共享一个窗口监听，同轮挂载和显示只读一次原生宽度', async () => {
  const mounted: Array<() => void> = [], shown: Array<() => void> = [], unmounted: Array<() => void> = []
  const listeners = new Set<(event?: { size?: { windowWidth?: number } }) => void>()
  let reads = 0, width = 320
  const context = vm.createContext({ nextTick, readonly, ref,
    onMounted: (callback: () => void) => mounted.push(callback), onShow: (callback: () => void) => shown.push(callback), onBeforeUnmount: (callback: () => void) => unmounted.push(callback),
    uni: { getWindowInfo: () => { reads++; return { windowWidth: width } }, onWindowResize: (callback: (event?: { size?: { windowWidth?: number } }) => void) => listeners.add(callback), offWindowResize: (callback: (event?: { size?: { windowWidth?: number } }) => void) => listeners.delete(callback) },
  })
  vm.runInContext(executable(readFileSync(new URL('./viewport.ts', import.meta.url), 'utf8')) + '\nglobalThis.useViewportWidth=useViewportWidth', context)
  const consumers = Array.from({ length: 100 }, () => context.useViewportWidth())
  mounted.forEach(callback => callback()); shown.forEach(callback => callback()); await nextTick()
  assert.equal(listeners.size, 1); assert.equal(reads, 1)
  for (const consumer of consumers) assert.equal(consumer.value, 320)
  listeners.forEach(callback => callback({ size: { windowWidth: 390 } }))
  for (const consumer of consumers) assert.equal(consumer.value, 390)
  assert.equal(reads, 1, '真实尺寸事件不能再逐卡同步读取窗口')
  width = 375; shown.forEach(callback => callback()); await nextTick()
  assert.equal(reads, 2); assert.equal(consumers[0].value, 375)
  unmounted.slice(0, 99).forEach(callback => callback()); assert.equal(listeners.size, 1)
  unmounted[99](); assert.equal(listeners.size, 0)
  unmounted.forEach(callback => callback()); assert.equal(listeners.size, 0)
  shown.forEach(callback => callback()); await nextTick(); assert.equal(reads, 2)
})

test('四个真实tab页共用一个滚动框架，筛选和滚动由页面缓存保留',()=>{
 const frame=readFileSync(new URL('../components/MainPage.vue',import.meta.url),'utf8')
 assert.equal((frame.match(/<scroll-view\b/g)||[]).length,1)
 assert.doesNotMatch(frame,/:scroll-top=|@scroll=|main-scroll-inactive/)
 const config=JSON.parse(readFileSync(new URL('../pages.json',import.meta.url),'utf8'))
 assert.deepEqual(config.tabBar.list.map((item:{pagePath:string})=>item.pagePath),['pages/main/index','pages/main/worker/index','pages/main/orders/index','pages/main/me/index'])
 for(const path of config.tabBar.list.map((item:{pagePath:string})=>item.pagePath)){
  assert.ok(config.pages.some((item:{path:string})=>item.path===path))
  const source=readFileSync(new URL('../'+path+'.vue',import.meta.url),'utf8');assert.match(source,/<MainPage/);assert.doesNotMatch(source,/reLaunch|<BottomNav/)
 }
 assert.match(frame,/tab==='worker'\|\|tab==='orders'/)
 assert.match(frame,/height:\s*calc\(100%\s*-\s*var\(--status-bar-height/)
})

test('图文图片使用可信宽高预占位，未知或异常旧尺寸不会因加载改变高度', () => {
  assert.deepEqual(communityImageStyle({ type: 'image', image_width: 1200, image_height: 800 }), { paddingTop: 800 / 1200 * 100 + '%' })
  assert.deepEqual(communityImageStyle({ type: 'image', image_width: 800, image_height: 1600 }), { paddingTop: '200%' })
  for (const values of [{}, { image_width: 0, image_height: 400 }, { image_width: -1, image_height: 400 }, { image_width: 100, image_height: Infinity }, { image_width: 1.1, image_height: 400 }]) {
    assert.deepEqual(communityImageStyle({ type: 'image', ...values }), { paddingTop: '56.25%' })
  }
  const source = readFileSync(new URL('../pages/community/clubs/detail.vue', import.meta.url), 'utf8')
  assert.match(source, /class="detail-image-frame" :style="communityImageStyle\(block\)"/)
  assert.match(source, /mode="aspectFit" lazy-load class="detail-image"/)
  assert.doesNotMatch(source, /mode="widthFix" class="detail-image"/)
})
