/** 文件作用：共享访客运营数据，按游戏和请求序号隔离旧回包；公告账号状态另行通过本人接口核对。 */
import { shallowRef } from 'vue'
import { getPublicContent } from '../api/content.api'
import type { PublicContent } from '../types/content'
import { state } from './store'
export const publicContent=shallowRef<PublicContent|null>(null)
let generation=0,inflight:{game:string;promise:Promise<void>}|null=null
/** 只采纳当前游戏最新的公开响应，同一在途读取复用而不重复请求。 */
export function refreshPublicContent():Promise<void>{const game=state.gameId;if(inflight?.game===game)return inflight.promise;const current=++generation;const promise=getPublicContent(game).then(value=>{if(current===generation&&state.gameId===game)publicContent.value=value}).finally(()=>{if(current===generation)inflight=null});inflight={game,promise};return promise}
/** 游戏切换立即清除旧图，避免新游戏请求失败时误显示上一游戏运营内容。 */
export function resetPublicContent():void{generation++;inflight=null;publicContent.value=null}
