/** 文件作用：构造匿名字体子集资源地址，只包含白名单编号及当前文案实际字形。 */
import {API_BASE} from './client'
import {ART_FONT_VERSION,artFontID,fontCharacters} from '../lib/art-fonts'
/** 返回服务端按字裁剪资源；空文字和系统字体不提供网络地址。 */
export function fontSubsetURL(id:string,text:string):string{
  const font=artFontID(id),characters=fontCharacters(text)
  return font&&characters?API_BASE+'/fonts/'+ART_FONT_VERSION+'/'+font+'.woff?text='+encodeURIComponent(characters):''
}
