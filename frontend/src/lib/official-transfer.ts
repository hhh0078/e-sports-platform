/** 文件作用：在公众号微信浏览器通过官方JSSDK和微信桥确认商家转账，实际到账仍由服务端核验。 */
import { getOfficialSDKConfig } from '../api/auth.api'
import type { TransferConfirmation } from '../types/wallet'
/** 官方JSSDK本次使用的最小接口。 */
interface OfficialSDK { config:(options:Record<string,unknown>)=>void; ready:(callback:()=>void)=>void; error:(callback:(error:unknown)=>void)=>void; checkJsApi:(options:{jsApiList:string[];success:(result:{checkResult?:unknown})=>void;fail:(error:unknown)=>void})=>void }
/** H5微信桥的转账确认能力。 */
interface OfficialTransferWindow { wx?:Partial<OfficialSDK>; WeixinJSBridge?:{invoke:(name:string,params:{mchId:string;appId:string;package:string},callback:(result:{err_msg?:string})=>void)=>void} }
let pendingSDK:Promise<OfficialSDK>|undefined
/** 外部SDK必须提供本次实际使用的全部方法，不能按全局变量存在猜测可用。 */
function hasOfficialSDK(value:Partial<OfficialSDK>|undefined):value is OfficialSDK{return typeof value?.config==='function'&&typeof value.ready==='function'&&typeof value.error==='function'&&typeof value.checkJsApi==='function'}
/** 复用已加载的官方SDK，加载失败允许用户主动重试。 */
function loadOfficialSDK():Promise<OfficialSDK>{
  const runtime=globalThis as unknown as OfficialTransferWindow
  if(hasOfficialSDK(runtime.wx))return Promise.resolve(runtime.wx)
  if(pendingSDK)return pendingSDK
  pendingSDK=new Promise<OfficialSDK>((resolve,reject)=>{
    const script=document.createElement('script');script.src='https://res.wx.qq.com/open/js/jweixin-1.6.0.js';script.async=true
    const timer=setTimeout(()=>{script.remove();pendingSDK=undefined;reject(new Error('微信收款确认组件加载超时，请重试'))},10000)
    script.onload=()=>{clearTimeout(timer);if(hasOfficialSDK(runtime.wx))resolve(runtime.wx);else{pendingSDK=undefined;reject(new Error('微信收款确认组件不可用'))}}
    script.onerror=()=>{clearTimeout(timer);script.remove();pendingSDK=undefined;reject(new Error('微信收款确认组件加载失败，请重试'))}
    document.head.appendChild(script)
  })
  return pendingSDK
}
/** 校验SDK就绪与当前微信的转账确认能力，不从页面回调推断资金到账。 */
async function prepareOfficialTransfer(expectedAppID:string):Promise<OfficialSDK>{
  if(!/micromessenger/i.test(navigator.userAgent))throw new Error('请在微信内打开公众号页面确认收款')
  const sdk=await loadOfficialSDK(),config=await getOfficialSDKConfig(window.location.href.split('#')[0])
  if(config.appId!==expectedAppID)throw new Error('公众号配置与原提现收款身份不一致，请联系平台核查原单')
  await new Promise<void>((resolve,reject)=>{const timer=setTimeout(()=>reject(new Error('微信收款确认授权超时，请检查公众号JS接口安全域名')),8000);sdk.ready(()=>{clearTimeout(timer);resolve()});sdk.error(()=>{clearTimeout(timer);reject(new Error('微信收款确认授权失败，请核对公众号配置'))});sdk.config({debug:false,...config,jsApiList:['checkJsApi','requestMerchantTransfer']})})
  await new Promise<void>((resolve,reject)=>{sdk.checkJsApi({jsApiList:['requestMerchantTransfer'],success:result=>{let checks=result.checkResult;if(typeof checks==='string'){try{checks=JSON.parse(checks) as unknown}catch{checks=null}}if(checks&&typeof checks==='object'&&'requestMerchantTransfer'in checks&&(checks.requestMerchantTransfer===true||checks.requestMerchantTransfer==='true'))resolve();else reject(new Error('当前微信版本不支持收款确认，请升级微信'))},fail:()=>reject(new Error('微信收款确认能力检查失败，请重试'))})})
  return sdk
}
/** 展示官方商家转账确认页，成功仅表示官方页面交互完成。 */
export async function confirmOfficialTransfer(confirmation:TransferConfirmation):Promise<void>{
  await prepareOfficialTransfer(confirmation.app_id)
  const bridge=(globalThis as unknown as OfficialTransferWindow).WeixinJSBridge
  if(!bridge)throw new Error('微信收款确认尚未就绪，请重新打开页面')
  return new Promise<void>((resolve,reject)=>bridge.invoke('requestMerchantTransfer',{mchId:confirmation.mch_id,appId:confirmation.app_id,package:confirmation.package_info},result=>{if(result.err_msg==='requestMerchantTransfer:ok')resolve();else reject(new Error(result.err_msg?.includes('cancel')?'已取消收款确认':'收款确认未完成，请同步原提现单'))}))
}
