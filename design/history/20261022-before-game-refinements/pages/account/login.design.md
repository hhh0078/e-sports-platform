# 登录 · 开放式品牌页

文件作用：对应 `frontend/src/pages/account/login.vue` 的先行新版设计和实施约束；2026-10-22按用户新增全站统一要求编辑。上一版图与说明归档在 `design/history/20261022-before-secondary-unify/pages/`。

- 保留普通统一顶部返回和登录集结标题、轻量先逛逛入口；无主导航底栏。使用开放排版和有效平台文案，不套大卡片、不增加无意义英文或重复标题。
- 平台文案为“热爱游戏，自由接单”“无需挂靠，凭实力找到队友”。图中两行大字属于登录核心说明，实施可沿用现有紧凑字号，避免挤满小屏。
- 主视觉复用真实纸感 `empty-services.png`，与图稿保持一致，不保留此前游戏拼图。图片只承载图像，不烘焙页面文字。
- 三项卖点采用小图标与文字轻排版。只有微信一键登录一个44px主操作；两协议复选框必须主动勾选，链接可阅读，绝不能默认同意。开发体验仅本地模式轻入口。
- 原有微信code换身份、协议版本更新、错误重试、可关闭头像昵称完善弹层保留；不增加手机号/密码登录表单，不改变登录后的next站内回跳。
- 可选头像昵称弹层延续相同开放分组，关闭/稍后完善为轻文字入口；不展示密钥或OpenID。

## 设计与实施约束

390×844首屏，内置ImageGen编辑后仅机械等比缩放。自然黑绿主题、16px水平边距、8/12/16/24间距、44px主操作与36px次操作，图标固定盒双轴居中。生成文字/比例误差以真实字段、此说明及系统安全区为准；所有业务内容实现为真实组件，实际验收图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program UI into the approved refined dark editorial design language: charcoal-green #0a1110, white#edf4ef, graygreen#9dafA3, mint#8de8b6. Natural restrained surfaces and thin separators, open typography and whitespace, no repeated big rounded outercards, no nested card-on-card, no dashed emptyframes, no giantbutton pair, no neon/blue/purplegradients. Keep all actual business fields/flows. 16pxhorizontalinsets,8/12/16/24spacing,44pxprimarybutton,36pxsecondary, officialstyleicons centered, readableChinese. Exact390:844portrait flat UI, no deviceframe,no perspective,no outside margins. Native WeChatstatusbar/capsule safearea must remain. Secondarypages use ONE compactsharedtopback/titleheader and NO primarybottomtabnav, unless explicitlysaid primarytab. Title 登录集结. Make login match approved paperillustration/quiet openpersonalcenter rather than gameposterlandingpage. Preservebackandquiet先逛逛entry. Removeoldthree-panelgamecharactermontage entirely. Uppercopy in openbackground: small集结brand, strongbutclean2-lineheadline 热爱游戏 / 自由接单, mutedsubline 无需挂靠，凭实力找到队友. One centralmatteivory/sage gamingheadset/controller/taskcard illustration usingImage2actualasset, about220x147px, no surroundingcard, nofakefigures, no newgamecharacters. Below show3smallunboxedicon+labelitems 自由接单 订单留证 俱乐部认证, nooversizedcirclebadges. Mainaction isONE44pxmintbutton 微信一键登录 withsmallWeChaticon. Underit mandatoryuncheckedsmallcheckbox plus 我已阅读并同意《用户协议》和《隐私政策》 links, cleanwrap at390and320width. No phone/passwordinputs, no autoagreedcheckbox, no loginwallbeforethispage. Developmentexperience is onlyquiettextlink in localmode and neednotshowinproductiondesign. Loggedinsuccessoptionalavatar/nicknamesheet remainsseparate, notshown. Openlayout, precisehierarchy, calmwarmoriginalillustration, no repetitivepanelborders. Keepallreadable.
