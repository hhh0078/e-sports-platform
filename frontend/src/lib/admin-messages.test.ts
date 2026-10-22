/** 文件作用：运行管理员发送消息真实脚本，验证定向/广播确认、取消、防连点及跨账号隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { reactive,ref,watch } from 'vue'
import { executable,pageScript,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'

/** 模拟官方确认弹框与传输，页面仍执行实际领域发送方法。 */
function harness(role='admin'){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],confirmations:Array<{options:{title:string;content:string};resolve:(value:{confirm:boolean})=>void}>=[],state=reactive({ready:true,user:{id:'admin-A',role}}),lifecycle={show:()=>{},hide:()=>{},unload:()=>{}}
  let authToken='token-A',revision=1
  const context=vm.createContext({reactive,ref,watch,state,token:()=>authToken,sessionRevision:()=>revision,init:async()=>{},notify:(value:unknown)=>notices.push(value),uni:{showModal(options:{title:string;content:string}){return new Promise<{confirm:boolean}>(resolve=>confirmations.push({options,resolve}))}},onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context);vm.runInContext(executable(pageScript('manage/messages.vue'))+'\nglobalThis.page={form,authorized,sending,error,load,send,messageBody,reset}',context)
  return{page:context.page,requests,notices,confirmations,lifecycle,
    /** 读取确实发出的管理员消息请求。 */
    take(){const request=requests.find(item=>!item.taken&&item.method==='POST'&&item.path==='/admin/messages');assert.ok(request,'缺少平台消息发送请求');request.taken=true;return request},
    /** 模拟官方页面隐藏后管理员认证变化。 */
    switchAdmin(){lifecycle.hide();authToken='token-B';revision++;state.user={id:'admin-B',role:'admin'}}
  }
}
/** 推进确认、网络及账号watch到确定状态。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 从真实管理员初始化进入表单，不直接修改授权标志。 */
async function ready(){const subject=harness();subject.lifecycle.show();await flush();Object.assign(subject.page.form,{title:'平台公告',body:'请及时查看订单进度并保持友好沟通。'});return subject}

test('玩家与俱乐部管理员不能确认或发送平台消息',async()=>{for(const role of ['user','club_admin']){const subject=harness(role);subject.lifecycle.show();await flush();Object.assign(subject.page.form,{title:'越权',body:'越权正文'});await subject.page.send();assert.equal(subject.confirmations.length,0);assert.equal(subject.requests.length,0)}})
test('标题正文和接收人字节数校验先于官方确认',async()=>{const subject=await ready();for(const title of ['','字'.repeat(61)]){subject.page.form.title=title;await subject.page.send()}subject.page.form.title='合法标题';subject.page.form.body='';await subject.page.send();subject.page.form.body='文'.repeat(5001);await subject.page.send();subject.page.form.body='合法正文';subject.page.form.recipient_id='密'.repeat(43);await subject.page.send();assert.equal(subject.confirmations.length,0);assert.equal(subject.requests.length,0)})
test('空接收人明确广播范围，取消不发请求且保留表单',async()=>{const subject=await ready(),sending=subject.page.send();assert.equal(subject.confirmations[0].options.title,'确认全站广播');assert.match(subject.confirmations[0].options.content,/全部有效用户/);subject.confirmations[0].resolve({confirm:false});await sending;assert.equal(subject.requests.length,0);assert.equal(subject.page.form.title,'平台公告');assert.equal(subject.page.sending.value,false)})
test('定向消息固定快照、防连点，并显示真实接收人数',async()=>{const subject=await ready();subject.page.form.recipient_id=' player ';const sending=subject.page.send();await subject.page.send();assert.equal(subject.confirmations.length,1);assert.match(subject.confirmations[0].options.content,/player/);subject.confirmations[0].resolve({confirm:true});await flush();const request=subject.take();assert.equal(request.body.recipient_id,'player');assert.equal(request.body.title,'平台公告');assert.equal(subject.requests.length,1);request.resolve({recipient_count:1});await sending;assert.equal(subject.page.form.title,'');assert.match(String(subject.notices.at(-1)),/1位用户/);assert.match(String(subject.notices.at(-1)),/不代表已读/)})
test('确认广播请求省略空接收人且零接收人数不伪装成功送达',async()=>{const subject=await ready(),sending=subject.page.send();subject.confirmations[0].resolve({confirm:true});await flush();const request=subject.take();assert.equal('recipient_id'in request.body,false);request.resolve({recipient_count:0});await sending;assert.match(String(subject.notices.at(-1)),/0位用户/)})
test('确认框打开后换账号，即使确认也不能继续发送',async()=>{const subject=await ready(),sending=subject.page.send();subject.switchAdmin();await flush();subject.lifecycle.show();await flush();subject.page.form.title='新账号草稿';subject.confirmations[0].resolve({confirm:true});await sending;assert.equal(subject.requests.length,0);assert.equal(subject.page.form.title,'新账号草稿')})
test('旧账号发送迟到响应不能清空新草稿或弹出成功消息',async()=>{const subject=await ready(),sending=subject.page.send();subject.confirmations[0].resolve({confirm:true});await flush();const request=subject.take();subject.switchAdmin();await flush();subject.lifecycle.show();await flush();subject.page.form.title='新账号草稿';request.resolve({recipient_count:32});await sending;assert.equal(subject.page.form.title,'新账号草稿');assert.equal(subject.notices.length,0)})
test('发送失败保留正文可重试，取消修改不发请求',async()=>{const subject=await ready(),sending=subject.page.send();subject.confirmations[0].resolve({confirm:true});await flush();const error={message:'暂时无法发送'};subject.take().reject(error);await sending;assert.equal(subject.notices.at(-1),error);assert.ok(subject.page.form.body);subject.page.reset();assert.equal(subject.page.form.body,'');assert.equal(subject.requests.length,1)})
