# 服务详情沉浸式设计

文件作用：对应 `frontend/src/pages/services/detail.vue` 的先行设计图和新版实施约束；2026-10-22 根据用户最新视觉要求编辑。旧图和旧说明归档于 `design/history/20261022-before-immersive/pages/`。

- 展示型二级页取消整条应用顶栏。封面从顶部开始沉浸展示，状态栏与微信胶囊保留系统安全空间；左上圆形36px细边返回按钮浮在图片上，不能与胶囊重叠。
- 封面底部从透明渐变到主题背景完全不透明，标签行在上、标题行在下，均为独立悬浮文字。用户自定义文案保持独立图层，层间布局防重叠。
- 价格/成交数置于图下正常流；作者改为32px头像、昵称、认证和查看主页的单行紧凑信息条，不再占用大卡片。
- 服务说明、数量、预约要求和固定下单操作继续保留，内容自然滚动；无主导航底栏。安全区和底部操作高度应为正文预留空间。
- 水平16px，主按钮44px。封面游戏图为版式示意，产品运行使用已有官方素材。

- 服务封面源图保持16:9；本次新版图内展示区按设计约4:3裁切，为标签、标题及自定义文案留足空间。所有独立文字按实际显示宽高适配，不能按源图尺寸写死坐标。

## 生成方式与指令

使用内置 ImageGen 编辑现有同路径图片，完成后机械等比缩至390×844；先设计后实施。生成图中的文字误差和系统胶囊缺省，以本说明和真实系统安全区为准。

Use case: ui-mockup. Edit the attached existing 390x844 Chinese mini-program design. Deliver exactly one flat portrait UI screenshot, same 390:844 aspect, no device frame, no surrounding space. Preserve the restrained dark charcoal-green theme #0a1110, panels #121d18, mint #8de8b6, 16px insets, 12px radii, clean Chinese type, 44px primary buttons and fine centered icons. Preserve correct Chinese words and existing screen purpose. Native WeChat status bar and native top-right capsule are present; reserve capsule-safe space. This is a design reference only: game artwork remains illustrative and app runtime uses official assets. Do not invent blue/purple gradients. Primary request: make service-detail an immersive image-led secondary screen. REMOVE the existing full top navigation bar and centered 服务详情 title entirely. A tactical dam cover fills screen width starting at y0 behind native status and capsule; its height about300px. Only a circular back button at x16 y52, 36px, subtle fine light border and dark translucent background with centered white left chevron; never a full bar. Native WeChat capsule stays at top right and all meaningful art/text avoids it. At the bottom of image add the REQUIRED gradient from fully transparent upper image to fully opaque #121d18 at bottom. Inside image, at its lower edge, put independent labels 俱乐部认证 三角洲 任务协作 in a small row ABOVE a bold service title 三角洲 · 大坝任务协作, white 22px. Place user custom words 耐心沟通 稳定撤离 as small separate overlay chips above those labels if needed, do not bake typography into artwork. Below image a compact ordinary flow price row, ¥68/小时 left and 已成交236右. NO large standalone title/price panel. Replace existing huge author card with a single compact line about48px tall: small32px avatar 北辰 and a small mint verified icon, right chevron 查看主页; no big completed-count second line. Then elegant 服务说明 panel containing concise paragraphs and compact service traits. 下单信息 section with 数量（小时）, stepper, optional appointment textarea. Fixed bottom row ¥68合计 and mint 立即下单. No main bottom tab nav. Preserve all transaction content, readable calm spacing, no overlap.

## 2026-10-22自愿平台押金局部增补

作者行附近、封面之外，在真实provider_deposit_cents>0时显示“已缴平台押金 ¥实际金额”，小字“仅展示缴纳金额，不是赔付承诺”；0/未知不显示。保留俱乐部认证身份原有意义，不把平台押金解释为担保，也不侵入封面独立文案区。

完整局部编辑提示词：

Use case: ui-mockup. PreciseLOCALeditofattachedChinese390x844design. Preserveallunrelatedstyle/fields/layout/gameart/safeareas,approved#0a1110darknaturalgreenand#8de8b6mint, thinseparatorsandcentredofficialicons. No newfeatures, no giantcards, no Englishdecoration/no bluepurplegradient. AllnewtextindependentUI,nobakedgameart. Exact390:844portraitflatUI,noframe/nooutside margins. ChangeONLYnearcompactauthorrow: below/samedocflowbeneathsmallavatar北辰/verified/name/viewhome, addquietsmalltext 已缴平台押金¥100.00 asILLUSTRATIVEpositiveamount, withtinywalleticon, andsecondarynote 仅展示缴纳金额，不是赔付承诺. Renderactualamountfrombackendonly,zerodepositomitsentireline. Keeptextoutsidecover/customlabelareaandawayfromexistingclubcertification. No largebadgecard,noguarantee/担保/可赔付promises. DoNOTredesignunrelateddescription/orderfields/CTA orhero. Allowlowercontentscrollasneeded.
