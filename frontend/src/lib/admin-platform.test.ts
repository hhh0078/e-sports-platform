/** 文件作用：执行平台设置真实页面脚本，验证权限、表单边界、保存刷新及跨会话响应隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { reactive, ref, watch } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'

/** 在可控传输和生命周期边界运行真实页面，不复制生产保存逻辑。 */
function harness(role='admin',refreshIdentity=false){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],initializations:boolean[]=[]
  const state=reactive({ready:true,user:{id:'admin-A',role}})
  let authToken='token-A',revision=1
  const lifecycle={show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({reactive,ref,watch,state,token:()=>authToken,sessionRevision:()=>revision,init:async(force=false)=>{initializations.push(force);if(force&&refreshIdentity)state.user={...state.user}},notify:(message:unknown)=>notices.push(message),onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context)
  vm.runInContext(executable(pageScript('manage/platform.vue'))+'\nglobalThis.page={form,loading,busy,loaded,error,load,save,restore,showPage,hidePage}',context)
  return{page:context.page,requests,notices,initializations,lifecycle,state,
    /** 读取真实具名API发出的下一次网络请求。 */
    take(method='GET'){const request=requests.find(item=>!item.taken&&item.path==='/admin/settings'&&item.method===method);assert.ok(request,'缺少'+method+'平台设置请求');request.taken=true;return request},
    /** 模拟页面隐藏及真实令牌、会话和账号同时变更。 */
    switchAdmin(){lifecycle.hide();authToken='token-B';revision++;state.user={id:'admin-B',role:'admin'}}
  }
}
/** 推进真实异步脚本到下个可观测边界。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 完成一次管理员平台设置读取。 */
async function ready(){const subject=harness();subject.lifecycle.show();await flush();subject.take().resolve({site_name:'集结',announcement:'公告A'});await flush();return subject}

test('普通玩家及俱乐部管理员不能读取或保存平台设置',async()=>{
  for(const role of ['user','club_admin']){const subject=harness(role);subject.lifecycle.show();await flush();subject.page.form.site_name='越权名称';await subject.page.save();assert.equal(subject.requests.length,0);assert.equal(subject.page.loaded.value,false)}
})

test('平台名称按真实后端2至24字及公告300字契约校验',async()=>{
  const subject=await ready()
  for(const name of ['','单','字'.repeat(25)]){subject.page.form.site_name=name;await subject.page.save()}
  subject.page.form.site_name='集结';subject.page.form.announcement='文'.repeat(301);await subject.page.save();assert.equal(subject.requests.length,1)
  subject.page.form.site_name='字'.repeat(24);subject.page.form.announcement='文'.repeat(300);const saving=subject.page.save();const request=subject.take('PATCH');assert.equal(String(request.body.site_name).length,24);assert.equal(String(request.body.announcement).length,300);request.resolve(request.body);await saving
})

test('保存只提交平台公开文字、允许清空公告并强制刷新全局配置',async()=>{
  const subject=await ready();subject.page.form.site_name='  新集结  ';subject.page.form.announcement='   '
  const saving=subject.page.save();const request=subject.take('PATCH');assert.deepEqual(JSON.parse(JSON.stringify(request.body)),{site_name:'新集结',announcement:''});request.resolve({site_name:'新集结',announcement:''});await saving
  assert.deepEqual(subject.initializations,[false,true]);assert.equal(subject.page.form.announcement,'');assert.equal(subject.notices.at(-1),'平台设置已保存');assert.equal(subject.page.busy.value,false)
})

test('保存进行中防止连点，取消修改恢复已确认结果',async()=>{
  const subject=await ready();subject.page.form.site_name='新集结';const saving=subject.page.save();await subject.page.save();assert.equal(subject.requests.filter(item=>item.method==='PATCH').length,1);subject.take('PATCH').resolve({site_name:'服务端名称',announcement:'最新公告'});await saving
  subject.page.form.site_name='未保存草稿';subject.page.restore();assert.equal(subject.page.form.site_name,'服务端名称');assert.equal(subject.page.form.announcement,'最新公告')
})

test('旧管理员读取结果不能覆盖新账号草稿',async()=>{
  const subject=harness();subject.lifecycle.show();await flush();const old=subject.take();subject.switchAdmin();await flush();subject.lifecycle.show();await flush();subject.take().resolve({site_name:'账号B平台',announcement:'B'});await flush();old.resolve({site_name:'旧平台',announcement:'A'});await flush();assert.equal(subject.page.form.site_name,'账号B平台')
})

test('旧管理员保存返回不能回填新账号或刷新全局配置',async()=>{
  const subject=await ready();subject.page.form.site_name='账号A草稿';const saving=subject.page.save();const old=subject.take('PATCH');subject.switchAdmin();await flush();subject.lifecycle.show();await flush();subject.take().resolve({site_name:'账号B平台',announcement:'B'});await flush();old.resolve({site_name:'旧保存',announcement:'A'});await saving
  assert.equal(subject.page.form.site_name,'账号B平台');assert.equal(subject.initializations.filter(Boolean).length,0);assert.equal(subject.page.busy.value,false)
})

test('离页后的加载失效，返回时重读且错误可重试',async()=>{
  const subject=harness();subject.lifecycle.show();await flush();const old=subject.take();subject.lifecycle.hide();old.resolve({site_name:'隐藏响应',announcement:'旧'});await flush();assert.equal(subject.page.loaded.value,false)
  subject.lifecycle.show();await flush();subject.take().reject({message:'暂时离线'});await flush();assert.equal(subject.page.error.value,'暂时离线');const retry=subject.page.load();await flush();subject.take().resolve({site_name:'恢复平台',announcement:''});await retry;assert.equal(subject.page.error.value,'');assert.equal(subject.page.form.site_name,'恢复平台')
})

test('全局刷新仅替换同身份资料对象时，不误判换号或抹掉保存成功状态',async()=>{
  const subject=harness('admin',true);subject.lifecycle.show();await flush();subject.take().resolve({site_name:'集结',announcement:''});await flush();subject.page.form.site_name='全新平台';const saving=subject.page.save();subject.take('PATCH').resolve({site_name:'全新平台',announcement:''});await saving;await flush()
  assert.equal(subject.page.loaded.value,true);assert.equal(subject.page.form.site_name,'全新平台');assert.equal(subject.notices.at(-1),'平台设置已保存');assert.equal(subject.requests.length,2)
})
