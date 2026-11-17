/** 文件作用：验证九类后台设置完整保留原入口、资金配置与业务操作分离，以及分类路由均已登记。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {ADMIN_BUSINESS_ENTRIES,ADMIN_CONTENT_ENTRIES,ADMIN_SETTINGS_GROUPS} from './admin-navigation'

test('设置按九类覆盖原配置，日常业务与运营入口不再混入配置',()=>{
 assert.deepEqual(ADMIN_SETTINGS_GROUPS.map(group=>group.label),['平台基础','游戏配置','账号与身份','协议管理','支付与结算','费率与押金','内容与审核','消息与订阅','客服支持'])
 const settings=ADMIN_SETTINGS_GROUPS.flatMap(group=>group.entries),business=[...ADMIN_BUSINESS_ENTRIES,...ADMIN_CONTENT_ENTRIES]
 assert.equal(ADMIN_SETTINGS_GROUPS.find(group=>group.id==='payments')?.entries.length,4)
 assert.ok(settings.some(item=>item.route==='/pages/manage/wallet?view=transfer'))
 assert.ok(business.some(item=>item.route==='/pages/manage/wallet'))
 assert.equal(new Set([...settings,...business].map(item=>item.route)).size,settings.length+business.length)
 const routes=JSON.parse(readFileSync(new URL('../pages.json',import.meta.url),'utf8')).pages as Array<{path:string}>
 for(const item of [...settings,...business])assert.ok(routes.some(page=>'/'+page.path===item.route.split('?')[0]),'未注册的管理入口 '+item.route)
})


test('前端模式、登录资料和支付方式各有独立入口',()=>{
 const entries=(id:string)=>ADMIN_SETTINGS_GROUPS.find(group=>group.id===id)!.entries
 assert.ok(entries('platform').some(item=>item.label==='前端模式'))
 assert.ok(entries('identity').some(item=>item.label==='微信登录'))
 assert.deepEqual(entries('payments').map(item=>item.label),['支付方式','微信支付','虚拟支付','商家转账'])
})
