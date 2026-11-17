/** 文件作用：运行真实身份配置页，验证无密钥基础模式、撤销以及离页清理。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {reactive,ref,computed,watch} from 'vue'
import {executable,pageScript,injectBusinessAPIs} from './sfc-test'
import type {DeferredRequest} from './sfc-test'
/** 只替换请求与生命周期，不替换页面配置逻辑。 */
function harness(){
  const requests:DeferredRequest[]=[],lifecycle={show:()=>{},hide:()=>{},unload:()=>{}},state=reactive({ready:true,user:{id:'admin',role:'admin'}})
  const context=vm.createContext({reactive,ref,computed,watch,state,token:()=> 'fixture-token',sessionRevision:()=>1,init:async()=>{},notify:()=>{},onShow:(fn:()=>void)=>{lifecycle.show=fn},onHide:(fn:()=>void)=>{lifecycle.hide=fn},onUnload:(fn:()=>void)=>{lifecycle.unload=fn},api:(path:string,method='GET',body:Record<string,unknown>={})=>new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))})
  injectBusinessAPIs(context);vm.runInContext(executable(pageScript('manage/identity.vue'))+'\nglobalThis.page={mode,status,apiKey,loaded,busy,chooseMode,cancel,save}',context)
  return{page:context.page,lifecycle,requests,
    /** 取得页面真正发出的配置请求。 */
    take(method='GET'){const request=requests.find(row=>!row.taken&&row.method===method);assert.ok(request);request.taken=true;return request}}
}
/** 排空初始化及Vue侦听。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 构造不含秘密且未设置加密主密钥的脱敏响应。 */
function configuration(mode='strict'){return{mode,has_api_key:false,configured:mode==='basic',storage_available:false,api_url:'https://v2.xxapi.cn/api/idCardCheck'}}
test('没有API密钥和加密主密钥仍能保存基础方式，取消恢复服务端选择',async()=>{
  const h=harness();h.lifecycle.show();await flush();h.take().resolve(configuration());await flush();assert.equal(h.page.loaded.value,true)
  h.page.chooseMode('basic');h.page.cancel();assert.equal(h.page.mode.value,'strict');h.page.chooseMode('basic');const saving=h.page.save(),request=h.take('PUT');assert.equal(request.body.mode,'basic');assert.equal(request.body.api_key,'');request.resolve(configuration('basic'));await saving;assert.equal(h.page.status.mode,'basic');assert.equal(h.page.busy.value,false)
})
test('离页清除临时密钥且迟到保存结果不能恢复页面',async()=>{
  const h=harness();h.lifecycle.show();await flush();h.take().resolve(configuration());await flush();h.page.apiKey.value='synthetic-secret';const saving=h.page.save(),request=h.take('PUT');h.lifecycle.hide();assert.equal(h.page.apiKey.value,'');request.resolve(configuration('basic'));await saving;assert.equal(h.page.loaded.value,false);assert.equal(h.page.status.mode,'strict');assert.equal(request.body.api_key,'')
})
