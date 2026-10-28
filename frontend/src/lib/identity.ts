/** 文件作用：统一本机首次身份提示与每次交易的服务端身份预检，隔离账号和弹层代次。 */
import {reactive} from 'vue'
import {state,sessionRevision} from './store'
import {token,errorMessage} from '../api/client'
import {getIdentityStatus} from '../api/users.api'
import {go} from './format'

/** 身份弹层模式区分可跳过的首次提醒与当前交易必须完成的核验。 */
export type IdentityMode='entry'|'required'
export const identityGate=reactive({visible:false,busy:false,error:'',epoch:0,mode:'entry' as IdentityMode})
let owner:{id:string;token:string;revision:number}|null=null,pending:Promise<boolean>|undefined,resolvePending:((value:boolean)=>void)|undefined,accepted=false
const reminderKey='jijie_identity_reminded'
let reminderShown=false

/** 捕获本次真实账号，供核验请求与弹层关闭事件校验归属。 */
function session(){return{id:state.user?.id||'',token:token(),revision:sessionRevision()}}
/** 判断服务端响应仍属于当前账号及登录修订。 */
function same(value:ReturnType<typeof session>):boolean{return value.id===(state.user?.id||'')&&value.token===token()&&value.revision===sessionRevision()}
/** 存储异常只影响下次启动，本次已展示的提示仍不重复。 */
function reminded():boolean{if(reminderShown)return true;try{return uni.getStorageSync(reminderKey)===true}catch{return false}}
/** 当前账号与弹层代次共同隔离迟到请求和旧页面关闭事件。 */
export function isCurrentIdentityGate(epoch:number):boolean{return identityGate.visible&&epoch===identityGate.epoch&&!!owner&&same(owner)}
/** 打开独立代次并合并同一账号尚未结束的验证等待。 */
function open(mode:IdentityMode,error=''):Promise<boolean>{
  if(pending&&isCurrentIdentityGate(identityGate.epoch)){if(mode==='required')identityGate.mode='required';if(error)identityGate.error=error;return pending}
  if(pending)finishIdentity(false,identityGate.epoch)
  owner=session();accepted=false;identityGate.epoch++;identityGate.mode=mode;identityGate.busy=false;identityGate.error=error;identityGate.visible=true
  pending=new Promise(resolve=>{resolvePending=resolve});return pending
}
/** 仅已恢复会话且尚未真正展示过时安排首次提示；公开预览由承载组件排除。 */
export function requestIdentityReminder():void{
  if(!state.ready||token()&&!state.user||state.user?.is_minor===false||reminded())return
  if(identityGate.visible&&isCurrentIdentityGate(identityGate.epoch))return
  void open('entry')
}
/** 官方弹层实际显示后才记录首次提示，排队、隐藏或失败不提前标记。 */
export function markIdentityPresented(epoch:number):void{
  if(!isCurrentIdentityGate(epoch)||!owner)return
  reminderShown=true
  try{uni.setStorageSync(reminderKey,true)}catch{/* 本次已显示，设备存储失败不阻断身份核验。 */}
}
/** 仅接受当前账号服务端明确返回的成年且已核验结果。 */
export function acceptIdentityResult(result:{is_minor:boolean;verified:boolean},epoch:number):boolean{
  if(!isCurrentIdentityGate(epoch))return false
  accepted=result.is_minor===false&&result.verified===true
  if(state.user)state.user.is_minor=!accepted
  return accepted
}
/** 每次下单或发起付款都重新读取数据库状态，禁止依赖客户端缓存放行。 */
export async function requireIdentity():Promise<boolean>{
  if(!state.user){go('/pages/account/login');return false}
  const current=session()
  try{
    const result=await getIdentityStatus()
    if(!same(current))return false
    const verified=result.is_minor===false&&result.verified===true
    if(state.user)state.user.is_minor=!verified
    if(verified)return true
    return open('required')
  }catch(error){if(!same(current))return false;return open('required',errorMessage(error))}
}
/** 结束指定代次，取消不继续原交易，旧请求不能结束新账号的核验。 */
export function finishIdentity(verified=false,epoch=identityGate.epoch):void{
  if(epoch!==identityGate.epoch)return
  const result=verified&&accepted&&isCurrentIdentityGate(epoch),resolve=resolvePending
  identityGate.epoch++;identityGate.visible=false;identityGate.busy=false;identityGate.error='';owner=null;accepted=false;pending=undefined;resolvePending=undefined
  resolve?.(result)
}
