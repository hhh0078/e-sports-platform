/** 文件作用：执行真实公告图片处理，验证透明PNG不经有损压缩且保持字节、像素与账号安全边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {executable} from './sfc-test'

/** 按uni-app条件编译选择实际平台分支，避免测试同时运行微信与网页实现。 */
function platformSource(platform:'MP-WEIXIN'|'H5'):string{const enabled=[true];return readFileSync(new URL('./media.ts',import.meta.url),'utf8').split('\n').filter(line=>{const match=line.match(/\/\/\s*#(ifdef|ifndef)\s+(\S+)/);if(match){enabled.push(enabled[enabled.length-1]&&(match[1]==='ifdef'?match[2]===platform:match[2]!==platform));return false}if(/\/\/\s*#endif/.test(line)){enabled.pop();return false}return enabled[enabled.length-1]}).join('\n')}
/** 只控制官方媒体元数据与字节数，真实用途分支和压缩调用仍由生产代码决定。 */
function native(width=900,height=1200,size=600*1024,type:string|undefined='png'){
  const operations:Array<{kind:string;options:Record<string,unknown>}>=[],uploads:Array<{purpose:string;path:string}>=[]
  let authToken='account-A'
  const context=vm.createContext({ApiError:class extends Error{cancelled=false},token:()=>authToken,
    uni:{showModal:(options:Record<string,unknown>)=>(options.success as (value:object)=>void)({confirm:true})},
    wx:{canIUse:()=>true,
      chooseMedia(options:Record<string,unknown>){operations.push({kind:'choose',options});(options.success as (value:object)=>void)({tempFiles:[{tempFilePath:'/tmp/original-without-extension'}]})},
      cropImage(options:Record<string,unknown>){operations.push({kind:'crop',options});(options.success as (value:object)=>void)({tempFilePath:'/tmp/cropped'})},
      getImageInfo(options:Record<string,unknown>){operations.push({kind:'info',options});(options.success as (value:object)=>void)({width,height,type})},
      compressImage(options:Record<string,unknown>){operations.push({kind:'compress',options});(options.success as (value:object)=>void)({tempFilePath:'/tmp/compressed.jpg'})},
      getFileSystemManager:()=>({getFileInfo:(options:Record<string,unknown>)=>(options.success as (value:object)=>void)({size})}),
    },uploadMediaImage:async(purpose:string,path:string)=>{uploads.push({purpose,path});return{url:'/api/media/safe-announcement'}},
  })
  vm.runInContext(executable(platformSource('MP-WEIXIN'))+';globalThis.media={prepareImage,chooseUploadImage}',context)
  return{media:context.media,operations,uploads,
    /** 在媒体操作期间模拟账号轮换，不触及真实微信会话。 */
    switchAccount(){authToken='account-B'},
  }
}
/** 用可观察的标准Canvas边界记录编码类型和是否铺底，不宣称真实设备图像验收。 */
function web(width=1600,height=1000,size=500*1024){
  const formats:string[]=[],backgrounds:string[]=[],draws:number[][]=[],dimensions:Array<[number,number]>=[],urls:string[]=[],choices:Array<Record<string,unknown>>=[]
  const context=vm.createContext({Blob,ApiError:class extends Error{},queueMicrotask,
    Image:class{width=width;height=height;onload:(()=>void)|null=null;set src(_value:string){queueMicrotask(()=>this.onload?.())}},
    uni:{showModal:(options:Record<string,unknown>)=>(options.success as (value:object)=>void)({confirm:true}),chooseImage(options:Record<string,unknown>){choices.push(options);(options.success as (value:object)=>void)({tempFilePaths:['/source.png']})}},
    document:{createElement:()=>{const canvas={width:0,height:0,getContext:()=>({fillStyle:'',fillRect(){backgrounds.push('filled')},drawImage(_image:unknown,...coordinates:number[]){draws.push(coordinates)}}),toBlob(callback:(blob:Blob)=>void,format:string){formats.push(format);dimensions.push([canvas.width,canvas.height]);callback(new Blob([new Uint8Array(size)],{type:format}))}};return canvas}},
    URL:{createObjectURL:(blob:Blob)=>{urls.push(blob.type);return'blob:transparent-announcement'}},
  })
  vm.runInContext(executable(platformSource('H5'))+';globalThis.prepare=prepareImage',context)
  return{prepare:context.prepare,formats,backgrounds,draws,dimensions,urls,choices}
}

test('微信公告选原图并按官方PNG类型保留原字节，不裁剪也不调用compressImage',async()=>{
  const s=native();assert.equal(await s.media.chooseUploadImage('announcement_image'),'/api/media/safe-announcement')
  assert.deepEqual(Array.from(s.operations.find(row=>row.kind==='choose')!.options.sizeType as string[]),['original'])
  assert.equal(s.operations.filter(row=>row.kind==='crop'||row.kind==='compress').length,0);assert.deepEqual(s.uploads,[{purpose:'announcement_image',path:'/tmp/original-without-extension'}])
})
test('微信透明公告PNG超出3MiB、像素或尺寸限制时拒绝上传，不降级成JPEG',async()=>{
  for(const [width,height,size] of [[900,1200,3*1024*1024+1],[1281,600,100],[1280,6251,100],[100,8193,100],[9000,9000,100],[0,800,100]]){
    const s=native(width,height,size);await assert.rejects(s.media.chooseUploadImage('announcement_image',{filePath:'/tmp/original.png'}),/3MB|像素|分辨率/)
    assert.equal(s.operations.filter(row=>row.kind==='compress').length,0);assert.equal(s.uploads.length,0)
  }
  const exact=native(1280,6250,3*1024*1024);assert.equal(await exact.media.prepareImage('announcement_image',{filePath:'/tmp/exact.png'}),'/tmp/exact.png')
})
test('公告JPEG仍正常压缩，微信无法确认格式时明确失败，选图换号不沿用旧令牌',async()=>{
  const jpeg=native(1600,900,600*1024,'jpeg');assert.equal(await jpeg.media.prepareImage('announcement_image',{filePath:'/tmp/photo.jpg'}),'/tmp/compressed.jpg')
  assert.equal(jpeg.operations.find(row=>row.kind==='compress')!.options.compressedWidth,1280);assert.equal(jpeg.operations.find(row=>row.kind==='compress')!.options.quality,82)
  const unknown=native(900,1200,600*1024,'');await assert.rejects(unknown.media.prepareImage('announcement_image',{filePath:'/tmp/unknown'}),/无法确认公告图片格式/)
  const switched=native(),pending=switched.media.chooseUploadImage('announcement_image',{filePath:'/tmp/transparent.png'});switched.switchAccount();await assert.rejects(pending,/账号已切换/);assert.equal(switched.uploads.length,0)
})
test('H5公告用PNG编码保留透明底与完整比例，选择时不提前压成有损图',async()=>{
  const s=web();assert.equal(await s.prepare('announcement_image'),'blob:transparent-announcement')
  assert.deepEqual(s.formats,['image/png']);assert.deepEqual(s.backgrounds,[]);assert.deepEqual(s.dimensions,[[1280,800]]);assert.deepEqual(s.draws[0],[0,0,1600,1000,0,0,1280,800]);assert.deepEqual(s.urls,['image/png'])
  assert.deepEqual(Array.from(s.choices[0].sizeType as string[]),['original'])
})
test('H5公告先检查解码像素与输出像素，再拒绝超过3MiB的PNG，不创建可上传URL',async()=>{
  for(const [width,height] of [[9000,9000],[1280,6500],[100,9000]]){const s=web(width,height);await assert.rejects(s.prepare('announcement_image',{filePath:'/source.png'}),/分辨率|像素/);assert.equal(s.urls.length,0);assert.equal(s.formats.length,0)}
  const large=web(900,1200,3*1024*1024+1);await assert.rejects(large.prepare('announcement_image',{filePath:'/source.png'}),/3MB/);assert.equal(large.urls.length,0)
})
test('普通头像封面仍为JPEG，二维码仍按原规则铺白底PNG',async()=>{
  const s=web(1200,1200);await s.prepare('avatar',{filePath:'/avatar.png'});await s.prepare('profile_cover',{filePath:'/cover.png'});await s.prepare('support_qr',{filePath:'/qr.png'})
  assert.deepEqual(s.formats,['image/jpeg','image/jpeg','image/png']);assert.deepEqual(s.dimensions,[[512,512],[1200,675],[800,800]]);assert.equal(s.backgrounds.length,1)
})
