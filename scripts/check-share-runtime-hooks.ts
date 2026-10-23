/** 文件作用：只读验证正式/测试微信编译产物的两种分享事件位，防止共享函数隐藏官方hook导致页面没有原生分享能力。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

const directory=path.resolve(process.argv[2]||'frontend/dist/build/mp-weixin')
const pages=['main/index','services/detail','requests/detail','community/players/detail','community/clubs/detail','shares/create','shares/detail']
const results:Array<{page:string;runtime_hooks:number;friend_registrations:number;timeline_registrations:number}>=[]
for(const page of pages){const filename=path.join(directory,'pages',page+'.js');assert.ok(fs.existsSync(filename),'缺少分享编译页面：'+filename);const config=JSON.parse(fs.readFileSync(filename.replace(/\.js$/,'.json'),'utf8')) as Record<string,unknown>;for(const key of ['enableShareAppMessage','enableShareTimeline'])assert.equal(Object.hasOwn(config,key),false,'微信页面不支持的配置：'+page+' '+key);const source=fs.readFileSync(filename,'utf8'),match=source.match(/__runtimeHooks\s*[:=]\s*(\d+)/);assert.ok(match,'编译页没有__runtimeHooks：'+page);const mask=Number(match[1]),friend=[...source.matchAll(/\.onShareAppMessage\s*\(/g)].length,timeline=[...source.matchAll(/\.onShareTimeline\s*\(/g)].length;assert.equal(mask&6,6,'编译页缺少好友或朋友圈事件位：'+page);assert.equal(friend,1,'页面必须直接注册一次好友hook：'+page);assert.equal(timeline,1,'页面必须直接注册一次朋友圈hook：'+page);results.push({page:'pages/'+page,runtime_hooks:mask,friend_registrations:friend,timeline_registrations:timeline})}
console.log(JSON.stringify({passed:true,directory,pages:results},null,2))
