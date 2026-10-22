/** 文件作用：按实际像素为封面文案计算可读字号、必要换行和九宫格避让，拥挤时增高封面。 */
import {normalizeLabelText,POSITIONS,type PlacedLabel} from './cover'
import type {CoverLabel} from '../types/models'
/** 真实像素中的布局结果，标签仍以共享360画布坐标供CoverArt渲染。 */
export interface ReadableCoverLayout {height:number;labels:PlacedLabel[]}
/** 内部像素矩形包含完整文案及展示尺寸。 */
interface PixelLabel {label:CoverLabel;id:string|number;text:string;position:string;font:number;x:number;y:number;w:number;h:number}
/** 估计文字宽度，中文和表情按宽字符预留，英文宽字符与窄字符分别处理。 */
function textUnits(text:string):number{return Array.from(text).reduce((sum,char)=>sum+(/^[ilI1.,'`!|:; ]$/.test(char)?.48:/^[MW@#%&]$/.test(char)?1.05:/^[\x20-\x7e]$/.test(char)?.76:1.12),0)}
/** 判断实际像素矩形是否相交，额外保留4px间距。 */
function collides(a:PixelLabel,b:PixelLabel):boolean{return a.x<b.x+b.w+4&&a.x+a.w+4>b.x&&a.y<b.y+b.h+4&&a.y+a.h+4>b.y}
/** 尝试保留每条文案的九宫格位置，再沿相邻可用位置寻找无碰撞落点。 */
function placeLabels(entries:PixelLabel[],width:number,areaHeight:number):PixelLabel[]{
  const placed:PixelLabel[]=[]
  for(const entry of entries){
    const [vertical,horizontal]=entry.position.split('-')
    const desiredX=horizontal==='right'?width-8-entry.w:horizontal==='center'?(width-entry.w)/2:8
    const desiredY=vertical==='top'?8:vertical==='middle'?(areaHeight-entry.h)/2:areaHeight-8-entry.h
    const candidates:Array<{x:number;y:number}>=[{x:desiredX,y:desiredY}]
    for(let y=8;y<=areaHeight-8-entry.h+.01;y+=4)for(let x=8;x<=width-8-entry.w+.01;x+=4)candidates.push({x,y})
    candidates.sort((a,b)=>Math.abs(a.x-desiredX)+Math.abs(a.y-desiredY)*2-Math.abs(b.x-desiredX)-Math.abs(b.y-desiredY)*2)
    const spot=candidates.find(candidate=>candidate.x>=7.99&&candidate.y>=7.99&&candidate.x+entry.w<=width-7.99&&candidate.y+entry.h<=areaHeight-7.99&&!placed.some(previous=>collides({...entry,...candidate},previous)))
    if(!spot)return placed
    placed.push({...entry,...spot})
  }
  return placed
}
/**
 * 在实际显示宽度下布局六条以内文字；字体不低于指定像素，长文案换行且保留顶部/底部系统区域。
 * @param labels - 独立文案配置。
 * @param width - 实测封面宽度。
 * @param baseHeight - 图片默认显示高度。
 * @param reserveBottom - 底部标题标签及押金预留像素。
 * @param reserveTop - 顶部角标或系统安全区预留像素。
 * @param minFontPixels - 本位置的最小可读字号。
 * @returns 最终高度和与共享CoverArt兼容的百分比坐标。
 */
export function layoutReadableCover(labels:CoverLabel[],width:number,baseHeight:number,reserveBottom=0,reserveTop=0,minFontPixels=10):ReadableCoverLayout{
  if(!Number.isFinite(width)||width<40)return{height:Math.max(0,baseHeight),labels:[]}
  const bottom=Number.isFinite(reserveBottom)?Math.max(0,reserveBottom):0,top=Number.isFinite(reserveTop)?Math.max(0,reserveTop):0,minFont=Number.isFinite(minFontPixels)?Math.max(10,Math.min(24,minFontPixels)):10
  const source=(Array.isArray(labels)?labels:[]).filter(Boolean).slice(0,6).map(label=>({...label,text:normalizeLabelText(label.text)})).filter(label=>label.text)
  const entries:PixelLabel[]=source.map((label,index)=>{
    const requested=Number(label.font_size),font=Math.max(minFont,(Number.isFinite(requested)?Math.max(12,Math.min(24,requested)):16)*width/360)
    const natural=textUnits(label.text)*font,maxWidth=width-16,w=Math.min(maxWidth,Math.ceil(natural+12)),lines=Math.max(1,Math.ceil(natural/Math.max(1,w-12)))
    return{label,id:label.id||'readable-'+index,text:label.text,position:POSITIONS.some(item=>item[0]===label.position)?label.position!:'bottom-left',font,x:0,y:0,w,h:Math.ceil(lines*font*1.25)+6}
  })
  let height=Math.max(Number.isFinite(baseHeight)?baseHeight:width/(4/3),top+bottom+(entries.length?Math.max(...entries.map(item=>item.h))+16:0))
  // 居中锚点会把空余空间分在两侧，多预留一整行以免偶数条文案最后一条无处落下。
  const maximum=Math.max(height,top+bottom+16+entries.reduce((sum,item)=>sum+item.h+8,0)+(entries.length?Math.max(...entries.map(item=>item.h))+16:0))
  let placed=placeLabels(entries,width,height-top-bottom)
  while(placed.length<entries.length&&height<maximum){height=Math.min(maximum,height+16);placed=placeLabels(entries,width,height-top-bottom)}
  return{height,labels:placed.map(entry=>({...entry.label,id:entry.id,text:entry.text,position:entry.position,font_size:entry.font*360/width,w:entry.w*360/width,h:entry.h*360/width,x:entry.x*360/width,y:(entry.y+top)*360/width,left:entry.x/width*100,top:(entry.y+top)/height*100,width:entry.w/width*100,height:entry.h/height*100}))}
}
