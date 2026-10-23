/** 文件作用：运行AI管理真实脚本，验证秘密清理、真实连接边界、配置长度和跨账号隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { reactive,ref,computed,watch } from 'vue'
import { executable,pageScript,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import type { AISettings } from '../types/ai'

/** 仅替换网络与生命周期，执行生产页面和实际领域API。 */
function harness(role='admin'){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],state=reactive({ready:true,user:{id:'admin-A',role}}),lifecycle={show:()=>{},hide:()=>{},unload:()=>{}}
  let authToken='token-A',revision=1
  const context=vm.createContext({reactive,ref,computed,watch,state,token:()=>authToken,sessionRevision:()=>revision,init:async()=>{},notify:(value:unknown)=>notices.push(value),onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context);vm.runInContext(executable(pageScript('manage/ai.vue'))+'\nglobalThis.page={form,status,apiKey,loaded,busy,testing,error,testError,connection,modeLabel,load,save,settingsBody,testConnection,restore}',context)
  return{page:context.page,requests,notices,lifecycle,
    /** 读取真实API发出的待处理请求。 */
    take(method='GET',route='/admin/ai-settings'){const request=requests.find(item=>!item.taken&&item.method===method&&item.path===route);assert.ok(request,'缺少'+method+route);request.taken=true;return request},
    /** 模拟隐藏、认证修订及账号同时切换。 */
    switchAdmin(){lifecycle.hide();authToken='token-B';revision++;state.user={id:'admin-B',role:'admin'}}
  }
}
/** 排空异步页面初始化与Vue身份监听。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 提供后端真实公开配置结构，不生成有效外部密钥。 */
function configuration(hasKey=false,model='qwen3.8-flash'):AISettings{return{base_url:'https://maas.qianwenaiapi.com/compatible-mode/v1',model,prompt:'仅根据平台审核规则判断服务内容是否存在风险，并给出完整原因。',has_api_key:hasKey,ready:hasKey,mode:hasKey?'ai':'local_simulation',configuration_notice:hasKey?'配置完整仍需真实连接测试':'本地模拟不调用真实模型'}}
/** 完成页面通过领域API发出的首次读取。 */
async function ready(hasKey=false){const subject=harness();subject.lifecycle.show();await flush();subject.take().resolve(configuration(hasKey));await flush();return subject}

test('玩家与俱乐部管理员不能读取、保存或测试AI配置',async()=>{for(const role of ['user','club_admin']){const subject=harness(role);subject.lifecycle.show();await flush();await subject.page.save();await subject.page.testConnection();assert.equal(subject.requests.length,0)}})
test('未配置AI明确显示人工审核，连接测试不发请求也不阻塞配置读取',async()=>{const subject=harness();subject.lifecycle.show();await flush();subject.take().resolve({...configuration(),mode:'manual',configuration_notice:'未配置AI，图文正常保存后等待人工审核'});await flush();assert.equal(subject.page.modeLabel.value,'人工审核');assert.equal(subject.page.loaded.value,true);await subject.page.testConnection();assert.equal(subject.requests.length,1)})
test('只接受公开配置，隐藏与取消立即清空本次API Key',async()=>{const subject=harness();subject.lifecycle.show();await flush();subject.take().resolve({...configuration(),api_key:'不可回显的额外字段'});await flush();assert.deepEqual(Object.keys(subject.page.form).sort(),['base_url','model','prompt']);assert.equal(subject.page.modeLabel.value,'本地模拟');subject.page.apiKey.value='本次草稿';subject.page.restore();assert.equal(subject.page.apiKey.value,'');subject.page.apiKey.value='离页草稿';subject.lifecycle.hide();assert.equal(subject.page.apiKey.value,'')})
test('保存只发送约定字段，完成后擦除请求和页面密钥',async()=>{const subject=await ready();subject.page.apiKey.value='not-a-real-key';subject.page.form.model=' qwen3.8-flash ';const saving=subject.page.save();const request=subject.take('PATCH');assert.deepEqual(Object.keys(request.body).sort(),['api_key','base_url','model','prompt']);assert.equal(request.body.api_key,'not-a-real-key');assert.equal(request.body.model,'qwen3.8-flash');request.resolve(configuration(true));await saving;assert.equal(request.body.api_key,'');assert.equal(subject.page.apiKey.value,'');assert.equal(subject.page.status.has_api_key,true);assert.equal(subject.page.modeLabel.value,'已配置AI')})
test('接口URL、提示词和多字节秘密遵守真实后端边界',async()=>{const subject=await ready(),page=subject.page;for(const url of ['http://example.com/v1','https://example.com:8443/v1','https://user@example.com/v1','https://example.com/v1?key=x','https://example.com/v1#x']){page.form.base_url=url;assert.throws(()=>page.settingsBody(),/HTTPS/)}page.form.base_url=configuration().base_url;for(const prompt of ['短'.repeat(19),'长'.repeat(12001)]){page.form.prompt=prompt;assert.throws(()=>page.settingsBody(),/20—12000/)}page.form.prompt='规'.repeat(12000);page.apiKey.value='密'.repeat(1366);assert.throws(()=>page.settingsBody(),/4096/);page.apiKey.value='key\n';assert.throws(()=>page.settingsBody(),/换行/);page.apiKey.value='';assert.equal(page.settingsBody().prompt.length,12000)})
test('无密钥的本地模拟及未保存草稿不发真实连接请求',async()=>{const local=await ready();await local.page.testConnection();assert.equal(local.requests.length,1);assert.equal(local.page.connection.value,null);const configured=await ready(true);configured.page.form.prompt+='未保存';await configured.page.testConnection();assert.equal(configured.requests.length,1);assert.match(String(configured.notices.at(-1)),/先保存/)})
test('连接测试只请求后端已保存配置，防连点且保留完整真实原因',async()=>{const subject=await ready(true),reason='安全测试文本已完成真实判定。'.repeat(20);const testing=subject.page.testConnection();await subject.page.testConnection();const request=subject.take('POST','/admin/ai-settings/test');assert.deepEqual(Object.keys(request.body),[]);assert.equal(subject.requests.filter(item=>item.method==='POST').length,1);request.resolve({mode:'ai',decision:'allow',reason});await testing;assert.equal(subject.page.connection.value.reason,reason);assert.equal(subject.page.testing.value,false)})
test('连接失败保留完整错误，模拟返回不能被标为真实响应',async()=>{const subject=await ready(true),failure={message:'真实上游不可用，完整原因应显示。'.repeat(15)};const first=subject.page.testConnection();subject.take('POST','/admin/ai-settings/test').reject(failure);await first;assert.equal(subject.page.testError.value,failure.message);assert.equal(subject.notices.at(-1),failure);const second=subject.page.testConnection();subject.take('POST','/admin/ai-settings/test').resolve({mode:'local_simulation',decision:'allow',reason:'模拟'});await second;assert.equal(subject.page.connection.value,null);assert.match(subject.page.testError.value,/未返回真实AI响应/)})
test('旧管理员保存返回不覆盖新配置，也不清空新账号正在输入的秘密',async()=>{const subject=await ready();subject.page.apiKey.value='old-key';const saving=subject.page.save(),request=subject.take('PATCH');subject.switchAdmin();await flush();subject.lifecycle.show();await flush();subject.take().resolve(configuration(true,'new-model'));await flush();subject.page.apiKey.value='new-admin-unsaved-key';request.resolve(configuration(true,'old-model'));await saving;assert.equal(subject.page.form.model,'new-model');assert.equal(subject.page.apiKey.value,'new-admin-unsaved-key');assert.equal(request.body.api_key,'')})
test('旧管理员测试返回不回填新账号真实连接结果',async()=>{const subject=await ready(true);const testing=subject.page.testConnection(),request=subject.take('POST','/admin/ai-settings/test');subject.switchAdmin();await flush();subject.lifecycle.show();await flush();subject.take().resolve(configuration());await flush();request.resolve({mode:'ai',decision:'allow',reason:'旧账号结果'});await testing;assert.equal(subject.page.connection.value,null);assert.equal(subject.page.modeLabel.value,'本地模拟')})
