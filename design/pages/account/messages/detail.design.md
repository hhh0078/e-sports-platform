# 消息详情 · 本机消息

文件作用：对应 `frontend/src/pages/account/messages/detail.vue` 的先行390×844设计与实施规则。

- 共享AppShell返回“消息详情”，无主底栏；开放文章标题、来源/时间与全文，正文16px、段落间距24px、正常纵向滚动。消息内容作为纯文本显示，禁止v-html执行通知文本。
- 从当前账号本机缓存按id读取完整内容，点击详情先将read:true持久化，只有落盘成功才删除远端；本机全文继续保留。未读绝不删远端，缓存失败不发DELETE。不显示重复大按钮、聊天输入或伪造审核结论。
- 当前账号无此消息时显示轻量“该消息暂不可用”及返回；访客不暴露其他账号缓存。示例文字只说明排版，实际时间来源以接口真实字段为准。

- 自然黑绿/薄荷主题，16px边距、细分隔、官方居中图标。原生状态栏及胶囊仍按系统安全区避让，生成字形误差以真实文案和说明为准。

## 完整生成提示词

Use case: ui-mockup. One professional Chinese WeChat Mini Program UI,exact390:844flatportrait,no frame/nooutside margins. Same established 开机电竞 darknaturalblackgreen #0a1110,white#edf4ef,muted#9dafa3,mint#8de8b6. Sharedsecondarybackarrow/title beneathnativeWeChatstatus/capsulesafearea. NO primarybottomnav. 16pxinsets,24pxsectiongaps,thin separators,officialcenteredicons,body16px/secondary14px,openeditorial layout,44pxprimarysubmit and inputs,36pxcompactactions,textarea lowgray8pxradius. NO nestedpanels,giantcards,dashedemptyframes,illustrations,Englishdecoration orbluepurplegradient. Alltext real UI elements, designisnotbackground. Title消息详情. Openarticlebeneathheader: bold22px服务审核结果 title, smallergraydate2026-10-22 14:30 andneutral来源平台通知 ononequietline. Thinseparator24pxgap. Readable16pxfullplainChinese text distributedas3paragraphs: 你发布的服务已完成审核。 / 服务内容符合平台发布要求，现在可以在玩家页面被发现。请保持服务信息真实、及时回应订单沟通。 / 如需修改服务内容，可在打手页面进入编辑。 Exampleonlyruntimeuseslocalcachedmessagefromserver. Fulltextwraps, normalverticalscroll,no chatbubbles,nocards. No calltoactionexceptsharedback;no buttons tolaunchirrelevantsettings,no metadata overload,noreplyinput,no unreadtag. Openingalready marksread; not showbig已读success. Missing-localmessagealternatequiet该消息暂不可用+返回. Mustnotinventserveronopenpersistence. Backgrounddarknaturalblackgreen. Nativecapsulesafearea.

## 2026-10-23 审核结果行动入口

沿用当前设计，仅在正文下补充可选“关联位置”和一条薄荷色文字操作，不增加大按钮或卡片。保留action_path、action_label、target_type、target_id、field_label到账号独立缓存，已读及服务器副本删除顺序不变。只允许精确现有个人资料、编辑服务、俱乐部资料/主页/控制台、订单详情入口；客服/平台/游戏/消息管理四个配置入口仅平台管理员可见。成员申请本人去俱乐部主页，负责人审核说明去俱乐部控制台；页面本身仍以服务端实时权限为准。无字段的旧通知正常阅读全文。非法、外域、任意查询、编码绕过的路径不显示入口，点击时再次核对账号和路由。
