/** 文件作用：执行真实管理详情组件，验证身份授权、作用域分页、消息归属、旧响应与确认弹层边界。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {computed,effectScope,nextTick,reactive,ref,watch} from 'vue'
import {executable,injectBusinessAPIs} from './sfc-test'
import type {DeferredRequest,DeferredUpload} from './sfc-test'
import {canConfirmClubTransfer,isPlatformRoot} from './management-permissions'
import type {User} from '../types/models'

/** 真实脚本的完整目标资料示例，不包含微信身份与支付秘密。 */
const user:User={id:'target-A',nickname:'北辰',avatar:'',bio:'游戏爱好者',profile_cover:'',role:'user',platform_admin_role:'' as const,status:'active',provider_enabled:true,phone_verified:true,phone_masked:'138****8000',created_at:'2026-10-25T00:00:00Z'}
const permissions={can_edit:true,can_manage_admins:true,can_send_message:true}
const funds={spendable_cents:7000,available_cents:1000,recharge_cents:6000}
/** 使用真实组件和API，仅替代网络完成时序与官方确认弹框。 */
function harness(t:TestContext,scope:'platform'|'club'='platform'){
  const props=reactive({scope,targetId:'target-A',clubId:'club-A'}),state=reactive({user:{id:'admin-A',role:'admin',platform_admin_role:'root',status:'active'}}),requests:DeferredRequest[]=[],uploads:DeferredUpload[]=[],notices:unknown[]=[],dialogs:Array<{resolve:(value:{confirm:boolean})=>void}>=[]
  const hooks={show:()=>{},hide:()=>{},unload:()=>{}};let revision=0
  const context=vm.createContext({ref,computed,reactive,watch,nextTick,state,defineProps:()=>props,init:async()=>{},sessionRevision:()=>revision,token:()=>state.user.id+'token',notify:(value:unknown)=>notices.push(value),onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},chooseUploadImage(purpose:string,options?:object){return new Promise((resolve,reject)=>uploads.push({purpose,options,resolve,reject}))},uni:{showModal:()=>new Promise(resolve=>dialogs.push({resolve})),setClipboardData:()=>{}},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context)
  const source=readFileSync(new URL('../components/ManagedPersonCenter.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(source)
  const effect=effectScope();effect.run(()=>vm.runInContext(executable(source)+';globalThis.page={load,loadRecords,retryRecords,selectTab,selectOrderRole,platform,member,wallet,entries,orders,moderations,hasMore,recordError,openMessage,message,submitDialog,dialog,busy,toggleFrozen,toggleDeputy,form,openEditor,permissions,upload,uploading}',context));t.after(()=>effect.stop())
  return{page:context.page,props,state,hooks,requests,uploads,notices,dialogs,
    /** 获取确实发出的指定请求。 */
    take(path:string,method='GET'){const row=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(row,'缺少请求 '+method+' '+path);row.taken=true;return row},
    /** 切号触发真实Vue清理与新目标读取。 */
    switchUser(){revision++;state.user={id:'admin-B',role:'admin',platform_admin_role:'deputy',status:'active'}},
  }
}
/** 排空异步边界，不依赖睡眠时长。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}
/** 完成首屏合法详情和当前页签读取。 */
async function ready(s:ReturnType<typeof harness>,items:unknown[]=[],hasMore=false):Promise<void>{s.hooks.show();await flush();if(s.props.scope==='platform'){s.take('/admin/users/target-A').resolve({user,permissions:{...permissions}});await flush();s.take('/admin/users/target-A/wallet').resolve(funds);await flush();s.take('/admin/users/target-A/wallet/entries?limit=20&offset=0').resolve({items,total:items.length,offset:0,limit:20,has_more:hasMore})}else{s.take('/clubs/club-A/members/target-A').resolve({user,membership:{user_id:user.id,status:'active'},management_role:'',permissions:{...permissions},scope_finance:{club_id:'club-A',user_id:user.id,settled_income_cents:8500,ongoing_cents:3000,settling_cents:0,club_deposit_cents:20000,club_deposit_refunding_cents:0}});await flush();s.take('/clubs/club-A/members/target-A/orders?limit=20&offset=0').resolve({items,total:items.length,offset:0,limit:20,has_more:hasMore})}await flush()}

test('根身份兼容历史admin，副admin不得继续授予平台权限',async t=>{
  assert.equal(isPlatformRoot({...user,role:'admin'}),true);assert.equal(isPlatformRoot({...user,role:'admin',platform_admin_role:'deputy'}),false)
  const s=harness(t);await ready(s);s.state.user.platform_admin_role='deputy';await s.page.toggleDeputy();assert.equal(s.dialogs.length,0);assert.equal(s.requests.filter(item=>item.method==='PATCH').length,0)
})
test('用户冻结需确认、不能冻结自己，重复点击不重复写，旧确认不跨目标',async t=>{
  const s=harness(t);await ready(s);const first=s.page.toggleFrozen();await s.page.toggleFrozen();assert.equal(s.dialogs.length,1);s.dialogs[0].resolve({confirm:false});await first;assert.equal(s.requests.filter(item=>item.method==='PATCH').length,0)
  const old=s.page.toggleFrozen();s.props.targetId='target-B';await flush();s.dialogs[1].resolve({confirm:true});await old;assert.equal(s.requests.filter(item=>item.method==='PATCH').length,0)
})
test('旧管理确认在隐藏并返回后不能执行原冻结操作',async t=>{
  const s=harness(t);await ready(s);const pending=s.page.toggleFrozen();s.hooks.hide();s.hooks.show();await flush();s.dialogs[0].resolve({confirm:true});await pending;assert.equal(s.requests.filter(item=>item.method==='PATCH').length,0)
})
test('平台目标流水失败重试原offset，页签切换拒绝旧流水',async t=>{
  const s=harness(t);await ready(s,Array.from({length:20},(_,i)=>({id:'entry-'+i,amount_cents:i})),true)
  const more=s.page.loadRecords(true);s.take('/admin/users/target-A/wallet/entries?limit=20&offset=20').reject(new Error('分页失败'));await more;assert.equal(s.page.entries.value.length,20)
  const retry=s.page.retryRecords();const pending=s.take('/admin/users/target-A/wallet/entries?limit=20&offset=20');s.page.selectTab('orders');s.take('/admin/users/target-A/orders?limit=20&offset=0&role=all').resolve({items:[{id:'order-A'}],has_more:false});pending.resolve({items:[{id:'old-entry'}],has_more:false});await retry;await flush();assert.equal(s.page.entries.value.length,0);assert.equal(s.page.orders.value[0].id,'order-A')
})
test('组织成员页只查询本俱乐部订单、收入和审核，不读取个人全局钱包',async t=>{
  const s=harness(t,'club');await ready(s);assert.equal(s.requests.some(item=>item.path.includes('/wallet')),false)
  s.page.selectTab('funds');s.take('/clubs/club-A/members/target-A/finance/entries?limit=20&offset=0').resolve({items:[{id:'scoped',scope:'club_member_deposit',amount_cents:20000}],has_more:false});await flush();assert.equal(s.page.entries.value[0].scope,'club_member_deposit')
  s.page.selectTab('moderation');s.take('/clubs/club-A/members/target-A/moderation?limit=20&offset=0').resolve({items:[],has_more:false});await flush();assert.equal(s.requests.some(item=>item.path.startsWith('/admin/')),false)
})
test('成员或平台消息固定目标，重复提交只发一笔，切号后不发成功提示',async t=>{
  for(const kind of ['platform','club'] as const)await t.test(kind,async child=>{const s=harness(child,kind);await ready(s);s.page.openMessage();s.page.message.title='测试通知';s.page.message.body='请查看当前订单';const sending=s.page.submitDialog();await s.page.submitDialog();const path=kind==='platform'?'/admin/users/target-A/messages':'/clubs/club-A/members/target-A/messages',writing=s.take(path,'POST');assert.deepEqual(JSON.parse(JSON.stringify(writing.body)),{title:'测试通知',body:'请查看当前订单'});assert.equal(s.requests.filter(item=>item.path===path).length,1);s.switchUser();await flush();writing.resolve({recipient_count:1});await sending;assert.equal(s.notices.length,0);assert.equal(s.page.dialog.value,null)})
})
test('组织副admin没有继续授权入口，服务端拒绝权限后旧资金立即清理',async t=>{
  const s=harness(t,'club');await ready(s);s.page.member.value.permissions.can_manage_admins=false;await s.page.toggleDeputy();assert.equal(s.dialogs.length,0);const reading=s.page.load();await flush();s.take('/clubs/club-A/members/target-A').reject({status:403,message:'已无管理权'});await reading;assert.equal(s.page.member.value,null);assert.equal(s.page.entries.value.length,0)
})
test('微信收款仅当前原负责人，组织副admin和变更受益人不能确认',()=>{
  assert.equal(canConfirmClubTransfer({id:'owner'},{owner_id:'owner'},{user_id:'owner'}),true)
  assert.equal(canConfirmClubTransfer({id:'deputy'},{owner_id:'owner'},{user_id:'owner'}),false)
  assert.equal(canConfirmClubTransfer({id:'owner'},{owner_id:'owner'},{user_id:'other'}),false)
})

test('同账号冻结、降级或失去平台角色立即清私有详情，旧确认不能继续写',async t=>{
  for(const [field,value] of [['status','frozen'],['platform_admin_role','deputy'],['role','user']] as const)await t.test(field,async child=>{
    const s=harness(child);await ready(s,[{id:'private-entry'}],true);const pending=s.page.toggleFrozen();assert.equal(s.dialogs.length,1);s.state.user[field]=value
    assert.equal(s.page.platform.value,null);assert.equal(s.page.wallet.value,null);assert.equal(s.page.entries.value.length,0);s.dialogs[0].resolve({confirm:true});await pending;assert.equal(s.requests.filter(item=>item.method==='PATCH').length,0)
    if(field!=='platform_admin_role'){await flush();assert.equal(s.requests.length,3,'失去基础资格后不再读私有API')}
  })
})

test('冻结期间迟到详情不继续查钱包，钱包403同时撤下旧详情、资金和弹框',async t=>{
  const old=harness(t);old.hooks.show();await flush();const reading=old.take('/admin/users/target-A');old.state.user.status='frozen';reading.resolve({user,permissions:{...permissions}});await flush();assert.equal(old.page.platform.value,null);assert.equal(old.requests.length,1)
  const s=harness(t);await ready(s);s.page.openMessage();s.page.message.body='不能保留的旧权限草稿';const count=s.requests.filter(item=>item.path.includes('/entries')).length,loading=s.page.load();await flush();s.take('/admin/users/target-A').resolve({user,permissions:{...permissions}});await flush();s.take('/admin/users/target-A/wallet').reject({status:403,message:'钱包访问权限已失效'});await loading
  assert.equal(s.page.platform.value,null);assert.equal(s.page.wallet.value,null);assert.equal(s.page.dialog.value,null);assert.equal(s.page.message.body,'');assert.equal(s.requests.filter(item=>item.path.includes('/entries')).length,count)
})

test('组织管理能力变化取消旧授权确认，全失权只清本机显示不删除任何资料',async t=>{
  const s=harness(t,'club');await ready(s,[{id:'club-order'}],true);const pending=s.page.toggleDeputy();assert.equal(s.dialogs.length,1);s.page.member.value.permissions.can_manage_admins=false;s.dialogs[0].resolve({confirm:true});await pending;assert.equal(s.requests.filter(item=>item.method==='PATCH').length,0);assert.ok(s.page.member.value,'仍有读取权时保留服务端确认的新详情')
  s.page.openMessage();s.page.message.title='旧草稿';s.page.member.value.permissions.can_edit=false;s.page.member.value.permissions.can_send_message=false
  assert.equal(s.page.member.value,null);assert.equal(s.page.orders.value.length,0);assert.equal(s.page.message.title,'');assert.equal(s.page.dialog.value,null);assert.equal(s.requests.filter(item=>item.method!=='GET').length,0)
})

test('合法原生上传隐藏保留草稿，切号作废旧上传且旧finally不解锁新上传',async t=>{
  const s=harness(t);await ready(s);s.page.openEditor();s.page.form.nickname='原生选图前草稿';const selecting=s.page.upload('avatar');s.hooks.hide();s.uploads[0].resolve('/api/media/same-user.jpg');await selecting;assert.equal(s.page.form.nickname,'原生选图前草稿');assert.equal(s.page.form.avatar,'/api/media/same-user.jpg')
  s.hooks.show();await flush();s.take('/admin/users/target-A').resolve({user,permissions:{...permissions}});await flush();s.take('/admin/users/target-A/wallet').resolve(funds);await flush();s.take('/admin/users/target-A/wallet/entries?limit=20&offset=0').resolve({items:[],has_more:false});await flush()
  const old=s.page.upload('avatar');s.switchUser();await flush();s.take('/admin/users/target-A').resolve({user,permissions:{...permissions}});await flush();s.take('/admin/users/target-A/wallet').resolve(funds);await flush();s.take('/admin/users/target-A/wallet/entries?limit=20&offset=0').resolve({items:[],has_more:false});await flush()
  s.page.openEditor();s.page.form.nickname='新管理员草稿';const current=s.page.upload('avatar');assert.equal(s.uploads.length,3);s.uploads[1].resolve('/api/media/old-admin.jpg');await old;assert.equal(s.page.uploading.value,true);assert.notEqual(s.page.form.avatar,'/api/media/old-admin.jpg')
  s.uploads[2].resolve('/api/media/new-admin.jpg');await current;assert.equal(s.page.uploading.value,false);assert.equal(s.page.form.avatar,'/api/media/new-admin.jpg');assert.equal(s.page.form.nickname,'新管理员草稿')
})
