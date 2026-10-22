# 平台控制台 · 核心数据与模块入口

文件作用：对应 `frontend/src/pages/manage/index.vue` 的新控制台先行设计；替代原七页签长页。旧图/说明归档 `design/history/20261022-before-admin-console/pages/manage/`。

- 顶部仅三项真实核心数据：注册用户、全部订单、真实成交额；待办为服务审核/俱乐部申请/订单争议，全部来自后端口径，不硬编码图中0值。
- 交易额曲线提供7天/30天/自定义日期，使用后端analytics真实聚合；无数据诚实空态/零基线，不随机造曲线。日期区间和金额坐标、加载/错误/空态都保留，图中示意文案以实际接口为准。
- 控制台只负责摘要、待办、曲线与导航，不再渲染原七个管理tabs或整段业务列表。
- 十个入口为用户、服务、俱乐部、订单售后、游戏、钱包审核、微信支付、协议、平台设置、审计；两列低对比轻卡片，不堆大按钮。平台设置独立保留名称与公告编辑；协议/支付可进入同一settings页不同query tab。
- 普通统一二级返回，无主底栏；16px边距、细分隔、主题色和官方居中图标。长内容正常滚动，320px不溢出。
- 只有平台管理员可见入口与数据，普通玩家及俱乐部管理员不可见且路由拒绝；后端仍独立校验。图中零金额仅空数据示意，不是赠款。

## 完整编辑提示词

Use case: ui-mockup. Redesign the attached old Chinese390x844managementscreen into the NEW platformADMINCONSOLE. Preserveonlyapprovednaturaldarkeditorialdesign #0a1110white#edf4efmutedgraygreen#9dafA3mint#8de8b6,16pxinsets,thinlines,quiet8pxcards,officialcenteredicons. Ordinarysharedbacktitle 平台管理, nativeWeChatstatus/capsulesafearea, NO mainbottomtabs. REMOVEold7managementtabsandlonginlinelistsentirely. ThispageisONLYcoredata/todo/trend/moduleentryconsole. TopTHREEopentypographicmetrics:注册用户0,全部订单0,真实成交额¥0.00; allruntimefrombackend,zeroisvalidemptyillustration,nofabricatedcash/counts. Compact待办lineor3smallicons 服务待审核0 / 俱乐部申请0 / 订单争议0. Next交易额趋势heading, compact7天/30天/自定义rangecontrols,7天active. Aclean120-150pxtallmintchartusesREALserverdata; showhonestemptyzerobaselineandsmall暂无成交数据 inthisdesign,notarbitraryrisinggraph. Customrangeopensnativefrom/to datepickers,notshown. Below管理模块headingandEXACTLYTENsubtlecompactentryCARDSin2columns5rowswithofficialthinicon,label,quietchevron:用户管理/服务审核/俱乐部管理/订单与售后/游戏配置/钱包审核/微信支付/协议管理/平台设置/操作审计. Cardsabout64-72pxhigh,lightthinborder,mutedcharcoalsurface,nogiantbuttonsorbadgedfakecounts;maynaturalscrollbelowfirstscreen,squeezeNOtext. IncludeALL10,platform设置retainsname/announcementeditingatseparatepage. DoNOTshowmerchantkeys,per-useradminconfig,inlineauditlistsorchartfakefigures. Onlyplatformadminscanseeconsole/routes;normal/clubadminhavenoentryorconfig. Naturalprofessionalinformationhierarchy,nopurplebluegradient,noillustrations,nodeviceframe,exact390:844flatportrait.
