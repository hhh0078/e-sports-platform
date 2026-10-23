# 消息中心 · 本机消息

文件作用：对应 `frontend/src/pages/account/messages/index.vue` 的先行390×844设计与实施规则。

- 共享AppShell返回“消息中心”，无主底栏；开放列表仅标题、未读点和箭头，不显示正文摘要，不堆头像、状态卡或假未读数。图中底部本机提示只是可选说明，实际可省略，避免多余技术文案。
- 访客正常空态，“请登录后查看”为轻量提示；不强制跳登录墙。已登录按账号隔离本机缓存。消息列表先完整同步分页并落盘，未读消息不能删除远端；仅详情点击已读并持久化成功后才DELETE，失败绝不删远端。
- 打开消息详情标已读，全文继续留在当前设备，切账号不串消息。长标题自然换行，列表加载、同步错误可重试；没有消息不使用图中三条样例作为假数据。

- 自然黑绿/薄荷主题，16px边距、细分隔、官方居中图标。原生状态栏及胶囊仍按系统安全区避让，生成字形误差以真实文案和说明为准。

## 完整生成提示词

Use case: ui-mockup. One professional Chinese WeChat Mini Program UI,exact390:844flatportrait,no frame/nooutside margins. Same established 开机电竞 darknaturalblackgreen #0a1110,white#edf4ef,muted#9dafa3,mint#8de8b6. Sharedsecondarybackarrow/title beneathnativeWeChatstatus/capsulesafearea. NO primarybottomnav. 16pxinsets,24pxsectiongaps,thin separators,officialcenteredicons,body16px/secondary14px,openeditorial layout,44pxprimarysubmit and inputs,36pxcompactactions,textarea lowgray8pxradius. NO nestedpanels,giantcards,dashedemptyframes,illustrations,Englishdecoration orbluepurplegradient. Alltext real UI elements, designisnotbackground. Title消息中心. Underheader a quiet right alignedsmall 刷新 text withrefreshicon. MainlistOPEN rowsseparatedbyfinehorizontalrules. Eachrowonlyshows amint6pxunreaddot(ifunread), messageTITLEandrightchevron. NO bodyexcerptorpreview, NO hugeavatar,NO timestamporsendermetadata. Three examples: ordernotificationtitle 订单进度更新, platformtitle 平台服务公告, AIreviewtitle 服务审核结果; firsttwo unreadmintdot,thirdreadmutedtext. Titles16px max2lines;100pxrowsorcomfortable72px. Belowlistquiet本机消息 withsubline 登录账号后同步，消息保存在当前设备 only if appropriate; avoid extratechnicalstorageexplanation inproduct. Better minimalno footerexplanation. Largecalmremainingblankspace. Guestalternateislightempty无消息 small请登录后查看 andquiet登录link,notforcedlogin. Clickingopensfulltextdetailandmarksread. Neverfakecounts. No newtabs/filter.

## 2026-10-24 微信订阅入口

沿用已批准布局，仅加入共享紧凑铃铛文字入口。真实授权弹层遵循 `design/pages/account/subscriptions.png` 与同名说明；不修改主交易控件，不异步自动弹微信授权。服务发布成功仅提示打手已付款新单，打手订单页可订阅退款申请；买家仅接单结果，俱乐部负责人仅成员申请/旗下争议，平台仅新俱乐部申请。消息中心增加本人微信提醒设置，平台控制台增加独立配置入口。普通聊天、钱包、调价和图文审核不发微信订阅提醒，仍保留站内消息。
