/** 文件作用：执行客服组件和管理员配置真实脚本，模拟原生预览拨号复制，核验客服归属与并发边界。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed,effectScope,nextTick,reactive,ref,watch } from 'vue'
import { executable,injectBusinessAPIs,pageScript } from './sfc-test'
import type { DeferredRequest } from './sfc-test'

const componentSource=readFileSync(new URL('../components/ContactSupport.vue',import.meta.url),'utf8')
const officialPopupSource=readFileSync(new URL('../../node_modules/@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue',import.meta.url),'utf8')
/** 排空Vue侦听与实际网络await，不依赖动画等待时间。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 执行已安装官方弹层的真实开关方法，手动推进其300ms定时器，禁止同步替换关闭动画。 */
function officialPopup(changed:(event:{show:boolean})=>void){
  const timers=new Map<number,{at:number;callback:()=>void}>(),delayedCloseEvents:Array<{show:boolean}>=[]
  let now=0,timerID=0,delayCloseEvents=false,openCount=0,closeCount=0
  const openBody=officialPopupSource.match(/\n\t\t\topen\(direction\) \{([\s\S]*?)\n\t\t\t\},/)?.[1]
  const closeBody=officialPopupSource.match(/\n\t\t\tclose\(type\) \{([\s\S]*?)\n\t\t\t\},/)?.[1]
  assert.ok(openBody);assert.ok(closeBody)
  const methods=vm.runInNewContext('({open(direction){'+openBody+'},close(type){'+closeBody+'}})',{
    console,
    setTimeout:(callback:()=>void,delay:number)=>{const id=++timerID;timers.set(id,{at:now+delay,callback});return id},
    clearTimeout:(id:number)=>timers.delete(id),
  }) as {open:(direction?:string)=>void;close:()=>void}
  const instance=reactive({showPopup:false,showTrans:false,timer:0,type:'bottom',config:{bottom:'bottom'},
    /** 只替代与本测试无关的布局计算，保留官方开关方法和change事件顺序。 */
    bottom(){this.showPopup=true;this.showTrans=true},
    /** 可模拟微信桥延迟交付旧关闭事件，事件本身仍来自官方close方法。 */
    $emit(_name:string,event:{show:boolean}){if(!event.show&&delayCloseEvents)delayedCloseEvents.push(event);else changed(event)},
    /** 记录官方方法实际执行次数，阻止动画期间重复close延长计时。 */
    open(direction?:string){openCount++;methods.open.call(this,direction)},
    /** 官方close保留showPopup直至其真实定时回调执行。 */
    close(){closeCount++;methods.close.call(this)},
  })
  return{instance,get openCount(){return openCount},get closeCount(){return closeCount},
    /** 仅控制事件交付时机，不更改官方showPopup延后关闭行为。 */
    delayClosingEvents(){delayCloseEvents=true},
    /** 推进真实关闭定时器并排空Vue更新，便于断言299ms与300ms边界。 */
    async advance(milliseconds:number){now+=milliseconds;for(const[id,timer]of timers){if(timer.at<=now){timers.delete(id);timer.callback()}}await flush()},
    /** 按原顺序交付此前官方close产生的事件。 */
    async deliverClosingEvents(){delayCloseEvents=false;for(const event of delayedCloseEvents.splice(0))changed(event);await flush()},
  }
}
/** 构建真实客服组件，仅用spy替代可能联系外部的官方能力，测试不拨号不发送消息。 */
function supportHarness(t:TestContext,club:Record<string,unknown>|null=null){
  const notices:unknown[]=[],calls:Array<{kind:string;options:Record<string,unknown>}>=[]
  const state=reactive({support_qr_url:'/api/media/platform-qr',support_phone:'020 8888 6666',support_wechat:'platform_support'})
  const props=reactive({club,active:true,compact:false,label:''}),revision=ref(1),hooks={show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({computed,nextTick,ref,watch,state,sessionRevision:()=>revision.value,assetUrl:(value:string)=>value,notify:(value:unknown)=>notices.push(value),errorMessage:(value:unknown)=>String(value),defineProps:()=>props,withDefaults:(value:object)=>value,defineExpose:()=>{},
    uni:{previewImage:(options:Record<string,unknown>)=>calls.push({kind:'preview',options}),makePhoneCall:(options:Record<string,unknown>)=>calls.push({kind:'phone',options}),setClipboardData:(options:Record<string,unknown>)=>calls.push({kind:'copy',options})},
    onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},
  })
  const script=componentSource.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(script)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(script)+'\nglobalThis.component={open,close,toggleQR,previewQR,callPhone,copyWechat,wechatHint,opened,qrOpened,phone,wechat,qrURL,popup,popupChanged}',context))
  const popup=officialPopup(context.component.popupChanged);context.component.popup.value=popup.instance
  t.after(()=>{hooks.unload();scope.stop()})
  return{component:context.component,popup,props,state,revision,hooks,calls,notices}
}
/** 构建真实客服设置页和具名API，保留可控制的权限、上传及迟到请求。 */
function adminHarness(t:TestContext,role='admin'){
  const pending:DeferredRequest[]=[],uploads:Array<{purpose:string;resolve:(value:string)=>void;reject:(error:unknown)=>void}>=[],notices:unknown[]=[],refreshes:boolean[]=[]
  const state=reactive({ready:true,user:{id:'admin-A',role}}),hooks={show:()=>{},hide:()=>{},unload:()=>{}};let authToken='token-A',revision=0
  const context=vm.createContext({reactive,ref,watch,state,token:()=>authToken,sessionRevision:()=>revision,init:async(force=false)=>{refreshes.push(force)},notify:(value:unknown)=>notices.push(value),assetUrl:(value:string)=>value,
    chooseUploadImage:(purpose:string)=>new Promise<string>((resolve,reject)=>uploads.push({purpose,resolve,reject})),uni:{previewImage:()=>{}},
    onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},
    api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>pending.push({path,method,body,resolve,reject,taken:false}))},
  })
  injectBusinessAPIs(context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('manage/support.vue'))+'\nglobalThis.page={load,save,uploadQR,form,formError,error,loaded,uploading}',context))
  t.after(()=>{hooks.unload();scope.stop()})
  return{page:context.page,hooks,pending,uploads,notices,refreshes,
    /** 明确提取实际发出的后台请求。 */
    take(path:string,method='GET'){const request=pending.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(request,'缺少请求 '+method+' '+path);request.taken=true;return request},
    /** 隐藏后变更管理员，令牌和会话版本同时变化。 */
    switchAdmin(){hooks.hide();authToken='token-B';revision++;state.user={id:'admin-B',role:'admin'}},
  }
}
/** 返回带无关站名公告的真实设置对象，测试保存不得覆盖这些字段。 */
function settings(label='A'){return{site_name:'必须保留的平台名称',announcement:'并发变化的公告',support_qr_url:'/api/media/qr-'+label,support_phone:'02088886666',support_wechat:'support_'+label}}
/** 完成当前管理员的设置读取。 */
async function readyAdmin(t:TestContext){const subject=adminHarness(t);subject.hooks.show();await flush();subject.take('/admin/settings').resolve(settings());await flush();return subject}

test('平台客服使用官方contact按钮，打开弹层不主动联系或创建聊天室',async t=>{
  const subject=supportHarness(t);subject.component.open();await flush()
  assert.equal(subject.calls.length,0)
  assert.match(componentSource,/open-type="contact"/)
  assert.doesNotMatch(componentSource,/createConversation|createChat|go\(/)
  assert.equal(subject.component.phone.value,'020 8888 6666')
})

test('空俱乐部客服不能回退到平台或技术支持号码',async t=>{
  const subject=supportHarness(t,{id:'club-A',name:'测试俱乐部'})
  subject.component.open();await flush();subject.component.callPhone();subject.component.toggleQR()
  assert.equal(subject.component.phone.value,'');assert.equal(subject.component.qrURL.value,'');assert.equal(subject.component.wechat.value,'')
  assert.equal(subject.calls.length,0);assert.match(String(subject.notices[0]),/暂未配置/)
  assert.doesNotMatch(componentSource,/17607080935/)
})

test('二维码预览电话和微信复制仅由显式操作触发，隐藏后旧提示失效',async t=>{
  const subject=supportHarness(t)
  subject.component.open();await flush();subject.component.toggleQR();subject.component.previewQR();subject.component.callPhone();subject.component.copyWechat()
  assert.equal(subject.calls[0].kind,'preview');assert.equal(subject.calls[0].options.current,'/api/media/platform-qr')
  assert.equal(subject.calls[1].kind,'phone');assert.equal(subject.calls[1].options.phoneNumber,'02088886666')
  assert.equal(subject.calls[2].kind,'copy');assert.equal(subject.calls[2].options.data,'platform_support')
  subject.hooks.hide();(subject.calls[2].options.success as ()=>void)();subject.component.callPhone()
  assert.equal(subject.notices.length,0);assert.equal(subject.calls.length,3)
})

test('俱乐部或配置切换后关闭原联系弹层，不能拨打旧号码',async t=>{
  const subject=supportHarness(t,{id:'club-A',name:'A',contact_phone:'01011112222'})
  subject.component.open();await flush();subject.props.club={id:'club-B',name:'B',contact_phone:'01033334444'};await flush()
  assert.equal(subject.component.opened.value,false)
  subject.component.callPhone();assert.equal(subject.calls.length,0)
})

test('客服关闭动画期间立即重开，等待官方300ms结束后只重开一次',async t=>{
  const subject=supportHarness(t);subject.component.open();await flush();subject.component.close();await flush()
  assert.equal(subject.popup.instance.showPopup,true);assert.equal(subject.popup.instance.showTrans,false)
  subject.component.open();await flush();await subject.popup.advance(299)
  assert.equal(subject.component.opened.value,true);assert.equal(subject.popup.openCount,1);assert.equal(subject.popup.closeCount,1)
  await subject.popup.advance(1)
  assert.equal(subject.popup.instance.showPopup,true);assert.equal(subject.popup.instance.showTrans,true);assert.equal(subject.popup.openCount,2);assert.equal(subject.popup.closeCount,1)
  assert.equal(subject.calls.length,0)
})

test('官方旧关闭事件迟到不能撤销动画结束后的新打开意图',async t=>{
  const subject=supportHarness(t);subject.component.open();await flush();subject.popup.delayClosingEvents()
  subject.component.close();await flush();subject.component.open();await flush();await subject.popup.advance(300)
  assert.equal(subject.popup.openCount,2);await subject.popup.deliverClosingEvents()
  assert.equal(subject.component.opened.value,true);assert.equal(subject.popup.instance.showTrans,true);assert.equal(subject.popup.closeCount,1)
})

test('客服遮罩主动关闭不重开，动画期间禁用原生动作且不重复调用close',async t=>{
  const subject=supportHarness(t);subject.component.open();await flush();subject.component.toggleQR();subject.popup.instance.close();await flush()
  assert.equal(subject.component.opened.value,false);assert.equal(subject.component.qrOpened.value,false)
  subject.component.callPhone();subject.component.previewQR();subject.component.copyWechat();await subject.popup.advance(300)
  assert.equal(subject.popup.instance.showPopup,false);assert.equal(subject.popup.openCount,1);assert.equal(subject.popup.closeCount,1);assert.equal(subject.calls.length,0)
})

test('active切换关闭旧客服，重新启用必须用户打开并等待原动画结束',async t=>{
  const subject=supportHarness(t);subject.component.open();await flush();subject.props.active=false;await flush()
  subject.component.open();assert.equal(subject.component.opened.value,false);subject.props.active=true;await flush()
  assert.equal(subject.component.opened.value,false);subject.component.open();await flush();await subject.popup.advance(300)
  assert.equal(subject.component.opened.value,true);assert.equal(subject.popup.instance.showTrans,true);assert.equal(subject.popup.openCount,2);assert.equal(subject.popup.closeCount,1)
})

test('俱乐部变化关闭旧弹层，新组织重开保留当前联系方式并丢弃旧二维码状态',async t=>{
  const subject=supportHarness(t,{id:'club-A',contact_phone:'01011112222',contact_qr_url:'/api/media/qr-A'})
  subject.component.open();await flush();subject.component.toggleQR();subject.props.club={id:'club-B',contact_phone:'01033334444',contact_qr_url:'/api/media/qr-B'};await flush()
  assert.equal(subject.component.opened.value,false);assert.equal(subject.component.qrOpened.value,false)
  subject.component.open();await flush();await subject.popup.advance(300)
  assert.equal(subject.component.opened.value,true);assert.equal(subject.component.phone.value,'01033334444');assert.equal(subject.component.qrURL.value,'/api/media/qr-B')
  assert.equal(subject.popup.openCount,2);assert.equal(subject.popup.closeCount,1);assert.equal(subject.calls.length,0)
})

test('客服设置只PATCH客服三项，不覆盖站名公告，并刷新公开配置',async t=>{
  const subject=await readyAdmin(t)
  subject.page.form.support_phone=' 020 8888 6666 ';subject.page.form.support_wechat=' new_support '
  const saving=subject.page.save(),request=subject.take('/admin/settings','PATCH')
  assert.deepEqual(Object.keys(request.body).sort(),['support_phone','support_qr_url','support_wechat'])
  assert.equal(request.body.support_phone,'020 8888 6666');assert.equal(request.body.support_wechat,'new_support')
  request.resolve({...settings(),...request.body});await saving
  assert.ok(subject.refreshes.includes(true));assert.equal(subject.notices.length,1)
})

test('客服配置审核失败保留草稿，非法联系方式不发送请求',async t=>{
  const subject=await readyAdmin(t)
  subject.page.form.support_wechat='新微信号';await subject.page.save();assert.equal(subject.pending.filter(item=>item.method==='PATCH').length,0)
  subject.page.form.support_wechat='valid_wechat';const saving=subject.page.save(),request=subject.take('/admin/settings','PATCH')
  request.reject(new Error('图片或文字内容需要修改'));await saving
  assert.equal(subject.page.form.support_wechat,'valid_wechat');assert.match(subject.page.formError.value,/需要修改/)
})

test('原生选图隐藏返回不丢草稿，取消保留原图，换账号丢弃旧上传',async t=>{
  const subject=await readyAdmin(t),first=subject.page.uploadQR()
  assert.equal(subject.uploads[0].purpose,'support_qr');subject.hooks.hide();subject.hooks.show()
  subject.uploads[0].resolve('/api/media/new-qr');await first
  assert.equal(subject.page.form.support_qr_url,'/api/media/new-qr')
  const cancelled=subject.page.uploadQR();subject.uploads[1].reject({cancelled:true});await cancelled
  assert.equal(subject.page.form.support_qr_url,'/api/media/new-qr');assert.equal(subject.page.formError.value,'')
  const old=subject.page.uploadQR();subject.switchAdmin();subject.hooks.show();await flush();subject.take('/admin/settings').resolve(settings('B'));await flush()
  subject.uploads[2].resolve('/api/media/old-A');await old
  assert.equal(subject.page.form.support_qr_url,'/api/media/qr-B');assert.equal(subject.page.uploading.value,false)
})

test('普通用户不能读取或保存商户无关的后台客服配置',async t=>{
  const subject=adminHarness(t,'user');subject.hooks.show();await flush();await subject.page.save()
  assert.equal(subject.pending.length,0);assert.equal(subject.page.loaded.value,false)
})

test('旧管理员客服保存迟到不污染新管理员草稿',async t=>{
  const subject=await readyAdmin(t);subject.page.form.support_wechat='draft_A'
  const saving=subject.page.save(),old=subject.take('/admin/settings','PATCH')
  subject.switchAdmin();subject.hooks.show();await flush();subject.take('/admin/settings').resolve(settings('B'));await flush()
  old.resolve({...settings(),support_wechat:'old_A'});await saving
  assert.equal(subject.page.form.support_wechat,'support_B');assert.equal(subject.notices.length,0)
})
