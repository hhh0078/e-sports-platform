/** 文件作用：验证身份证年龄边界、实时身份预检、首次提醒及跨账号核验隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {reactive} from 'vue'
import {validateIdentityInput} from './identity-card'
import {executable} from './sfc-test'

/** 生成仅供离线边界测试的格式有效证件号码，不调用第三方接口。 */
function fixture(birthday:string):string{const raw='110101'+birthday+'001',weights=[7,9,10,5,8,4,2,1,6,3,7,9,10,5,8,4,2];return raw+'10X98765432'[weights.reduce((sum,value,index)=>sum+Number(raw[index])*value,0)%11]}

test('上海时间满18岁当天允许核验，生日前一天及未来日期拒绝',()=>{
  const now=new Date('2026-10-27T16:00:00Z')
  assert.equal(validateIdentityInput('测试姓名',fixture('20081028'),true,now).consent,true)
  assert.throws(()=>validateIdentityInput('测试姓名',fixture('20081029'),true,now),/您未满18岁/)
  assert.throws(()=>validateIdentityInput('测试姓名',fixture('20271028'),true,now),/不能晚于今天/)
})
test('校验真实日期、身份证校验位和主动同意，规范化小写x',()=>{
  const now=new Date('2026-10-28T00:00:00Z'),card=fixture('19930228')
  assert.throws(()=>validateIdentityInput('测试姓名',fixture('20000230'),true,now),/出生日期无效/)
  assert.throws(()=>validateIdentityInput('测试姓名',card.slice(0,17)+(card[17]==='0'?'1':'0'),true,now),/校验失败/)
  assert.throws(()=>validateIdentityInput('测试姓名',card,false,now),/同意/)
  assert.throws(()=>validateIdentityInput('',card,true,now),/真实姓名/)
  assert.throws(()=>validateIdentityInput('张',card,true,now),/2—50/)
  assert.throws(()=>validateIdentityInput('张'.repeat(51),card,true,now),/2—50/)
  assert.throws(()=>validateIdentityInput('测试姓名',card.slice(0,14)+'000'+card[17],true,now),/顺序码/)
  const normalized=validateIdentityInput(' 测试姓名 ',card.toLowerCase(),true,now)
  assert.equal(normalized.name,'测试姓名');assert.equal(normalized.idcard,card)
})
test('闰日生日在非闰年三月一日满18岁',()=>{
  const card=fixture('20080229')
  assert.throws(()=>validateIdentityInput('测试姓名',card,true,new Date('2026-02-28T00:00:00Z')),/未满18岁/)
  assert.equal(validateIdentityInput('测试姓名',card,true,new Date('2026-03-01T00:00:00Z')).idcard,card)
})

/** 直接执行生产身份状态模块，由测试控制服务端返回顺序和设备存储。 */
function harness(){
  const storage=new Map<string,unknown>(),reads:Array<{resolve:(value:unknown)=>void;reject:(error:unknown)=>void}>=[],routes:string[]=[]
  let revision=0,currentToken='token-A'
  const state=reactive({ready:true,user:{id:'A',is_minor:true} as {id:string;is_minor:boolean}|null})
  const context=vm.createContext({reactive,state,sessionRevision:()=>revision,token:()=>currentToken,errorMessage:(error:unknown)=>error instanceof Error?error.message:String(error),go:(path:string)=>routes.push(path),getIdentityStatus:()=>new Promise((resolve,reject)=>reads.push({resolve,reject})),uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:unknown)=>storage.set(key,value)}})
  vm.runInContext(executable(readFileSync(new URL('./identity.ts',import.meta.url),'utf8'))+'\nglobalThis.subject={identityGate,requireIdentity,requestIdentityReminder,markIdentityPresented,acceptIdentityResult,finishIdentity,isCurrentIdentityGate}',context)
  return{subject:context.subject,state,storage,reads,routes,
    /** 登录修订、令牌与账号一起变化，模拟真实切号。 */
    switchAccount(){revision++;currentToken='token-B';state.user={id:'B',is_minor:true}},
    /** 切换为访客会话，首次提示仍使用同一本机标记。 */
    guest(){revision++;currentToken='';state.user=null},
  }
}
/** 推进预检返回后的微任务，不使用真实网络或定时等待。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}

test('每次交易发起均重新GET，只有false与verified同时满足才放行',async()=>{
  const h=harness(),first=h.subject.requireIdentity();assert.equal(h.reads.length,1);h.reads[0].resolve({is_minor:false,verified:true});assert.equal(await first,true)
  const second=h.subject.requireIdentity();assert.equal(h.reads.length,2);h.reads[1].resolve({is_minor:false,verified:false});await flush();assert.equal(h.subject.identityGate.visible,true);assert.equal(h.state.user?.is_minor,true);h.subject.finishIdentity();assert.equal(await second,false)
  const third=h.subject.requireIdentity();h.reads[2].resolve({is_minor:true,verified:true});await flush();h.subject.finishIdentity();assert.equal(await third,false)
})
test('预检错误在身份弹框展示，不能凭已有客户端成年标记放行',async()=>{
  const h=harness();h.state.user!.is_minor=false
  const pending=h.subject.requireIdentity();h.reads[0].reject(new Error('身份服务暂不可用'));await flush()
  assert.equal(h.subject.identityGate.error,'身份服务暂不可用');assert.equal(h.subject.identityGate.visible,true);h.subject.finishIdentity();assert.equal(await pending,false)
})
test('首次提醒仅在实际展示后记已读，关闭排队状态仍可重新安排',()=>{
  const h=harness();h.subject.requestIdentityReminder();assert.equal(h.storage.size,0)
  h.subject.finishIdentity();h.subject.requestIdentityReminder();assert.equal(h.subject.identityGate.visible,true)
  h.subject.markIdentityPresented(h.subject.identityGate.epoch);assert.equal(h.storage.get('jijie_identity_reminded'),true)
  h.subject.finishIdentity();h.subject.requestIdentityReminder();assert.equal(h.subject.identityGate.visible,false)
  h.switchAccount();h.subject.requestIdentityReminder();assert.equal(h.subject.identityGate.visible,false)
})
test('已核验用户不弹首次提醒，访客看过后登录也不再提醒',()=>{
  const h=harness();h.state.user!.is_minor=false;h.subject.requestIdentityReminder();assert.equal(h.subject.identityGate.visible,false)
  h.guest();h.subject.requestIdentityReminder();h.subject.markIdentityPresented(h.subject.identityGate.epoch);assert.equal(h.storage.get('jijie_identity_reminded'),true)
  h.subject.finishIdentity();h.switchAccount();h.subject.requestIdentityReminder();assert.equal(h.subject.identityGate.visible,false)
})
test('旧账号预检与核验返回不能放行新账号，旧关闭事件不能结束新弹框',async()=>{
  const h=harness(),old=h.subject.requireIdentity();h.switchAccount();h.reads[0].resolve({is_minor:false,verified:true});assert.equal(await old,false);assert.equal(h.state.user?.is_minor,true)
  const current=h.subject.requireIdentity();h.reads[1].resolve({is_minor:true,verified:false});await flush();const epoch=h.subject.identityGate.epoch
  assert.equal(h.subject.acceptIdentityResult({is_minor:false,verified:true},epoch-1),false)
  h.subject.finishIdentity(true,epoch-1);assert.equal(h.subject.identityGate.visible,true)
  assert.equal(h.subject.acceptIdentityResult({is_minor:false,verified:true},epoch),true);h.subject.finishIdentity(true,epoch);assert.equal(await current,true)
})
