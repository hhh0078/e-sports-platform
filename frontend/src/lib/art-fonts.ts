/** 文件作用：共用服务端字体白名单，按实际文案建立无碰撞字集标识，拒绝任意字体地址。 */
import catalog from '../../../backend/internal/artfonts/catalog.json'
import type {CoverLabel} from '../types/models'
import {normalizeLabelText} from './cover'

export const ART_FONT_VERSION=catalog.version
export const ART_FONTS=[{id:'',label:'系统默认'},...catalog.fonts.map(({id,label})=>({id,label}))]

/** 将未知字体编号安全回退到系统字体，兼容旧记录和已移除字体。 */
export function artFontID(value:unknown):string{return typeof value==='string'&&ART_FONTS.some(font=>font.id===value)?value:''}
/** 实际文案去重排序，最大144码点与服务端保持一致。 */
export function fontCharacters(text:string):string{return [...new Set(Array.from(text).slice(0,144).filter(char=>!/[\u0000-\u001f\u007f-\u009f]/.test(char)&&(char.codePointAt(0)!<0xd800||char.codePointAt(0)!>0xdfff)))].sort((a,b)=>a.codePointAt(0)!-b.codePointAt(0)!).join('')}
/** 为同一字体的实际字形集合创建唯一CSS名称，使用码点直接编码以避免哈希碰撞。 */
export function artFontFamily(id:string,text:string):string{return 'Art'+ART_FONT_VERSION+'_'+id+'_'+Array.from(fontCharacters(text)).map(char=>char.codePointAt(0)!.toString(16)).join('_')}
/** 对当前可见文案按字体合并字集；系统字体、空文案和未知编号不产生下载。 */
export function artFontGroups(labels:CoverLabel[]):Array<{id:string;text:string;family:string}>{
  const groups=new Map<string,string>()
  for(const label of labels){const id=artFontID(label.font_family),text=normalizeLabelText(label.text);if(id&&text)groups.set(id,(groups.get(id)||'')+text)}
  return [...groups].map(([id,text])=>{const chars=fontCharacters(text);return{id,text:chars,family:artFontFamily(id,chars)}}).filter(group=>group.text)
}
