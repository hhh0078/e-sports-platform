/** 文件作用：统一微信付款及商家转账确认，原生成功仅代表操作完成，资金结果仍需服务端查单。 */
import type { PaymentParams } from '../types/models'
import type { TransferConfirmation } from '../types/wallet'
import { beginFinancialActivity } from './update-manager'
/** 调用官方小程序支付；取消以可识别错误返回，不伪造到账。 */
export function requestWechatPayment(params:PaymentParams):Promise<void>{
  const finish=beginFinancialActivity()
  return new Promise<void>((resolve,reject)=>{try{uni.requestPayment({provider:'wxpay',...params,success:()=>{finish();resolve()},fail:error=>{finish();reject(error)}})}catch(error){finish();reject(error)}})
}
/** 新版收款能力的最小官方接口，避免 SDK 尚未更新时使用无类型对象。 */
interface TransferRuntime { canIUse:(name:string)=>boolean; requestMerchantTransfer?:(options:{mchId:string;appId:string;package:string;success:()=>void;fail:(error:unknown)=>void})=>void }
/** 拉起微信官方收款确认页，兼容能力不足时明确提示升级微信。 */
export function confirmWechatTransfer(confirmation:TransferConfirmation):Promise<void>{
  const runtime=(globalThis as unknown as {wx?:TransferRuntime}).wx
  if(!runtime?.canIUse('requestMerchantTransfer')||!runtime.requestMerchantTransfer)return Promise.reject(new Error('请在新版微信小程序中确认收款'))
  const finish=beginFinancialActivity()
  return new Promise<void>((resolve,reject)=>{try{runtime.requestMerchantTransfer!({mchId:confirmation.mch_id,appId:confirmation.app_id,package:confirmation.package_info,success:()=>{finish();resolve()},fail:error=>{finish();reject(error)}})}catch(error){finish();reject(error)}})
}
