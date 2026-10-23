/** 文件作用：执行真实控制台统计逻辑，验证角色隔离、快捷与自定义周期、迟到响应和部分失败。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { readFileSync } from 'node:fs'
import { reactive,ref,watch,effectScope } from 'vue'
import { executable,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import { adminDateRange } from './admin-chart'
/** 保持真实控制台生命周期和领域API，仅控制请求结束顺序。 */
function harness(t:TestContext,role='admin'){
  const pending:DeferredRequest[]=[],state=reactive({user:{id:'A',role}}),hooks={show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({ref,watch,state,adminDateRange,init:async()=>{},token:()=>state.user.id,sessionRevision:()=>0,onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},api:(path:string,method='GET',body={})=>new Promise((resolve,reject)=>pending.push({path,method,body,resolve,reject,taken:false}))})
  injectBusinessAPIs(context);const scope=effectScope();scope.run(()=>vm.runInContext(executable(readFileSync(new URL('./use-admin-console.ts',import.meta.url),'utf8'))+'\nglobalThis.page=useAdminConsole();globalThis.validatePeriod=validateAdminPeriod',context));t.after(()=>scope.stop())
  return{page:context.page,validate:context.validatePeriod,state,hooks,pending,
    /** 真实请求必须存在，不能用假结果跳过统计调用。 */
    take(prefix:string){const item=pending.find(row=>!row.taken&&row.path.startsWith(prefix));assert.ok(item,prefix);item.taken=true;return item},
  }
}
/** 排空Vue监听及领域API微任务。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 生成接口返回的统计夹具，不推导业务金额。 */
function result(start='2026-10-16',end='2026-10-22',amount=1000){return{period:{start,end,timezone:'Asia/Shanghai'},summary:{gmv_cents:amount,refund_cents:0,paid_order_count:1,refunded_order_count:0},series:[],legacy_untracked_orders:0,dev_mode:true,data_mode:'local',statistics_note:'本地测试'}}

test('个人用户和俱乐部管理员不请求后台总览或商户数据',async t=>{for(const role of ['user','clubowner']){const s=harness(t,role);s.hooks.show();await flush();assert.equal(s.pending.length,0);assert.equal(s.page.overview.value,null);assert.equal(s.page.analytics.value,null)}})
test('快捷周期旧响应不能覆盖新周期',async t=>{const s=harness(t);s.hooks.show();await flush();const old=s.take('/admin/analytics?');s.page.chooseRange('30');const latest=s.take('/admin/analytics?');assert.notEqual(old.path,latest.path);latest.resolve(result('2026-09-23','2026-10-22',9000));await flush();old.resolve(result());s.take('/admin/overview').resolve({users:5});await flush();assert.equal(s.page.analytics.value.summary.gmv_cents,9000);assert.equal(s.page.overview.value.users,5)})
test('切自定义时未完成的快捷统计不覆盖日期草稿',async t=>{const s=harness(t);s.hooks.show();await flush();const old=s.take('/admin/analytics?');s.page.chooseRange('custom');s.page.startDate.value='2026-01-01';s.page.endDate.value='2026-01-31';old.resolve(result());await flush();assert.equal(s.page.startDate.value,'2026-01-01');assert.equal(s.page.analytics.value,null)})
test('自定义日期校验真实日历、顺序及366日上限',t=>{const s=harness(t);assert.deepEqual({...s.validate('2024-02-29','2024-03-01')},{start:'2024-02-29',end:'2024-03-01'});for(const dates of [['2026-02-31','2026-03-04'],['2026-10-22','2026-10-21'],['2025-01-01','2026-10-22']])assert.throws(()=>s.validate(...dates))})
test('统计失败不清除已核实平台总览',async t=>{const s=harness(t);s.hooks.show();await flush();s.take('/admin/overview').resolve({users:8});s.take('/admin/analytics?').reject(new Error('统计暂不可用'));await flush();assert.equal(s.page.overview.value.users,8);assert.match(s.page.chartError.value,/不可用/);assert.equal(s.page.analytics.value,null)})
test('切换身份或销毁时迟到统计不回填私有经营数据',async t=>{for(const action of ['switch','unload']){const s=harness(t);s.hooks.show();await flush();const old=s.take('/admin/analytics?');if(action==='switch')s.state.user={id:'B',role:'user'};else s.hooks.unload();await flush();old.resolve(result());await flush();assert.equal(s.page.analytics.value,null)}})
test('跨日只有补款或退差金额时真实模板不显示无交易空态',()=>{const source=readFileSync(new URL('../pages/manage/index.vue',import.meta.url),'utf8'),condition=source.match(/<text v-if="([^"]+)" class="chart-empty">/);assert.ok(condition);for(const [gmv,refund,empty]of[[1000,0,false],[0,500,false],[0,0,true]] as const){const analytics={summary:{gmv_cents:gmv,refund_cents:refund,paid_order_count:0,refunded_order_count:0}};assert.equal(vm.runInNewContext(condition[1],{analytics}),empty)}})
