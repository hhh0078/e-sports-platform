# 个人中心 · 当前游戏灰蒙版

文件作用：对应 `frontend/src/pages/main/me/me.vue` 的先行新版设计和实施约束；2026-10-22按用户新增全站统一要求编辑。上一版图与说明归档在 `design/history/20261022-before-secondary-unify/pages/`。

- 最新个人中心方向：顶部使用当前游戏的 `game.cover_url` 全宽背景，约240px（含系统安全区；按真实设备可自然增高），覆盖中性灰黑透明蒙版，并在底部自然退到主题背景。禁止写死三角洲素材；设计图只用其官方图示意。
- 四主视图均无应用顶栏、重复标题和返回按钮。头像昵称从状态栏/胶囊下开始，64px头像、昵称与简短简介形成干净身份行；微信胶囊真实尺寸以系统API为准。
- 三列统计在身份行之后正常流排列，位于hero底部；不能叠到头像或昵称上，也不能悬浮为独立大卡片。移除顶部通行证插画，避免和游戏背景叠加杂乱。
- 下方依次为我的订单四入口、我的主页/我的俱乐部双入口、接单身份开关、细分隔账户菜单；保留原有权限/数据/手机号与协议流程。
- 访客同样显示游戏背景和完整结构，昵称“登录后查看”、数据0、手机号未验证；管理入口仅实际管理员可见。
- 16px水平边距、8/12/16/24间距，轻入口与中心对齐图标。主导航仍由唯一主容器提供，数据、滚动与会话竞态保护不改。

## 设计与实施约束

390×844首屏，内置ImageGen编辑后仅机械等比缩放。自然黑绿主题、16px水平边距、8/12/16/24间距、44px主操作与36px次操作，图标固定盒双轴居中。生成文字/比例误差以真实字段、此说明及系统安全区为准；所有业务内容实现为真实组件，实际验收图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program UI into the approved refined dark editorial design language: charcoal-green #0a1110, white#edf4ef, graygreen#9dafA3, mint#8de8b6. Natural restrained surfaces and thin separators, open typography and whitespace, no repeated big rounded outercards, no nested card-on-card, no dashed emptyframes, no giantbutton pair, no neon/blue/purplegradients. Keep all actual business fields/flows. 16pxhorizontalinsets,8/12/16/24spacing,44pxprimarybutton,36pxsecondary, officialstyleicons centered, readableChinese. Exact390:844portrait flat UI, no deviceframe,no perspective,no outside margins. Native WeChatstatusbar/capsule safearea must remain. Secondarypages use ONE compactsharedtopback/titleheader and NO primarybottomtabnav, unless explicitlysaid primarytab. Primary request overrides: this is primarytab个人中心, so NO apptopnav, NO 我的 title, NO backarrow; onepersistentbottomnav with我的active. Replace existing upper-rightpassportillustration completely with a FULL WIDTH CURRENTGAME BACKGROUND HERO approximately240px tall fromtopincluding native-safearea. Image2 is actualexamplegamecover: showitcroppednaturally behindhero, acrosswholewidth. Put a neutral gray-black translucentoverlay overentirehero, plus asoftnaturalverticalfade atbottom into #0a1110, nevergreen/neonglow. Identityrow beneathnativecapsule (abouty100) has64pxavatar atx16, whiteboldnickname云间旅人 and shortmutedbio认真游戏，快乐同行 atx96, subtlechevron; cleanreadable nooverlapwithcapsule. Stats are normalflow BELOW identity inside hero, atabouty200, threerefinedcolumns12下单记录3进行中接单8完成订单; doNOT overlaystatsontopofavatar/nickname and doNOT makefloatingstatcards. This hero uses selectedgame'sdynamiccover at runtime; doNOT brand asfixed三角洲 and doNOT addgamecovercaptions. Removeoldprofile-pass illustration entirely toavoidbusybackground. Afterhero add我的订单 with4centeredquickicons, dual compact我的主页/我的俱乐部entries,接单身份switch,fineseparatoraccountlist个人资料/手机号验证/用户协议/隐私政策/平台管理中心. Keep lowerbodydesignfromtarget, justreflowaccordingnewhero. Maintain meaningfulcontent, normalflow, no giantcards, safe bottomnav. Activegameaccentcanchangewhileoverlayneutralgray.
