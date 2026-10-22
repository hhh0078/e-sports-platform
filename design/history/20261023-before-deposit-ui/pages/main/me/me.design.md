# 个人中心 · 默认接单与钱包入口

文件作用：对应 `frontend/src/pages/main/me/me.vue` 的先行设计及用户最新局部要求；2026-10-22生成/编辑。已有页面上一版归档在 `design/history/20261022-before-game-refinements/pages/`，钱包为本次新增，无旧版。

- 保留当前game.cover_url全宽背景、中性灰黑蒙版和底部自然淡出；64px头像/昵称/简介及三列统计正常流、避开状态栏和胶囊，所有数据仍来自真实用户。
- 删除接单身份NativeSwitch及所有账户级开启/关闭操作，改为静态“接单身份 · 默认开启”，说明无需挂靠，自由接单。所有新老用户默认可接单且无法关闭，访客可显示“登录后即可接单”。
- 在身份说明后的菜单增加轻量“我的钱包”入口，官方钱包图标和箭头、同48px细分隔行；这里只是跳转，不显示假余额、不堆充值大卡。未登录按原站内回跳登录。
- 在钱包之后增加同48px规格的“消息中心”轻入口，官方通知图标与箭头；真实未读状态可用小圆点，禁止伪造数量。消息页面保持访客轻空态，不能因新增入口压缩其他菜单。
- 保留我的订单、个人主页/俱乐部、手机号、协议和真实管理权限入口。新增菜单使首屏变长时自然滚动，不能压缩整页字号/行高。
- 默认接单身份不绕过手机号、冻结、简介及交易校验；单服务审核/暂停仍属服务状态。无主应用顶栏，唯一底栏和主题选中图标规则保留。

## 生成与验收边界

390×844，使用内置ImageGen。既有页面仅局部编辑，钱包新建；输出仅机械等比缩放。生成示例文字/数字/局部比例以真实业务、本说明和系统安全区为准，不能作为真实资金凭证。实际SDK与交易验收由root执行，本图不代表支付/真机通过。

## 完整生成或编辑提示词

最新消息能力局部编辑（优先于下方保留的原始指令）：

Use case: ui-mockup. Preciselocaleditofattached390x844personalcenter. PreserveEXACTLYexistingdynamicgamebackground/neutralgrayblackoverlay/avatar/nickname/stats/orders/dualhomeclubentry/staticprovider默认开启/钱包/个人资料/phone/agreements/privacy/admin/bottomnav. AddONLYonequietmenurow 消息中心,officialnotificationoutlineicon+rightchevron,matchingexisting48pxrowandthinseparator,immediatelyafter我的钱包. Smallunreaddotmayappearonlyfromactualcachedunreadstate,nofakeunreadcount. Morecontentmustscrollbelowfold,allowadminentrybelowfirstscreen;NO shrinkingtextorremovingexistingentries, no redesign,newart,bigcards,banners,bluepurplegradient. Bottomnav我的activeunchanged. Flat390:844,no frame.


Use case: ui-mockup. PRECISE LOCAL EDIT of attachedexistingChinese390x844design. Preserveallunrelatedapprovedlayout, naturaldarkgreen#0a1110,mint#8de8b6,thinsectiondividers,16pxinsets,officialcenteredicons,existingfieldsandnativeWeChatsafearea. DoNOTinventfeatures,newart,headlines,English,bluepurplegradients,orreplacegameartsource. Onlyedittheexplicitprovideridentitypoint. Exact390:844flatportrait,no deviceframe,nooutside margins. TextisindependentUI, notbakedintoimages. ThisisprimaryMytab,noapptopnavbar. Keepcurrentgamefullwidthbackgroundhero,neutralgrayblackoverlay,64pxavatar/nickname/bioandnormalflowstats unchanged; usesdynamicselectedgamecoveratruntime,supportingImage2isjustofficialexample. Keeplowerorders/dualhomeclubentries/accountmenuandbottomnavunchanged. ChangeONLY接单身份row: REMOVEtheentiretoggle/switchandthumbfromright; replacewithSTATICsmallminttext 默认开启 at right, optionallysmallcheckicon, plainnoninteractive. Rowleft接单身份,subline 无需挂靠，自由接单. Thiscapabilityisdefaultonforallnew/existingusersandcannotbeturnedoff; NO 开启/关闭 controlor去开启link. Guestvariantmayusetiny 登录后即可接单 sublineandzeroactualstats,notshownasloginwall. Keepallotherareasidentical. Bottomnav我的active,playerinactive; whenplayerisactiveinotherpagesgamepadshouldtheme-mint.

钱包轻入口局部追加：
Use case: ui-mockup. PRECISELOCALedit attached390x844personalcenter. PreserveEXACTLYallcurrentgamebackground/neutralgrayoverlay/avatar/stats/orders/dualhomeclubentry/staticprovider默认开启row, typography/colors/safeareasandbottomnav. AddONLYonequietmenuentry 我的钱包 withofficialwalletoutlineiconandchevron, same48pxrow/fine-dividerstyleas个人资料/手机号验证, belowstaticproviderrowbefore个人资料. No balancefigure/no fakefunds/no bigwalletcard/no rechargebuttonhere. Ifcontentheightincreases,letlowerrowsnaturallyscrollandlastplatformadminrowfallbelowfirstscreen;doNOTshrink/compressallcontent. ProvideridentityremainsSTATICdefaulton,no toggle/关闭/去开启. Onepersistentbottomnav我的active. Noappnavbar,no inventedart,no extraotherchanges. Flat390:844output,no deviceframe,no surroundingmargins.
