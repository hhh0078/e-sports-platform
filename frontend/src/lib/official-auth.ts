/** 文件作用：在公众号授权往返期间校验本标签页state及主动协议同意，不保存微信code。 */
import type { Consent } from '../types/models'
const storageKey='jijie_official_oauth_once'
/** 仅在当前页面启动与登录页之间短暂传递凭据，绝不写入持久缓存。 */
type OfficialCallback={code:string;state:string;consent:Consent}
let pendingCallback:OfficialCallback|null=null,pendingError:unknown
/** 只保存本标签页的随机state和已同意协议版本，回跳后立即删除。 */
export function rememberOfficialAuthorization(url:string,consent:Consent):void{
  const target=new URL(url)
  if(target.protocol!=='https:'||target.hostname!=='open.weixin.qq.com'||target.pathname!=='/connect/oauth2/authorize')throw new Error('公众号授权地址无效')
  const state=target.searchParams.get('state')
  if(!state||consent.agreed!==true)throw new Error('公众号授权状态缺失，请重新登录')
  sessionStorage.setItem(storageKey,JSON.stringify({state,consent,created:Date.now()}))
}
/** 读取并清除URL中的一次性凭据，与同一标签页发起的授权逐项匹配。 */
function readOfficialCallback():OfficialCallback|null{
  if(typeof window==='undefined')return null
  const url=new URL(window.location.href),hashParts=url.hash.split('?'),hashQuery=new URLSearchParams(hashParts.slice(1).join('?'))
  const code=url.searchParams.get('code')||hashQuery.get('code'),state=url.searchParams.get('state')||hashQuery.get('state')
  if(!code&&!state)return null
  url.searchParams.delete('code');url.searchParams.delete('state');hashQuery.delete('code');hashQuery.delete('state');url.hash=hashParts[0]+(hashQuery.size?'?'+hashQuery.toString():'')
  window.history.replaceState(null,'',url.toString())
  const raw=sessionStorage.getItem(storageKey);sessionStorage.removeItem(storageKey)
  let saved:{state?:string;consent?:Consent;created?:number}={}
  try{saved=JSON.parse(raw||'{}') as typeof saved}catch{/* 损坏授权记录不能用于登录。 */}
  if(!code||!state||state!==saved.state||saved.consent?.agreed!==true||!saved.created||Date.now()-saved.created>300000||Date.now()<saved.created)throw new Error('公众号授权已过期或来源不匹配，请重新勾选协议并登录')
  return{code,state,consent:saved.consent}
}
/** H5从无fragment的授权回跳地址启动时，先清理一次性凭据再打开登录页。 */
export function routeOfficialCallback():void{
  if(typeof window==='undefined'||(!window.location.search.includes('code=')&&!window.location.search.includes('state=')))return
  try{pendingCallback=readOfficialCallback()}catch(error){pendingError=error}
  uni.reLaunch({url:'/pages/account/login'})
}
/** 登录页消费当前进程内的一次性授权，普通进入返回空。 */
export function takeOfficialCallback():OfficialCallback|null{if(pendingError){const error=pendingError;pendingError=undefined;throw error}if(pendingCallback){const value=pendingCallback;pendingCallback=null;return value}return readOfficialCallback()}
