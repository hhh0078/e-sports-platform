# 订单主视图新版设计

文件作用：对应 `frontend/src/pages/main/orders/orders.vue` 的先行设计图与实施约束；2026-10-22 根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/`。

- 本页无应用顶栏、无“我的订单”大标题、无返回按钮。首行买卖身份切换紧凑左排，给原生右侧胶囊留足安全空间；状态栏/胶囊尺寸读取系统API。
- 我下的单/我接的单保留业务切换。状态筛选横向受控，可滚动但不能撑宽页面或挤压其他布局。
- 图为无订单状态：使用 `frontend/src/static/illustrations/empty-orders.png`，原创双手柄/地图/卷轴场景，透明3:2资源，按先行图实际视觉约260～300px宽等比展示；公共插画盒在常规宽度为300×200px，350px及以下窄屏为250×166.7px，避免PNG透明边缘使主体过小。插画空态标题常规20px、窄屏18px；无插画空态沿用原规格。禁止虚线空框、套圆图标或巨型登录按钮。
- 主提示“还没有订单”，说明一行；访客小字“请登录后查看”，配轻入口“去发现服务”。有订单时替换为真实列表，不重复空态插画。
- 正常订单卡保留进度、金额、订单详情与售后入口，角色权限、筛选状态、滚动位置均保留。底部主导航只由主容器挂载一次。
- 色彩统一自然黑绿，可随游戏accent切换；主次文字、图标和按钮尺寸沿用公共设计变量。

## 生成与实施边界

使用内置ImageGen编辑同路径旧图后等比缩放至390×844。图片用于可读首屏设计，所有界面需实现为真实组件；生成文字误差以本说明和真实业务为准。图中省略的微信胶囊仍必须按系统API真实避让。实际验收截图单独放output，不得替换设计图。

## 完整编辑指令

Use case: ui-mockup. Edit the attached 390x844 Chinese WeChat mini-program design. Output one flat 390:844 portrait UI, no device frame, no outside margins. Preserve brand 开机电竞, dark charcoal-green #0a1110, white #edf4ef, muted graygreen #9dafA3, mint accent #8de8b6, fine restrained dividers, 16px horizontal insets and legible Chinese. Native statusbar and top-right WeChat capsule safe area are real constraints. Avoid generic AI dashboard appearance: no repeated big rounded boxes, no dashed placeholders, no decorative giant buttons, no blue/purple gradients. Official icons aligned centrally. Any game artwork is design-reference only. Primary tabs have only ONE persistent bottom nav 玩家 打手 订单 我的. Redesign the orders main screen in EMPTY STATE, with no full app top navbar, no centered 我的订单 title, no back arrow. At top beneath native statusbar, compact left-aligned segmented tabs 我下的单 我接的单, about210px wide, safely left of native capsule at far right; the active tab has a small underline or subtle mint surface, not a giantbutton. Below at y105 one constrained horizontal status row 全部 待付款 进行中 待验收 售后, all fits390width. Empty body uses purposeful ORIGINAL illustration around y250-440: matte soft 3D gaming-order scene with mint clipboard/order ticket, little gamecontroller/headset, two small team markers and a gentle connection line, no text inside illustration, 180-210px wide, not a stock circle-icon. Open dark background, no large empty-card border and NO dashed outline. Under scene heading 还没有订单 and small muted helper 下单找队友，进度与沟通都在这里. One understated inline action 去发现服务 →, not giant full-widthbutton. Guest state uses 请登录后查看 in small text near heading, not forced loginwall. Keep plenty calm breathing space and clear hierarchy. Bottom 订单 active. Illustration is the visual focalpoint; do not fill with generic repeatedcards.

二次编辑：替换为项目正式透明插画，保留布局。
Use case: compositing / ui-mockup. Image1 is edit-target Chinese390x844 order empty-state UI; Image2 is final transparent illustration insert of paired gamecontrollers/map/scroll. Preserve EXACTLY targetlayout, allChineseorderrolefilters, emptyheading/helper, quietlink andbottomnav, darkgreenminttheme, spacing. Change ONLY the large centerorder-ticket/neon scene: replace with matching matte sage/ivory pairedcontrollers paperquestmap scroll illustration from Image2. Preserve distinctivecomposition/colors/papertextures and no textinart, transparent blendintodarkbackground, about230pxwideand153pxtall centered near y365, makecontentgroup verticallybalanced with headingbelowat500. No glowingclipboard, neon lines orgenericcircleicon. No surroundingcard/dashedborder, no giantbutton, no topnavbar. Outputflat390:844portrait,no deviceframe,no extraUI.
