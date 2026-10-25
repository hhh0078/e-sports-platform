/** 文件作用：在首页实测品牌/按钮安全带下布局独立轮播文案，全部幻灯片共用稳定高度和实际最小字号。 */
import {layoutReadableCover} from './cover-readable'
import type {PlacedLabel} from './cover'
import type {ContentSlide} from '../types/content'
/** 父层仅在显示、游戏配置或窗口变化时测量一次，所有数值为实际逻辑像素。 */
export interface CarouselGeometry{width:number;baseHeight:number;reserveTop:number;reserveBottom:number}
/** 每张图片使用同一高度，避免定时切图推动下方分类与服务列表。 */
export interface CarouselLayout{height:number;slides:PlacedLabel[][]}
/**
 * 复用可读封面算法，先求全部图片所需高度，再在统一高度内重排并归一化坐标。
 * @param slides - 当前游戏展开后的独立图片及文字图层。
 * @param geometry - 实际宽度、原图高度与顶部/指示点安全区。
 * @returns 稳定最大高度及每张图完整、互不覆盖的标签；不缩为不足10实际像素。
 */
export function layoutCarouselSlides(slides:ContentSlide[],geometry:CarouselGeometry|null):CarouselLayout{
  const base=geometry&&Number.isFinite(geometry.baseHeight)?Math.max(0,geometry.baseHeight):0
  if(!geometry||!Number.isFinite(geometry.width)||geometry.width<40)return{height:base,slides:slides.map(()=>[])}
  const top=Number.isFinite(geometry.reserveTop)?Math.max(0,geometry.reserveTop):0,bottom=Number.isFinite(geometry.reserveBottom)?Math.max(0,geometry.reserveBottom):0
  let height=base,layouts=slides.map(slide=>layoutReadableCover(slide.labels||[],geometry.width,height,bottom,top,10))
  // 某一张的居中锚点可能在统一增高后重新分布，最多八轮单调收敛；计算不绑定轮播索引。
  for(let turn=0;turn<8;turn++){
    const next=Math.max(height,...layouts.map(item=>item.height))
    if(next<=height+.001)break
    height=next;layouts=slides.map(slide=>layoutReadableCover(slide.labels||[],geometry.width,height,bottom,top,10))
  }
  height=Math.max(height,...layouts.map(item=>item.height))
  return{height,slides:layouts.map(item=>item.labels.map(label=>({...label,top:label.top*item.height/height,height:label.height*item.height/height})))}
}
