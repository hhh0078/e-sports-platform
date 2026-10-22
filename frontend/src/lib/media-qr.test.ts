/** 文件作用：执行真实图片处理模块，核验客服二维码官方1:1裁剪、800px清晰压缩及账户切换上传边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { executable } from './sfc-test'

/** 按uni-app条件编译保留指定平台代码，避免在Node中同时执行微信和网页分支。 */
function platformSource(source:string,platform:'MP-WEIXIN'|'H5'):string{const enabled=[true];return source.split('\n').filter(line=>{const match=line.match(/\/\/\s*#(ifdef|ifndef)\s+(\S+)/);if(match){const active=match[1]==='ifdef'?match[2]===platform:match[2]!==platform;enabled.push(enabled[enabled.length-1]&&active);return false}if(/\/\/\s*#endif/.test(line)){enabled.pop();return false}return enabled[enabled.length-1]}).join('\n')}
/** 只模拟官方图像API回调，图片处理和用途选择来自真实生产代码。 */
function harness(sizes=[400*1024]){
  const operations:Array<{kind:string;value:Record<string,unknown>}>=[],uploads:Array<{purpose:string;path:string;clubID?:string;token:string}>=[]
  let authToken='A',sizeIndex=0
  const context=vm.createContext({token:()=>authToken,ApiError:class extends Error{cancelled=false},
    uni:{showModal:(options:Record<string,unknown>)=>(options.success as (value:object)=>void)({confirm:true})},
    wx:{canIUse:()=>true,chooseMedia:(options:Record<string,unknown>)=>{operations.push({kind:'choose',value:options});(options.success as (value:object)=>void)({tempFiles:[{tempFilePath:'/tmp/code.png'}]})},cropImage:(options:Record<string,unknown>)=>{operations.push({kind:'crop',value:options});(options.success as (value:object)=>void)({tempFilePath:'/tmp/cropped.png'})},getImageInfo:(options:Record<string,unknown>)=>(options.success as (value:object)=>void)({width:1200,height:1200}),compressImage:(options:Record<string,unknown>)=>{operations.push({kind:'compress',value:options});(options.success as (value:object)=>void)({tempFilePath:'/tmp/compressed.png'})},getFileSystemManager:()=>({getFileInfo:(options:Record<string,unknown>)=>(options.success as (value:object)=>void)({size:sizes[Math.min(sizeIndex++,sizes.length-1)]})})},
    uploadMediaImage:async(purpose:string,path:string,clubID:string|undefined,token:string)=>{uploads.push({purpose,path,clubID,token});return{url:'/api/media/qr'}}})
  vm.runInContext(executable(platformSource(readFileSync(new URL('./media.ts',import.meta.url),'utf8'),'MP-WEIXIN'))+'\nglobalThis.media={prepareImage,chooseUploadImage,IMAGE_RULES}',context)
  return{media:context.media,operations,uploads,
    /** 仅替换本次测试身份，不执行任何网络授权。 */
    switchAccount(){authToken='B'},
  }
}

test('平台和俱乐部二维码均走官方1:1裁剪800px高质量压缩',async()=>{
  for(const purpose of ['support_qr','club_contact_qr']){
    const subject=harness()
    await subject.media.chooseUploadImage(purpose,{clubId:purpose==='club_contact_qr'?'club-A':undefined})
    assert.equal(subject.operations.find(item=>item.kind==='crop')?.value.cropScale,'1:1')
    const compress=subject.operations.find(item=>item.kind==='compress')!
    assert.equal(compress.value.compressedWidth,800);assert.equal(compress.value.quality,96)
    assert.equal(subject.uploads[0].purpose,purpose);assert.equal(subject.uploads[0].clubID,purpose==='club_contact_qr'?'club-A':undefined)
  }
})

test('二维码较大时仍优先清晰度，不使用普通封面的45质量',async()=>{
  const subject=harness([700*1024,700*1024,450*1024])
  await subject.media.prepareImage('support_qr',{filePath:'/tmp/qr.png'})
  assert.deepEqual(subject.operations.filter(item=>item.kind==='compress').map(item=>item.value.quality),[96,92])
})

test('二维码超过3MiB拒绝上传，选图期间换账号也不能沿用旧上传令牌',async()=>{
  const large=harness([4*1024*1024]);await assert.rejects(large.media.chooseUploadImage('support_qr',{filePath:'/tmp/qr.png'}),/3MB/);assert.equal(large.uploads.length,0)
  const switched=harness(),uploading=switched.media.chooseUploadImage('support_qr',{filePath:'/tmp/qr.png'})
  switched.switchAccount();await assert.rejects(uploading,/账号已切换/);assert.equal(switched.uploads.length,0)
})

test('网页二维码实际请求白底PNG编码并使用800px，普通封面仍按JPEG压缩',async()=>{
  const formats:string[]=[],backgrounds:string[]=[],dimensions:Array<[number,number]>=[]
  const context=vm.createContext({Blob,token:()=> 'A',ApiError:class extends Error{},queueMicrotask,
    Image:class{width=1200;height=1200;onload:(()=>void)|null=null;set src(_value:string){queueMicrotask(()=>this.onload?.())}},
    document:{createElement:()=>{const canvas={width:0,height:0,getContext:()=>({fillStyle:'',fillRect(){backgrounds.push('white')},drawImage(){}}),toBlob(callback:(blob:Blob)=>void,format:string){formats.push(format);dimensions.push([canvas.width,canvas.height]);callback(new Blob(['image'],{type:format}))}};return canvas}},
    URL:{createObjectURL:()=> 'blob:processed'},
  })
  vm.runInContext(executable(platformSource(readFileSync(new URL('./media.ts',import.meta.url),'utf8'),'H5'))+'\nglobalThis.prepare=prepareImage',context)
  assert.equal(await context.prepare('support_qr',{filePath:'/tmp/code.png'}),'blob:processed')
  assert.equal(formats[0],'image/png');assert.deepEqual(dimensions[0],[800,800]);assert.equal(backgrounds.length,1)
  await context.prepare('profile_cover',{filePath:'/tmp/cover.jpg'})
  assert.equal(formats[1],'image/jpeg');assert.equal(backgrounds.length,1)
})
