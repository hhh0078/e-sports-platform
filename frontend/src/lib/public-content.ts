/** 文件作用：共享访客运营数据，按游戏和请求序号隔离旧回包；公告账号状态另行通过本人接口核对。 */
import { ref,shallowRef } from 'vue'
import { getPublicContent } from '../api/content.api'
import { errorMessage } from '../api/client'
import type { PublicContent } from '../types/content'
import { initPublic,state } from './store'
export const publicContent=shallowRef<PublicContent|null>(null)
export const publicContentLoading=ref(false),publicContentError=ref('')
let generation=0,inflight:{game:string;promise:Promise<void>}|null=null
/** 公开初始化不等待本人接口；仅采纳当前游戏最新响应，热刷新保留已确认画面。 */
export function refreshPublicContent():Promise<void>{
  if(inflight?.game===state.gameId)return inflight.promise
  const current=++generation
  let requestedGame=state.gameId
  if(publicContent.value?.game_id!==requestedGame)publicContent.value=null
  publicContentLoading.value=true;publicContentError.value=''
  const promise:Promise<void>=(async()=>{
    try{
      await initPublic()
      if(current!==generation)return
      requestedGame=state.gameId
      if(inflight)inflight.game=requestedGame
      const value=await getPublicContent(requestedGame)
      if(current!==generation||state.gameId!==requestedGame)return
      if(value.game_id!==requestedGame)throw new Error('运营信息与当前游戏不一致，请重试')
      if(JSON.stringify(publicContent.value)!==JSON.stringify(value))publicContent.value=value
    }catch(error){
      if(current===generation&&state.gameId===requestedGame)publicContentError.value=errorMessage(error)
      throw error
    }finally{
      if(current===generation){inflight=null;publicContentLoading.value=false}
    }
  })()
  inflight={game:requestedGame,promise};return promise
}
/** 游戏切换立即清除旧图，避免新游戏请求失败时误显示上一游戏运营内容。 */
export function resetPublicContent():void{generation++;inflight=null;publicContent.value=null;publicContentLoading.value=false;publicContentError.value=''}
