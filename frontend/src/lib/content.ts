/** 文件作用：提供运营内容的白名单跳转与初始编辑稿，不将URL或图层烘焙进图片。 */
import type {AdminPublicContent,ContentTarget} from '../types/content'
import {go} from './format'
/** 只根据已登记类型构造站内路由，外部或异常类型不执行。 */
export function contentTargetPath(target:ContentTarget|null|undefined):string{if(!target||!['service','request','article'].includes(target.kind)||typeof target.id!=='string'||Array.from(target.id).length<1||Array.from(target.id).length>128||target.id.startsWith('demo_')||/[\s/\\?#\u0000-\u001f]/.test(target.id))return '';const paths={service:'/pages/services/detail',request:'/pages/requests/detail',article:'/pages/content/articles/detail'};return paths[target.kind]+'?id='+encodeURIComponent(target.id)}
/** 用户明确点击运营内容后才导航，没有目标时不做任何事。 */
export function openContentTarget(target:ContentTarget|null|undefined):void{const path=contentTargetPath(target);if(path)go(path)}
/** 创建无虚构运营记录的安全编辑初值，真实修订号读取后才允许保存。 */
export function emptyPublicContent():AdminPublicContent{return {revision:0,global_slides:[],game_slides:{},announcement:{id:'home',version:0,enabled:false,frequency:'once',mode:'text',image_url:'',title:'',body:'',target:null},page_tips:{player:{text:'',article_id:''},worker:{text:'',article_id:''},orders:{text:'',article_id:''},me:{text:'',article_id:''}},service_notice:'',request_notice:'',updated_at:''}}
