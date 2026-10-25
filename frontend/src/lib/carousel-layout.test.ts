/** 文件作用：验证真实首页轮播安全带、最小像素字号、九宫格与全部图片共享高度，避免品牌按钮和指示点覆盖。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {layoutCarouselSlides} from './carousel-layout'
import {POSITIONS} from './cover'
import type {ContentSlide} from '../types/content'
/** 创建密集文案但无虚构跳转的轮播配置。 */
function slide(id:string,count:number,position:string,font=24):ContentSlide{return{id,kind:'image',image_url:'',target:null,labels:Array.from({length:count},(_,i)=>({id:i,text:i%2?'最长文案十二字一起开心玩':'WWWWWWWWWWWW',position,font_size:font,color:'#ffffff',background:'#000000'}))}}
test('320/390宽的3/6最长文案在九位置均完整且避开真实品牌按钮/指示点安全带',()=>{
  for(const width of [320,390])for(const count of [3,6])for(const[position]of POSITIONS){
    const geometry={width,baseHeight:144*width/390+64,reserveTop:140*width/390,reserveBottom:20*width/390},slides=[slide('dense',count,position),slide('short',1,'bottom-right',12),slide('empty',0,'top-left')],source=JSON.stringify(slides),result=layoutCarouselSlides(slides,geometry)
    assert.ok(result.height>=geometry.baseHeight);assert.equal(result.slides.length,3);assert.equal(result.slides[0].length,count);assert.equal(result.slides[1].length,1);assert.equal(result.slides[2].length,0)
    for(const labels of result.slides){for(const label of labels){assert.ok(label.font_size*width/360>=9.999);assert.ok(label.top/100*result.height>=geometry.reserveTop+7.99);assert.ok((label.top+label.height)/100*result.height<=result.height-geometry.reserveBottom-7.99);assert.ok(label.left>=0&&label.left+label.width<=100.01)}for(let i=0;i<labels.length;i++)for(let j=i+1;j<labels.length;j++){const a=labels[i],b=labels[j];assert.ok((a.left+a.width)/100*width+3.99<=b.left/100*width||(b.left+b.width)/100*width+3.99<=a.left/100*width||(a.top+a.height)/100*result.height+3.99<=b.top/100*result.height||(b.top+b.height)/100*result.height+3.99<=a.top/100*result.height)}}
    assert.equal(JSON.stringify(slides),source)
  }
})
test('同一组空/有文案图片共用稳定最大高度，配置减少时允许恢复原图高度',()=>{
  const geometry={width:390,baseHeight:208,reserveTop:140,reserveBottom:20},result=layoutCarouselSlides([slide('empty',0,'top-left'),slide('dense',6,'middle-center')],geometry)
  assert.ok(result.height>208);assert.equal(result.slides[0].length,0);const repeated=layoutCarouselSlides([slide('empty',0,'top-left'),slide('dense',6,'middle-center')],geometry);assert.equal(result.height,repeated.height)
  assert.equal(layoutCarouselSlides([slide('empty',0,'top-left')],geometry).height,208);assert.equal(layoutCarouselSlides([],geometry).height,208)
  assert.deepEqual(layoutCarouselSlides([slide('not-measured',3,'bottom-left')],null).slides,[[]])
})
