/** 文件作用：浏览器验证管理设置与窄屏布局，全部业务接口为离线夹具。 */
async (page) => {
 await page.unrouteAll(); const errors=[],requests=[]; let mode='strict';
 page.on('pageerror',e=>errors.push(e.message));
 await page.context().route('**/*',async route=>{
  const u=new URL(route.request().url());if(!u.pathname.startsWith('/api/'))return route.continue();
  const path=u.pathname.slice(4),method=route.request().method();requests.push({path,method});let data={};
  if(method==='OPTIONS')return route.fulfill({status:204,headers:{'access-control-allow-origin':'*','access-control-allow-headers':'*','access-control-allow-methods':'*'}});
  if(path==='/bootstrap') data={games:[{id:'delta',name:'三角洲行动',primary_color:'#8de8b6',bg_color:'#101614',text_color:'#f2f6f3',muted_color:'#91a99c',surface_color:'#17221c'}],site_name:'开机电竞',dev_mode:false};
  else if(path==='/me') data={id:'fixture-admin',role:'admin',status:'active',nickname:'测试管理员',is_minor:false,phone_verified:true};
  else if(path==='/admin/identity-settings'){if(method==='PUT')mode=route.request().postDataJSON().mode;data={mode,has_api_key:false,configured:mode==='basic',storage_available:false,api_url:'https://v2.xxapi.cn/api/idCardCheck'}}
  else if(path==='/me/identity')data={verified:true,is_minor:false,verification_mode:mode};
  else if(path.includes('/summary'))data={users:10,orders:2,pending_services:0,pending_clubs:0,pending_withdrawals:0};
  else if(path.includes('/analytics'))data={period:{start:'2026-11-11',end:'2026-11-17'},summary:{gmv_cents:0,refund_cents:0,paid_order_count:0,refunded_order_count:0},series:[],dev_mode:true};
  else if(path==='/agreements')data={user_version:'1',privacy_version:'1',user_agreement:'测试协议',privacy_policy:'测试隐私'};
  else if(path.includes('withdrawals'))data=[];
  else if(path==='/admin/wallet-settings')data={settings:{withdrawal_enabled:false,transfer_scene_id:'',transfer_scene_report_infos:[],transfer_notify_url:'',transfer_remark:'',user_recv_perception:''},transfer_ready:false,configuration_notice:'未配置'};
  else if(path.includes('commerce'))data={miniapp_payment_mode:'wechat',official_payment_mode:'wechat'};
  await route.fulfill({json:{data},headers:{'access-control-allow-origin':'*','access-control-allow-headers':'*'}});
 });
 await page.goto('http://127.0.0.1:5181/');await page.evaluate(()=>localStorage.setItem('jijie_token',JSON.stringify({type:'string',data:'fixture-token'})));
 await page.goto('http://127.0.0.1:5181/#/pages/manage/settings/index');
 await page.getByText('平台基础',{exact:true}).waitFor();
 if(await page.locator('.settings-entry').count()!==9)throw Error('分类数量不正确');
 await page.screenshot({path:'output/admin-settings-390.png',fullPage:true});
 await page.locator('.settings-entry').filter({hasText:'支付与结算'}).click();await page.getByText('微信支付',{exact:true}).waitFor();
 if(await page.locator('.settings-entry').count()!==3)throw Error('支付二级数量错误');
 await page.screenshot({path:'output/admin-payments-390.png',fullPage:true});
 await page.goto('http://127.0.0.1:5181/#/pages/manage/identity');await page.getByText('宽松基础校验',{exact:true}).waitFor();
 await page.locator('.identity-mode').filter({hasText:'宽松基础校验'}).click();
 if(await page.locator('.identity-config-key').count())throw Error('基础方式不应显示密钥输入');
 await page.locator('.identity-config-save').click();await page.waitForTimeout(300);if(mode!=='basic')throw Error('基础方式没有保存');
 await page.waitForTimeout(1500);await page.screenshot({path:'output/admin-identity-basic-390.png',fullPage:true});
 for(const width of [320,390]){
  await page.setViewportSize({width,height:844});
  for(const target of ['settings/index','identity','index','settings?tab=commerce','settings?tab=payment','settings?tab=agreements','wallet','wallet?view=transfer']){
   await page.goto('http://127.0.0.1:5181/#/pages/manage/'+target);await page.reload();await page.waitForTimeout(700);
   const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1);if(overflow)throw Error('水平溢出 '+target+' '+width);
   if(target==='wallet?view=transfer')await page.getByText('商家转账配置',{exact:true}).first().waitFor();
   await page.screenshot({path:'output/admin-'+target.replace(/[^a-z]/g,'-')+'-'+width+'.png',fullPage:true});
  }
 }
 if(errors.length)throw Error(JSON.stringify(errors));
 console.log(JSON.stringify({groups:9,paymentGroups:3,basicSaved:mode,widths:[320,390],pageErrors:errors,apiRequests:requests.length}));
}
