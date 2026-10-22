# 个人资料 · 轻量表单

文件作用：对应 `frontend/src/pages/account/profile.vue` 的先行新版设计和实施约束；2026-10-22按用户新增全站统一要求编辑。上一版图与说明归档在 `design/history/20261022-before-secondary-unify/pages/`。

- 保留普通统一顶部返回和个人资料标题，无主底栏；移除包裹整套表单的圆角外框。
- 头像改为紧凑左对齐编辑行，右侧更换头像和1:1裁剪压缩提示。昵称、简介、手机号、接单身份依次开放分组，用细分隔和留白区分。
- 仅真正可编辑的input/textarea保留淡填充和细边；不再给手机号行/开关行重复套框。输入44px、简介80～96px，简介上限保持300字。
- 主页封面独立小节，替换为轻入口；源图及正常预览16:9，未设置时使用简洁文本，不用虚线占位框。图中窄山景只示意封面内容，实施应按真实比例并自然滚动，不能为把所有按钮挤入首屏而压扁图片。
- 保存资料为44px主操作，取消修改为文字轻操作；公开主页/退出登录保持轻入口，不与保存同等权重。
- 访客能正常看到空资料，编辑不可用时给小字登录提示；保留真实保存、撤销、图片裁剪压缩、手机号验证、接单开关与会话隔离逻辑。

- 全局表单实施规格以本轮root的form-section为准：input透明背景/单底线44px，textarea低对比灰底8px；生成图的微弱全边框只作控件示意，不恢复外层大panel。

## 设计与实施约束

390×844首屏，内置ImageGen编辑后仅机械等比缩放。自然黑绿主题、16px水平边距、8/12/16/24间距、44px主操作与36px次操作，图标固定盒双轴居中。生成文字/比例误差以真实字段、此说明及系统安全区为准；所有业务内容实现为真实组件，实际验收图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program UI into the approved refined dark editorial design language: charcoal-green #0a1110, white#edf4ef, graygreen#9dafA3, mint#8de8b6. Natural restrained surfaces and thin separators, open typography and whitespace, no repeated big rounded outercards, no nested card-on-card, no dashed emptyframes, no giantbutton pair, no neon/blue/purplegradients. Keep all actual business fields/flows. 16pxhorizontalinsets,8/12/16/24spacing,44pxprimarybutton,36pxsecondary, officialstyleicons centered, readableChinese. Exact390:844portrait flat UI, no deviceframe,no perspective,no outside margins. Native WeChatstatusbar/capsule safearea must remain. Secondarypages use ONE compactsharedtopback/titleheader and NO primarybottomtabnav, unless explicitlysaid primarytab. Title个人资料. Preserveactualfieldsbutremovebigouterformcard and repetitiveboxes. Top compactleftaligned56pxavatar withsmallcamerabadge, right 更换头像 and muted 头像1:1，自动裁剪压缩, nothugecenteredprofilephoto. Cleanopenformsections separatedbywhitespaceandhairlines: 昵称44pxinput云间旅人; 一句话介绍 textarea认真游戏，快乐同行 height80-96px withcounter10/300, not50; 手机号验证 singleordinaryrow 已验证138****5678 andchevron; 接单身份 ordinaryswitchrow 我可以接单 withsmallmutedhelper. Inputs may haveone subtle lowcontrastfill/hairline but no enclosingroundedgroup. Below asection个人主页封面 withquiet更换封面textlink andsmall16:9actualuseruploadedcoverpreview (can showonlytopportionatbottomoffirstscreen ifneeded); NO dashedplaceholderbox. Ifnocover just quietneutraltext未设置封面andlight选择封面entry. Bottom44px保存资料 primary and compact取消修改lightaction, 查看我的公开主页 and退出登录 quiettextlinks lowerdownnatural-scroll, notallcrammed. No decorativeillustrationneededhere. Keepprofileediting, phoneauthorization andproviderflows, clearalignedlabels,minimaleditingchrome.
