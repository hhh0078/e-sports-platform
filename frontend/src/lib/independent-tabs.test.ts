/** 文件作用：验证四个真实主导航路径、无查询switchTab和首开订单筛选参数交接，旧链接保持兼容。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {MAIN_PAGES,mainNavigation,normalizePageUrl,navigate,applyTabQuery} from './navigation'

test('旧首页标签与别名归一化为独立路由，订单无需依赖tab参数',()=>{
 assert.equal(normalizePageUrl('/pages/main/index?tab=orders&role=buyer&status=refund'),'/pages/main/orders/index?role=buyer&status=refund')
 assert.equal(normalizePageUrl('/pages/orders/index?role=provider'),'/pages/main/orders/index?role=provider')
 assert.equal(normalizePageUrl('/pages/main/index?tab=me'),MAIN_PAGES.me)
 assert.equal(normalizePageUrl('/pages/main/index?tab=worker'),MAIN_PAGES.worker)
 assert.equal(normalizePageUrl('/pages/main/index?tab=invalid&game_id=honor'),MAIN_PAGES.player+'?game_id=honor')
 assert.equal(new Set(Object.values(MAIN_PAGES)).size,4)
})

test('首开订单页接收筛选，switchTab不带查询，不使用reLaunch重建页面',()=>{
 const platform=globalThis as unknown as {uni?:unknown;getCurrentPages?:unknown},oldUni=platform.uni,oldPages=platform.getCurrentPages,calls:string[]=[]
 platform.getCurrentPages=()=>[{route:'pages/main/me/index'}]
 platform.uni={switchTab:({url}:{url:string})=>{calls.push(url);applyTabQuery('orders',{})},navigateTo:()=>assert.fail('主导航不能入栈'),reLaunch:()=>assert.fail('主导航不能重启'),showToast:()=>{}}
 try{navigate('/pages/main/orders/index?role=provider&status=working');assert.deepEqual(calls,[MAIN_PAGES.orders]);assert.equal(mainNavigation.tab,'orders');assert.equal(mainNavigation.query.role,'provider');assert.equal(mainNavigation.query.status,'working')}
 finally{platform.uni=oldUni;platform.getCurrentPages=oldPages}
})

test('直接进入订单中心按订单页初始化，不能将主页参数覆盖到订单页',()=>{
 mainNavigation.tab='player';mainNavigation.query={game_id:'delta'}
 applyTabQuery('orders',{})
 assert.equal(mainNavigation.tab,'orders');assert.deepEqual(mainNavigation.query,{})
})
