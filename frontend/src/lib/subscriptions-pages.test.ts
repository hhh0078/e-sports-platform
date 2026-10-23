/** 文件作用：执行订阅真实页面脚本，检查四身份隔离、模板草稿、偏好保存、拒绝与切号回包，不代替真实微信收件验收。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {computed,reactive,ref,watch} from 'vue'
import {appendUniqueRows} from './list-page'
import {executable,pageScript} from './sfc-test'
import {SUBSCRIPTION_LABELS,SUBSCRIPTION_SOURCES,subscriptionCandidates,validateSubscriptionSettings,subscriptionSourceForField,subscriptionTemplateAllowed,subscriptionTemplateFieldsSupported,requestSubscriptionFromGesture} from './subscriptions'
import type {SubscriptionSettings,SubscriptionOverview,SubscriptionScene} from '../types/subscriptions'
import type {DeferredRequest} from './sfc-test'

/** 在可控网络和原生边界执行生产页面，保留业务判断与会话检查。 */
function harness(page:'manage'|'account'='manage',role='admin'){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],state=reactive({user:{id:'A',role},ready:true}),hooks={show:()=>{},hide:()=>{},unload:()=>{}},nativeCalls:Array<{tmplIds:string[];success:(data:Record<string,unknown>)=>void;fail:(error:unknown)=>void}>=[]
  const nativeHooks:{options?:{refresh:()=>Promise<void>;more?:()=>Promise<void>;blocked?:()=>boolean}}={}
  let authToken='token-A',revision=1
  const context=vm.createContext({ref,reactive,computed,watch,appendUniqueRows,useNativePageRefresh:(options:NonNullable<typeof nativeHooks.options>)=>{nativeHooks.options=options;return{}},state,game:{value:{primary_color:'#00e18a'}},token:()=>authToken,sessionRevision:()=>revision,init:async()=>{},notify:(message:unknown)=>notices.push(message),time:(value:string)=>value,SUBSCRIPTION_LABELS,SUBSCRIPTION_SOURCES,subscriptionCandidates,validateSubscriptionSettings,subscriptionSourceForField,subscriptionTemplateAllowed,subscriptionTemplateFieldsSupported,requestSubscriptionFromGesture,wechatSubscribeAPI:()=>((options:typeof nativeCalls[number])=>nativeCalls.push(options)),onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},errorMessage:(err:unknown)=>err instanceof Error?err.message:String(err),request(path:string,method='GET',body={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},queryString:(query:object={})=>{const entries=Object.entries(query);return entries.length?'?'+entries.map(([key,value])=>key+'='+value).join('&'):''}})
  vm.runInContext(executable(readFileSync(new URL('../api/subscriptions.api.ts',import.meta.url),'utf8')),context)
  vm.runInContext(executable(pageScript(page+'/subscriptions.vue'))+'\nglobalThis.page={load,'+(page==='manage'?'settings,fieldRows,body,save,authorized,tab,chooseTab,loadDeliveries,retryFailed,deliveries,total,deliveryError,retry,openTemplates,chooseTemplate,templates,templatesOpened':'overview,selected,selectedCandidates,setPreference,toggleSelection,subscribe')+',loading,busy,error}',context)
  return{page:context.page,requests,notices,nativeCalls,hooks,state,nativeHooks,
    /** 取得真实页面具名接口发出的请求。 */
    take(path:string,method='GET'){const request=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(request,'缺少 '+method+' '+path);request.taken=true;return request},
    /** 模拟真实换号，不靠客户端角色缓存继续处理。 */
    switchUser(id='B',role='user'){authToken='token-'+id;revision++;state.user={id,role}}
  }
}
/** 推进生产异步链到可观察网络边界。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 生成后端公开的六类测试目录。 */
function configuration():SubscriptionSettings{const scenes:SubscriptionScene[]=['provider_order','provider_refund','buyer_accepted','club_member','club_dispute','admin_club'];return{enabled:false,scenes:scenes.map(scene=>({scene,enabled:false,template_id:'',fields:{}})),sources:['title','time','amount'],scene_catalog:scenes.map(scene=>({scene,label:SUBSCRIPTION_LABELS[scene],description:'测试说明'}))}}
/** 生成当前用户实际可用的三项提醒。 */
function personal():SubscriptionOverview{return{enabled:true,scenes:configuration().scenes.slice(0,3).map((item,index)=>({...item,label:SUBSCRIPTION_LABELS[item.scene],description:'本人消息',template_id:'test_template_'+index,enabled:true,available:true,reason:'',preference:true})),preferences:{},authorization_note:'一次订阅一次提醒'}}
/** 完成真实初始化请求。 */
async function ready(page:'manage'|'account'='manage'){const subject=harness(page);subject.hooks.show();await flush();subject.take(page==='manage'?'/admin/subscriptions/settings':'/subscriptions').resolve(page==='manage'?configuration():personal());await flush();return subject}

test('普通玩家、打手及俱乐部负责人均不能读取或保存平台订阅配置',async()=>{for(const role of ['user','worker','clubowner']){const s=harness('manage',role);s.hooks.show();await flush();await s.page.save();await s.page.openTemplates('provider_order');assert.equal(s.requests.length,0);assert.equal(s.page.authorized.value,false)}})
test('配置关闭可保存未配置草稿且只发送六项约定字段',async()=>{const s=await ready();const saving=s.page.save(),request=s.take('/admin/subscriptions/settings','PATCH');assert.deepEqual(Object.keys(request.body).sort(),['enabled','scenes']);assert.equal((request.body.scenes as unknown[]).length,6);assert.deepEqual(Object.keys((request.body.scenes as object[])[0]).sort(),['enabled','fields','scene','template_id']);request.resolve(configuration());await saving;assert.equal(s.notices.at(-1),'订阅消息配置已保存')})
test('启用缺配置和重复字段不发请求，不以对象覆盖掩盖错误',async()=>{const s=await ready();s.page.settings.value.scenes[0].enabled=true;await s.page.save();assert.equal(s.requests.length,1);assert.match(s.page.error.value,/配置/);s.page.settings.value.scenes[0].enabled=false;s.page.fieldRows.provider_order=[{key:'thing1',source:'title'},{key:'thing1',source:'time'}];await s.page.save();assert.equal(s.requests.length,1);assert.match(s.page.error.value,/重复/)})
test('选择官方模板只生成待核对映射，不自动启用或发送',async()=>{const s=await ready(),opening=s.page.openTemplates('provider_order');s.take('/admin/subscriptions/templates').resolve({items:[{template_id:'real_template_format',title:'交易提醒',type:2,content:'',fields:[{key:'thing1',label:'事项',type:'thing'},{key:'time2',label:'时间',type:'time'}]}]});await opening;s.page.chooseTemplate(s.page.templates.value[0]);assert.equal(s.page.settings.value.scenes[0].enabled,false);assert.equal(s.page.settings.value.scenes[0].template_id,'real_template_format');assert.equal(s.page.templatesOpened.value,false);assert.equal(s.nativeCalls.length,0)})
test('不确定发送结果不能点击重试，记录仅使用服务端分页',async()=>{const s=await ready();s.page.chooseTab('deliveries');s.take('/admin/subscriptions/deliveries?offset=0&limit=20').resolve({items:[{id:'uncertain',status:'uncertain',can_retry:false}],total:1});await flush();await s.page.retry(s.page.deliveries.value[0]);assert.equal(s.requests.filter(item=>item.method==='POST').length,0)})
test('旧管理员读取和保存响应不能覆盖新账号或恢复管理页',async()=>{const s=await ready();const saving=s.page.save(),old=s.take('/admin/subscriptions/settings','PATCH');s.switchUser();await flush();old.resolve(configuration());await saving;assert.equal(s.page.settings.value,null);assert.equal(s.page.authorized.value,false);assert.equal(s.notices.length,0)})
test('用户偏好只更新本人场景且关闭后移出本次授权候选',async()=>{const s=await ready('account');const saving=s.page.setPreference('provider_order',false),request=s.take('/subscriptions/preferences','PATCH');assert.deepEqual(JSON.parse(JSON.stringify(request.body)),{preferences:{provider_order:false}});const result=personal();result.scenes[0].preference=false;result.preferences.provider_order=false;request.resolve(result);await saving;assert.ok(!s.page.selected.value.includes('provider_order'));assert.equal(s.requests.some(item=>item.path.startsWith('/admin/')),false)})
test('用户点击立即调用官方授权，拒绝仅记录reject且不访问管理员接口',async()=>{const s=await ready('account');const pending=s.page.subscribe();assert.equal(s.nativeCalls.length,1);assert.equal(s.requests.length,1);s.nativeCalls[0].success(Object.fromEntries(s.nativeCalls[0].tmplIds.map(id=>[id,'reject'])));await flush();const record=s.take('/subscriptions/authorizations','POST');assert.equal((record.body.results as Array<{status:string}>).every(item=>item.status==='reject'),true);record.resolve({ok:true});await pending;assert.equal(s.notices.at(-1),'未订阅成功，不影响正常使用')})
test('用户点击授权后换号，迟到accept不能记到新身份',async()=>{const s=await ready('account');const pending=s.page.subscribe();s.switchUser();await flush();s.nativeCalls[0].success(Object.fromEntries(s.nativeCalls[0].tmplIds.map(id=>[id,'accept'])));await pending;assert.equal(s.requests.some(item=>item.path==='/subscriptions/authorizations'),false)})
test('访客可正常显示提醒设置页，不读取私人接口',async()=>{const s=harness('account');s.state.user=null as unknown as typeof s.state.user;s.hooks.show();await flush();assert.equal(s.requests.length,0);assert.equal(s.page.overview.value,null)})

/** 执行真实共享弹层脚本，保留其加载、授权和回调竞态行为。 */
function reminderHarness(){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],state=reactive({user:{id:'A',role:'user'}}),hooks={show:()=>{},hide:()=>{},unload:()=>{}},nativeCalls:Array<{tmplIds:string[];success:(result:Record<string,unknown>)=>void;fail:(error:unknown)=>void}>=[]
  let revision=1,authToken='token-A'
  const context=vm.createContext({ref,computed,watch,state,token:()=>authToken,sessionRevision:()=>revision,subscriptionCandidates,requestSubscriptionFromGesture,wechatSubscribeAPI:()=>((options:typeof nativeCalls[number])=>nativeCalls.push(options)),notify:(message:unknown)=>notices.push(message),errorMessage:(err:unknown)=>err instanceof Error?err.message:String(err),defineProps:()=>({scenes:['provider_order'],promptKey:0}),withDefaults:(props:object,defaults:object)=>({...defaults,...props}),defineExpose:()=>{},onMounted:()=>{},onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},request(path:string,method='GET',body={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},queryString:()=>''})
  vm.runInContext(executable(readFileSync(new URL('../api/subscriptions.api.ts',import.meta.url),'utf8')),context)
  const source=readFileSync(new URL('../components/SubscribeReminder.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(source)
  vm.runInContext(executable(source)+'\nglobalThis.reminder={offer,open,close,subscribe,opened,overview,busy,error}',context)
  return{reminder:context.reminder,requests,notices,hooks,nativeCalls,
    /** 模拟异步授权期间真正换号。 */
    switchUser(){revision++;authToken='token-B';state.user={id:'B',role:'user'}}
  }
}

test('发布成功仅展示真实弹层，关闭不授权且成功导航只执行一次',async()=>{const s=reminderHarness();s.hooks.show();s.requests[0].resolve(personal());await flush();let completed=0;s.reminder.offer(()=>completed++);assert.equal(s.reminder.opened.value,true);assert.equal(s.nativeCalls.length,0);assert.equal(completed,0);s.reminder.close();s.reminder.close();assert.equal(completed,1);assert.equal(s.nativeCalls.length,0)})
test('发布成功模板未配置时不阻断成功导航，也不申请原生权限',()=>{const s=reminderHarness();let completed=0;s.reminder.offer(()=>completed++);assert.equal(completed,1);assert.equal(s.reminder.opened.value,false);assert.equal(s.nativeCalls.length,0)})
test('弹层明确点击才申请，原生暂时隐藏后仍可记录原账号结果并完成导航',async()=>{const s=reminderHarness();s.hooks.show();s.requests[0].resolve(personal());await flush();let completed=0;s.reminder.offer(()=>completed++);const pending=s.reminder.subscribe();assert.equal(s.nativeCalls.length,1);s.hooks.hide();s.nativeCalls[0].success({test_template_0:'accept'});await flush();assert.equal(s.requests[1].path,'/subscriptions/authorizations');assert.equal(completed,0);s.hooks.show();assert.equal(completed,1);s.requests[1].resolve({ok:true});await pending;assert.equal(completed,1);assert.equal(s.reminder.opened.value,false)})
test('弹层授权回包发生在换号后不能记录到新账号或继续旧导航',async()=>{const s=reminderHarness();s.hooks.show();s.requests[0].resolve(personal());await flush();let completed=0;s.reminder.offer(()=>completed++);const pending=s.reminder.subscribe();s.switchUser();await flush();s.nativeCalls[0].success({test_template_0:'accept'});await pending;assert.equal(s.requests.some(item=>item.path==='/subscriptions/authorizations'),false);assert.equal(completed,0)})
test('官方模板选择过滤type3长期和type4新版，不能混进一次性授权',async()=>{const s=await ready(),opening=s.page.openTemplates('provider_order');s.take('/admin/subscriptions/templates').resolve({items:[2,3,4].map(type=>({template_id:'test_template_'+type,title:'交易提醒',type,content:'',fields:[{key:'thing1',label:'事项',type:'thing'}]}))});await opening;assert.equal(s.page.templates.value.length,1);assert.equal(s.page.templates.value[0].type,2)})


test('发布成功授权上报失败也不阻断导航，导航在网络结束之前完成且只有一次',async()=>{const s=reminderHarness();s.hooks.show();s.requests[0].resolve(personal());await flush();let completed=0;s.reminder.offer(()=>completed++);const pending=s.reminder.subscribe();s.nativeCalls[0].success({test_template_0:'accept'});await flush();assert.equal(completed,1);s.requests[1].reject(new Error('订阅记录网络失败'));await pending;s.hooks.show();s.reminder.close();assert.equal(completed,1)})
test('发布成功弹层尚未授权就切后台，回到页面仍继续成功导航一次',async()=>{const s=reminderHarness();s.hooks.show();s.requests[0].resolve(personal());await flush();let completed=0;s.reminder.offer(()=>completed++);s.hooks.hide();assert.equal(completed,0);s.hooks.show();s.hooks.show();assert.equal(completed,1);assert.equal(s.nativeCalls.length,0)})
test('原生隐藏后上报失败，重新显示照常导航，换号则撤销旧导航',async()=>{for(const changeUser of [false,true]){const s=reminderHarness();s.hooks.show();s.requests[0].resolve(personal());await flush();let completed=0;s.reminder.offer(()=>completed++);const pending=s.reminder.subscribe();s.hooks.hide();s.nativeCalls[0].success({test_template_0:'reject'});await flush();s.requests[1].reject(new Error('断网'));await pending;if(changeUser){s.switchUser();await flush()}s.hooks.show();assert.equal(completed,changeUser?0:1)}})
test('已选三模板后增加目录更靠前场景不能隐式挤掉已选项',async()=>{const s=await ready('account'),data=personal();data.scenes.push({...data.scenes[0],scene:'club_member',template_id:'test_club_template'});s.page.overview.value=data;s.page.selected.value=['provider_refund','buyer_accepted','club_member'];s.page.toggleSelection('provider_order');assert.deepEqual([...s.page.selected.value],['provider_refund','buyer_accepted','club_member']);assert.match(String(s.notices.at(-1)),/最多申请三个/)})
test('官方模板列表只展示当前六类场景含义匹配的标题',async()=>{const s=await ready(),opening=s.page.openTemplates('provider_order');s.take('/admin/subscriptions/templates').resolve({items:['交易提醒','退款申请通知','接单成功提醒','商家入驻申请通知'].map((title,index)=>({template_id:'template_'+index,title,type:2,content:'',fields:[{key:'thing1',label:'事项',type:'thing'}]}))});await opening;assert.deepEqual([...s.page.templates.value.map((item:{title:string})=>item.title)],['交易提醒'])})


test('个人提醒下拉保留明确取消的本次勾选，不写偏好或重新请求微信授权',async()=>{const s=await ready('account');s.page.toggleSelection('provider_order');assert.ok(!s.page.selected.value.includes('provider_order'));const refreshing=s.nativeHooks.options?.refresh();await flush();s.take('/subscriptions').resolve(personal());await refreshing;assert.ok(!s.page.selected.value.includes('provider_order'));assert.equal(s.nativeCalls.length,0);assert.equal(s.requests.filter(item=>item.method!=='GET').length,0)})
test('管理员配置下拉只更新只读目录，不覆盖未保存模板和字段草稿',async()=>{const s=await ready();s.page.settings.value.scenes[0].template_id='local_unsaved_template';s.page.fieldRows.provider_order=[{key:'thing1',source:'title'}];const refreshing=s.nativeHooks.options?.refresh();s.take('/admin/subscriptions/settings').resolve(configuration());await refreshing;assert.equal(s.page.settings.value.scenes[0].template_id,'local_unsaved_template');assert.equal(s.page.fieldRows.provider_order.length,1);assert.equal(s.requests.filter(item=>item.method!=='GET').length,0)})
test('发送记录追加按原始offset推进，重复记录去重后不漏下一页',async()=>{const s=await ready();s.page.chooseTab('deliveries');s.take('/admin/subscriptions/deliveries?offset=0&limit=20').resolve({items:Array.from({length:20},(_,i)=>({id:'n'+i})),total:60});await flush();const second=s.nativeHooks.options?.more?.();s.take('/admin/subscriptions/deliveries?offset=20&limit=20').resolve({items:Array.from({length:20},(_,i)=>({id:'n'+(i+19)})),total:60});await second;assert.equal(s.page.deliveries.value.length,39);const third=s.nativeHooks.options?.more?.();s.take('/admin/subscriptions/deliveries?offset=40&limit=20').resolve({items:[],total:40});await third})

test('订阅日志错误重试恢复首读或追加，不把刷新失败当成下一页',async()=>{
  const s=await ready();s.page.chooseTab('deliveries');s.take('/admin/subscriptions/deliveries?offset=0&limit=20').resolve({items:Array.from({length:20},(_,i)=>({id:'n'+i})),total:40});await flush()
  const refresh=s.nativeHooks.options?.refresh();s.take('/admin/subscriptions/deliveries?offset=0&limit=20').reject(new Error('首读失败'));await refresh
  const firstRetry=s.page.retryFailed();s.take('/admin/subscriptions/deliveries?offset=0&limit=20').resolve({items:Array.from({length:20},(_,i)=>({id:'n'+i})),total:40});await firstRetry
  const more=s.nativeHooks.options?.more?.();s.take('/admin/subscriptions/deliveries?offset=20&limit=20').reject(new Error('追加失败'));await more
  const secondRetry=s.page.retryFailed();s.take('/admin/subscriptions/deliveries?offset=20&limit=20').resolve({items:[{id:'last'}],total:21});await secondRetry
  assert.equal(s.page.deliveries.value.length,21);assert.equal(s.requests.some(item=>item.method!=='GET'),false)
})
