/** 文件作用：执行公开二级页真实脚本，验证服务和评价真实分页、原始偏移、刷新竞态及失效目标清理。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {ref,reactive,computed,watch,effectScope} from 'vue'
import {executable,pageScript,injectBusinessAPIs} from './sfc-test'
import type {DeferredRequest,PageLifecycle} from './sfc-test'
import type {NativePageRefreshOptions} from './native-page-refresh'

/** 仅替换网络和原生页面边界，分页、合并及失效判断均运行生产实现。 */
function harness(kind:'service'|'player'|'club'){
  const requests:DeferredRequest[]=[],native:{options?:NativePageRefreshOptions}={},scope=effectScope()
  const lifecycle:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const state=reactive({user:{id:'viewer',role:'user'},games:[],gameId:'delta',ready:true})
  const context=vm.createContext({ref,computed,reactive,watch,state,init:async()=>{},game:{value:{cover_url:''}},token:()=>'',sessionRevision:()=>1,pageTheme:()=>({}),requirePhone:async()=>true,
    uni:{getWindowInfo:()=>({windowWidth:390})},onLoad:(fn:PageLifecycle['load'])=>{lifecycle.load=fn},onShow:(fn:()=>void)=>{lifecycle.show=fn},onHide:(fn:()=>void)=>{lifecycle.hide=fn},onUnload:(fn:()=>void)=>{lifecycle.unload=fn},onResize:()=>{},
    useNativePageRefresh:(options:NativePageRefreshOptions)=>{native.options=options;return{}},
    api:(path:string,method='GET',body={})=>new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false})),notify:()=>{},
  })
  injectBusinessAPIs(context)
  const route=kind==='service'?'services/detail.vue':kind==='player'?'community/players/detail.vue':'community/clubs/detail.vue'
  const exposed=kind==='service'?'service,reviews,reviewsMore,load,retryFailed,quantity,requirements':kind==='player'?'profile,services,hasMore,load,retryFailed':'club,services,serviceMore,members,canManage,load,retryFailed,tab,visibleCertifiedMembers,visibleMembers,orders,loadOrders'
  scope.run(()=>vm.runInContext(executable(pageScript(route))+'\nglobalThis.page={'+exposed+'}',context))
  return{page:context.page,native,lifecycle,scope,requests,
    /** 取真实发出的分页地址，缺少或者重复消费均失败。 */
    take(offset=0){const prefix=kind==='service'?'/services/target':kind==='player'?'/players/target':'/clubs/target',path=prefix+'?limit=20&offset='+offset,item=requests.find(row=>!row.taken&&row.path===path);assert.ok(item,'缺少请求 '+path);item.taken=true;return item},
    /** 使用服务端返回形状建立公开业务对象，不从展示数量推算偏移。 */
    result(rows:Array<{id:string;status?:string}>){return kind==='service'?{id:'target',status:'active',reviews:rows}:kind==='player'?{profile:{id:'target',nickname:'玩家'},services:rows,clubs:[]}:{club:{id:'target',game_ids:['delta']},services:rows,members:[],can_manage:true}},
    /** 读取不同页面的实际列表以共同验证公开分页契约。 */
    rows(){return kind==='service'?context.page.reviews.value:context.page.services.value},
  }
}
/** 排空真实初始化与API的微任务，直到下一网络边界。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 建立服务端原始分页，状态明确为可售以排除业务过滤干扰。 */
function batch(start:number,count=20){return Array.from({length:count},(_,i)=>({id:'row'+(start+i),status:'active'}))}

for(const kind of ['service','player','club'] as const){
  test(kind+'两处错误重试都恢复失败方向，首读失败不跳下一页、追加失败不回首屏',async t=>{
    const s=harness(kind);t.after(()=>s.scope.stop());s.lifecycle.load({id:'target'});await flush();s.take().resolve(s.result(batch(0)));await flush()
    const refreshing=s.native.options?.refresh();await flush();s.take().reject(new Error('首读网络失败'));await refreshing
    const firstRetry=s.page.retryFailed();await flush();s.take().resolve(s.result(batch(0)));await firstRetry;assert.equal(s.rows().length,20)
    const more=s.native.options?.more?.();await flush();s.take(20).reject(new Error('追加网络失败'));await more
    const secondRetry=s.page.retryFailed();await flush();s.take(20).resolve(s.result(batch(20,2)));await secondRetry;assert.equal(s.rows().length,22)
    const route=kind==='service'?'services/detail.vue':kind==='player'?'community/players/detail.vue':'community/clubs/detail.vue'
    const source=pageScript(route);assert.ok(source.includes('async function retryFailed'))
  })
  test(kind+'公开列表按原始20条推进，去重不漏下一页且迟到追加不覆盖刷新',async t=>{
    const s=harness(kind);t.after(()=>s.scope.stop());s.lifecycle.load({id:'target'});await flush();s.take().resolve(s.result(batch(0)));await flush()
    assert.equal(s.rows().length,20)
    const second=s.native.options?.more?.();await flush();s.take(20).resolve(s.result(batch(19)));await second
    assert.equal(s.rows().length,39)
    const third=s.native.options?.more?.();await flush();const old=s.take(40)
    const refreshing=s.native.options?.refresh();await flush();s.take().resolve(s.result(batch(100,2)));await refreshing
    old.resolve(s.result(batch(40)));await third
    assert.deepEqual([...s.rows().map((item:{id:string})=>item.id)],['row100','row101'])
    assert.equal(s.requests.filter(item=>item.method!=='GET').length,0)
  })
  test(kind+'刷新网络失败保留原内容，404不继续显示已失效对象',async t=>{
    const s=harness(kind);t.after(()=>s.scope.stop());s.lifecycle.load({id:'target'});await flush();s.take().resolve(s.result(batch(0,2)));await flush()
    const offline=s.native.options?.refresh();await flush();s.take().reject(new Error('网络暂不可用'));await offline;assert.equal(s.rows().length,2)
    const removed=s.native.options?.refresh();await flush();s.take().reject({status:404,message:'目标已不存在'});await removed;assert.equal(s.rows().length,0)
    assert.equal((kind==='service'?s.page.service:kind==='player'?s.page.profile:s.page.club).value,null)
  })
}

test('服务评价分页及下拉不改购买数量和需求草稿',async t=>{
  const s=harness('service');t.after(()=>s.scope.stop());s.lifecycle.load({id:'target'});await flush();s.take().resolve(s.result(batch(0)));await flush();s.page.quantity.value=3;s.page.requirements.value='预约周六晚，先确认区服'
  const refreshing=s.native.options?.refresh();await flush();s.take().resolve(s.result(batch(20,1)));await refreshing
  assert.equal(s.page.quantity.value,3);assert.equal(s.page.requirements.value,'预约周六晚，先确认区服')
})

test('俱乐部成员宫格触底只增量渲染已认证成员，不伪造服务端分页',async t=>{
  const s=harness('club');t.after(()=>s.scope.stop());s.lifecycle.load({id:'target',tab:'members'});await flush()
  s.take().resolve({...s.result([]),members:Array.from({length:45},(_,i)=>({user_id:'member'+i,status:i<42?'active':'pending'}))});await flush()
  assert.equal(s.page.visibleCertifiedMembers.value.length,20);await s.native.options?.more?.();assert.equal(s.page.visibleCertifiedMembers.value.length,40);await s.native.options?.more?.();assert.equal(s.page.visibleCertifiedMembers.value.length,42)
  assert.equal(s.requests.length,1)
})

test('俱乐部团队订单触底只读订单下一页，按原始offset处理重复记录',async t=>{
  const s=harness('club');t.after(()=>s.scope.stop());s.lifecycle.load({id:'target'});await flush();s.take().resolve(s.result([]));await flush();s.page.tab.value='orders'
  const first=s.page.loadOrders();const initial=s.requests.find(item=>item.path==='/clubs/target/orders?limit=20&offset=0');assert.ok(initial);initial.resolve(batch(0));await first
  const second=s.native.options?.more?.();const next=s.requests.find(item=>item.path==='/clubs/target/orders?limit=20&offset=20');assert.ok(next);next.resolve(batch(19));await second;assert.equal(s.page.orders.value.length,39)
  const third=s.native.options?.more?.();const last=s.requests.find(item=>item.path==='/clubs/target/orders?limit=20&offset=40');assert.ok(last);last.resolve([]);await third;assert.equal(s.requests.length,4)
})
