/** 文件作用：执行真实store主题函数，验证局部游戏主题和显式选择的本地偏好相互独立。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, computed, ref } from 'vue'
import { executable, pageScript } from './sfc-test'

/** 在可观察的存储边界执行真实store，不替换主题或选择逻辑。 */
function harness() {
  const storage = new Map([['jijie_game', 'delta']]), writes: Array<[string, string]> = []
  const context = vm.createContext({ reactive, computed, ref, uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => { writes.push([key, value]); storage.set(key, value) } } })
  vm.runInContext(executable(readFileSync(new URL('./store.ts', import.meta.url), 'utf8')) + '\nglobalThis.subject={state,pageTheme,themeStyle,selectGame}', context)
  context.subject.state.games = [
    { id: 'delta', primary_color: '#00e18a', bg_color: '#0a1110', surface_color: '#15201e', text_color: '#f2f5f3' },
    { id: 'honor', primary_color: '#e3ba71', bg_color: '#18120a', surface_color: '#2b2012', text_color: '#fff8e8' },
  ]
  return { subject: context.subject, storage, writes, context }
}

test('读取其它游戏的局部主题不改变主游戏或写入本地缓存', () => {
  const { subject, storage, writes } = harness()
  const style = subject.pageTheme('honor')
  assert.equal(style['--accent'], '#e3ba71')
  assert.equal(style['--bg'], '#18120a')
  assert.equal(subject.state.gameId, 'delta')
  assert.equal(storage.get('jijie_game'), 'delta')
  assert.equal(writes.length, 0)
  style['--accent'] = '#ffffff'
  assert.equal(subject.pageTheme('honor')['--accent'], '#e3ba71')
})

test('指定游戏不存在或未指定时回退主游戏，空配置也有完整默认变量', () => {
  const { subject, writes } = harness()
  assert.equal(subject.pageTheme('missing')['--accent'], '#00e18a')
  assert.equal(subject.pageTheme()['--bg'], '#0a1110')
  subject.state.games = []
  assert.equal(Object.keys(subject.pageTheme('missing')).length, 4)
  assert.equal(subject.pageTheme('missing')['--surface'], '#15201e')
  assert.equal(writes.length, 0)
})

test('只有显式选择有效游戏持久化，局部计算能响应后台主题更新', () => {
  const { subject, storage, writes } = harness()
  subject.selectGame('honor')
  assert.equal(subject.state.gameId, 'honor')
  assert.equal(storage.get('jijie_game'), 'honor')
  assert.deepEqual(writes, [['jijie_game', 'honor']])
  subject.selectGame('missing')
  assert.equal(writes.length, 1)
  const localStyle = computed(() => subject.pageTheme('delta'))
  subject.state.games[0].primary_color = '#77ccaa'
  assert.equal(localStyle.value['--accent'], '#77ccaa')
  assert.equal(subject.themeStyle.value['--accent'], '#e3ba71')
  assert.equal(storage.get('jijie_game'), 'honor')
})

test('俱乐部的原生授权勾选色与局部主题一致，不改写用户主游戏偏好', () => {
  const { subject, storage, writes, context } = harness()
  Object.assign(context,{onLoad:()=>{},onShow:()=>{},onHide:()=>{},onShareAppMessage:()=>{}})
  vm.runInContext(executable(pageScript('community/clubs/detail.vue'))+'\nglobalThis.clubPage={club,clubGameId,clubTheme}',context)
  context.clubPage.club.value={game_ids:['honor']}
  assert.equal(context.clubPage.clubGameId.value,'honor')
  assert.equal(context.clubPage.clubTheme.value['--accent'],'#e3ba71')
  assert.equal(subject.state.gameId,'delta')
  assert.equal(storage.get('jijie_game'),'delta')
  assert.equal(writes.length,0)
  context.clubPage.club.value={game_ids:['delta','honor']}
  assert.equal(context.clubPage.clubTheme.value['--accent'],'#00e18a')
})

test('玩家主页初始加载和无封面刷新保持普通顶栏，已知有封面刷新保持沉浸', () => {
  const source=readFileSync(new URL('../pages/community/players/detail.vue',import.meta.url),'utf8')
  const expression=source.match(/<AppShell\b[^>]*:immersive="([^"]+)"/)?.[1]
  assert.ok(expression)
  for(const profile of [null,{profile_cover:''},{profile_cover:'/api/media/cover'}])for(const loading of [false,true]){
    assert.equal(vm.runInNewContext(expression,{profile,loading}),!!profile?.profile_cover)
  }
})
