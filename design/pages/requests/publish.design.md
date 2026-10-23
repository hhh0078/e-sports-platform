# 发布需求 · 开放式交易表单

文件作用：对应 `frontend/src/pages/requests/publish.vue` 的先行新版设计和实施约束；2026-10-22按用户新增全站统一要求编辑。上一版图与说明归档在 `design/history/20261022-before-secondary-unify/pages/`。

- 保留统一顶部返回和发布需求标题，无主导航底栏。去掉大外层表单框和单独告示卡。
- 真实字段顺序为选择游戏、需求标题、目标与验收标准、总预算。标签与细分隔形成层级；input44px、textarea140px，仅控件自身有淡填充/细边，禁止框中框。
- 标题上限40字、验收要求1000字、预算1～10000元且最多两位小数，保持原接口与校验。游戏切换不改写首页偏好。
- 选择游戏统一复用首页批准的共享GameSelector搜索/三列图标弹层，单选即选即回；取消保留所有需求草稿。打开状态见[publish-game-picker.design.md](publish-game-picker.design.md)与同名PNG，禁止恢复白色原生游戏滚轮。
- 支付说明与游戏规则使用小字及轻量图标，顺着正文流展示；不新增装饰大标题或重复插画。
- 唯一44px主操作“确认需求，去支付”；表单可自然滚动并预留底部安全空间。登录及手机号仍在发布前强制校验，不能视觉改造绕过。
- 保留单次提交锁、会话切换保护、待支付订单创建与原订单详情跳转；不要新增未实现的业务入口。

- 全局表单实施规格以本轮root的form-section为准：input透明背景/单底线44px，textarea低对比灰底8px；生成图的微弱全边框只作控件示意，不恢复外层大panel。

## 设计与实施约束

390×844首屏，内置ImageGen编辑后仅机械等比缩放。自然黑绿主题、16px水平边距、8/12/16/24间距、44px主操作与36px次操作，图标固定盒双轴居中。生成文字/比例误差以真实字段、此说明及系统安全区为准；所有业务内容实现为真实组件，实际验收图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program UI into the approved refined dark editorial design language: charcoal-green #0a1110, white#edf4ef, graygreen#9dafA3, mint#8de8b6. Natural restrained surfaces and thin separators, open typography and whitespace, no repeated big rounded outercards, no nested card-on-card, no dashed emptyframes, no giantbutton pair, no neon/blue/purplegradients. Keep all actual business fields/flows. 16pxhorizontalinsets,8/12/16/24spacing,44pxprimarybutton,36pxsecondary, officialstyleicons centered, readableChinese. Exact390:844portrait flat UI, no deviceframe,no perspective,no outside margins. Native WeChatstatusbar/capsule safearea must remain. Secondarypages use ONE compactsharedtopback/titleheader and NO primarybottomtabnav, unless explicitlysaid primarytab. Title发布需求. Preserveactualbusinessorderandallfields: 选择游戏 picker三角洲行动, 需求标题 input今晚找位耐心的三角洲陪练 (max40), 目标与验收标准 textarea explaining区服/时段/任务目标/时长/怎样算完成 (max1000), 总预算（元） input68.00. Reorderfromoldimage toactualfieldsequence game,title,requirements,budget. Removeoldlargeouterroundedformcardandseparatenoticecard; layoutdirectlyonbackgroundwithopenlabelsandthinsectiondividers. Gamepicker compactrow withsmallicon/chevron; textinput44px, textarea140px withsubtlefilledsurface/radius6notpanel-on-panel. Show small quiet two-linehelper 支付后进入接单大厅，未接单可取消；协作约定与沟通随订单保存. Rulesnotice simplemutedtextbelowhairline, notloudbox: 遵守游戏官方规则，请勿填写账号密码. Mainone44pxmint 确认需求，去支付 atbottomwithsafespace. No decorativehero/illustration, no redundantEnglish/slogan, no newfeatures. Preservephoneverificationrequirementbeforetransaction, notshownasextraforminput. Elegantprofessionalformwithclearvisualhierarchy.

## 2026-10-24 微信订阅入口

沿用已批准布局，仅加入共享紧凑铃铛文字入口。真实授权弹层遵循 `design/pages/account/subscriptions.png` 与同名说明；不修改主交易控件，不异步自动弹微信授权。服务发布成功仅提示打手已付款新单，打手订单页可订阅退款申请；买家仅接单结果，俱乐部负责人仅成员申请/旗下争议，平台仅新俱乐部申请。消息中心增加本人微信提醒设置，平台控制台增加独立配置入口。普通聊天、钱包、调价和图文审核不发微信订阅提醒，仍保留站内消息。
