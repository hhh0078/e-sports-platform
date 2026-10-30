/** 文件作用：统一微信付款及商家转账确认，原生成功仅代表操作完成，资金结果仍需服务端查单。 */
import type { PaymentParams, WechatPaymentParams } from '../types/models'
import type { TransferConfirmation } from '../types/wallet'
import type { PaymentPlatform } from '../types/commerce'
import { refreshWechatPaymentSession } from '../api/auth.api'
import { state,sessionRevision } from './store'
import { token } from '../api/client'
import { confirmOfficialTransfer } from './official-transfer'
import { beginFinancialActivity } from './update-manager'
/** 微信虚拟支付原生能力的最小类型，不接收任何服务端会话密钥。 */
interface VirtualRuntime { canIUse?:(name:string)=>boolean; requestVirtualPayment?:(options:Record<string,unknown>)=>void; login?:(options:{success:(result:{code:string})=>void;fail:(error:unknown)=>void})=>void }
/** 公众号内置浏览器提供的普通微信支付桥。 */
interface PaymentBridge { invoke:(name:string,params:WechatPaymentParams,callback:(result:{err_msg?:string})=>void)=>void }
/** 返回用于服务端报价的平台，最终扣款仍以官方结果为准。 */
export function paymentPlatform():PaymentPlatform{return /ios/i.test(String(uni.getSystemInfoSync().platform||''))?'ios':'android'}
/** 拉起虚拟支付前刷新服务端签名会话，密钥始终只留在服务端。 */
export async function prepareVirtualPayment(required=false):Promise<void>{
  if(state.devMode||!required&&state.commerce?.mode!=='mini_program')return
  const userID=state.user?.id,revision=sessionRevision(),savedToken=token()
  const runtime=(globalThis as unknown as {wx?:VirtualRuntime}).wx
  if(!runtime?.login||!required&&(!runtime.requestVirtualPayment||runtime.canIUse&&!runtime.canIUse('requestVirtualPayment')))throw new Error(required?'请使用原小程序买家账号刷新微信身份并同步原资金单':'请在支持虚拟支付的新版微信小程序内付款')
  const result=await new Promise<{code:string}>((resolve,reject)=>runtime.login!({success:resolve,fail:reject}))
  if(!result.code)throw new Error('微信支付会话刷新失败，请重试')
  if(userID!==state.user?.id||revision!==sessionRevision()||savedToken!==token())throw new Error('账号已切换，请在当前账号重新操作')
  await refreshWechatPaymentSession(result.code)
}
/** 在公众号微信浏览器内调用普通JSAPI支付，能力不足或取消明确返回错误。 */
function requestOfficialPayment(params:WechatPaymentParams):Promise<void>{
  const browser=globalThis as unknown as {WeixinJSBridge?:PaymentBridge;document?:Document;navigator?:{userAgent?:string}}
  if(!params.appId)return Promise.reject(new Error('公众号支付AppID缺失，请同步原付款单'))
  if(!/micromessenger/i.test(browser.navigator?.userAgent||''))return Promise.reject(new Error('请在微信内打开公众号页面完成支付'))
  return new Promise<void>((resolve,reject)=>{
    let timer:ReturnType<typeof setTimeout>|undefined
    /** 等待微信桥就绪后提交当前临时参数，完成即移除监听。 */
    function invoke():void{if(!browser.WeixinJSBridge)return;browser.document?.removeEventListener('WeixinJSBridgeReady',invoke);if(timer)clearTimeout(timer);browser.WeixinJSBridge.invoke('getBrandWCPayRequest',params,result=>{if(result.err_msg==='get_brand_wcpay_request:ok')resolve();else reject(new Error(result.err_msg?.includes('cancel')?'已取消微信支付':'微信支付未确认，请同步原单'))})}
    if(browser.WeixinJSBridge){invoke();return}
    browser.document?.addEventListener('WeixinJSBridgeReady',invoke)
    timer=setTimeout(()=>{browser.document?.removeEventListener('WeixinJSBridgeReady',invoke);reject(new Error('微信支付能力尚未就绪，请重新打开页面'))},8000)
  })
}
/** 调用官方小程序支付；取消以可识别错误返回，不伪造到账。 */
export function requestWechatPayment(params:PaymentParams):Promise<void>{
  const finish=beginFinancialActivity()
  const task=new Promise<void>((resolve,reject)=>{try{
    if('mode'in params){const runtime=(globalThis as unknown as {wx?:VirtualRuntime}).wx;if(!runtime?.requestVirtualPayment){reject(new Error('当前微信版本不支持虚拟支付'));return}runtime.requestVirtualPayment({...params,success:()=>resolve(),fail:reject});return}
    if(typeof document!=='undefined'){requestOfficialPayment(params).then(resolve,reject);return}
    uni.requestPayment({provider:'wxpay',...params,success:()=>resolve(),fail:reject})
  }catch(error){reject(error)}})
  return task.finally(finish)
}
/** 新版收款能力的最小官方接口，避免 SDK 尚未更新时使用无类型对象。 */
interface TransferRuntime { canIUse:(name:string)=>boolean; requestMerchantTransfer?:(options:{mchId:string;appId:string;package:string;success:()=>void;fail:(error:unknown)=>void})=>void }
/** 拉起微信官方收款确认页，兼容能力不足时明确提示升级微信。 */
export function confirmWechatTransfer(confirmation:TransferConfirmation):Promise<void>{
  if(typeof document!=='undefined'){const finish=beginFinancialActivity();return confirmOfficialTransfer(confirmation).finally(finish)}
  const runtime=(globalThis as unknown as {wx?:TransferRuntime}).wx
  if(!runtime?.canIUse('requestMerchantTransfer')||!runtime.requestMerchantTransfer)return Promise.reject(new Error('请在新版微信小程序中确认收款'))
  const finish=beginFinancialActivity()
  return new Promise<void>((resolve,reject)=>{try{runtime.requestMerchantTransfer!({mchId:confirmation.mch_id,appId:confirmation.app_id,package:confirmation.package_info,success:()=>{finish();resolve()},fail:error=>{finish();reject(error)}})}catch(error){finish();reject(error)}})
}
