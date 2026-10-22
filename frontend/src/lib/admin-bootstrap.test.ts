/** 文件作用：以真实store初始化和管理页脚本验证冷启动断网可见错误、重新初始化及权限边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'

/** 本轮独立配置页面各自已有的初始化重试入口。 */
type ManagementPage='platform'|'wallet'|'settings'|'ai'|'messages'
/** 运行生产store与生产页面，仅控制网络和官方生命周期边界。 */
function harness(name:ManagementPage){
  const storage=new Map<string,string>([['jijie_token','token-A']]),requests:DeferredRequest[]=[]
  const lifecycle:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({reactive,ref,computed,watch,uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},token:()=>storage.get('jijie_token')||'',notify:()=>{},go:()=>{},onLoad:(callback:PageLifecycle['load'])=>{lifecycle.load=callback},onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('./store.ts',import.meta.url),'utf8'))+'\nglobalThis.store={state}',context)
  const exposed=name==='platform'?'load,loaded,error,form':name==='wallet'?'showPage,loaded,bootstrapError,initializing,form,withdrawals':name==='ai'?'load,loaded,error,form,status':name==='messages'?'load,loaded:authorized,error,form':'load,loaded,error,payment,tab'
  vm.runInContext(executable(pageScript('manage/'+name+'.vue'))+'\nglobalThis.page={'+exposed+'}',context)
  return{name,page:context.page,store:context.store,requests,lifecycle,
    /** 取得真实API已发出的单次请求，重复读取或缺失均报错。 */
    take(route:string){const request=requests.find(item=>!item.taken&&item.path===route);assert.ok(request,'缺少请求 '+route);request.taken=true;return request},
    /** 操作页面上错误区域绑定的真实初始化重试方法。 */
    retry(){return name==='wallet'?context.page.showPage():context.page.load()}
  }
}
/** 推进真实Promise和Vue身份watch到可观察状态。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 完成真实bootstrap及真实本人资料请求，不直接给页面注入已知身份。 */
async function identify(subject:ReturnType<typeof harness>,role='admin'){
  subject.take('/bootstrap').resolve({games:[],dev_mode:true,site_name:'开机电竞',announcement:''});await flush()
  subject.take('/me').resolve({id:'admin-A',role,nickname:'测试用户',status:'active'});await flush()
}
/** 返回实际配置接口约定的最少合法内容，避免使用页面内部辅助方法造状态。 */
function resolveConfiguration(subject:ReturnType<typeof harness>){
  if(subject.name==='platform')subject.take('/admin/settings').resolve({site_name:'恢复平台',announcement:'真实重试读取'})
  else if(subject.name==='wallet'){subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve([]);subject.take('/admin/wallet-settings').resolve({settings:{withdrawal_enabled:false,transfer_scene_id:'',transfer_scene_report_infos:[],transfer_notify_url:'',transfer_remark:'',user_recv_perception:''},transfer_ready:false,configuration_notice:'尚未配置'})}
  else if(subject.name==='ai')subject.take('/admin/ai-settings').resolve({base_url:'https://maas.qianwenaiapi.com/compatible-mode/v1',model:'qwen3.8-flash',prompt:'审核提示词'.repeat(10),has_api_key:false,ready:false,mode:'local_simulation',configuration_notice:'本地模拟不调用真实模型'})
  else if(subject.name==='settings'){const document={title:'已发布协议',version:'v1',content:'完整协议正文'.repeat(8)};subject.take('/admin/agreements').resolve({user_agreement:document,privacy_policy:document});subject.take('/admin/payment-settings').resolve({merchant_id:'public-merchant',certificate_serial:'',platform_public_key_id:'',notify_url:'',refund_notify_url:'',configured:false,storage_available:true,source:'environment',api_v3_key_set:false,merchant_private_key_set:false,platform_public_key_set:false})}
}

for(const name of ['platform','wallet','settings','ai','messages'] as const){
  test(name+'冷启动bootstrap失败后，真实初始化重试可恢复管理员配置',async()=>{
    const subject=harness(name);subject.lifecycle.load({tab:'payment'});subject.lifecycle.show();await flush();subject.take('/bootstrap').reject({message:'冷启动网络不可用'});await flush()
    assert.equal(subject.store.state.ready,false);assert.equal(subject.store.state.user,null);assert.equal((name==='wallet'?subject.page.bootstrapError:subject.page.error).value,'冷启动网络不可用');assert.equal(subject.requests.filter(item=>item.path.startsWith('/admin/')).length,0)
    const retry=subject.retry();await flush();assert.equal(subject.requests.filter(item=>item.path==='/bootstrap').length,2);await identify(subject);resolveConfiguration(subject);await flush();await retry
    assert.equal(subject.store.state.ready,true);assert.equal(subject.store.state.user.role,'admin');assert.equal(subject.page.loaded.value,true);assert.equal((name==='wallet'?subject.page.bootstrapError:subject.page.error).value,'')
    if(name==='settings'){assert.equal(subject.page.tab.value,'payment');assert.equal(subject.page.payment.merchant_id,'public-merchant')}
    if(name==='platform')assert.equal(subject.page.form.site_name,'恢复平台')
    if(name==='wallet')assert.equal(subject.page.initializing.value,false)
  })

  test(name+'初始化重试确认普通用户后，保持权限拒绝且不读取管理接口',async()=>{
    const subject=harness(name);subject.lifecycle.show();await flush();subject.take('/bootstrap').reject({message:'离线'});await flush();const retry=subject.retry();await flush();await identify(subject,'user');await retry;await flush()
    assert.equal(subject.store.state.ready,true);assert.equal(subject.store.state.user.role,'user');assert.equal(subject.page.loaded.value,false);assert.equal(subject.requests.filter(item=>item.path.startsWith('/admin/')).length,0);assert.equal((name==='wallet'?subject.page.bootstrapError:subject.page.error).value,'')
  })
}
