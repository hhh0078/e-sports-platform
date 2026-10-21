# 个人中心新版设计

文件作用：对应 `frontend/src/pages/main/me/me.vue` 的先行设计图与实施约束；2026-10-22 根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/`。

- 本页完全取消应用顶栏、重复“我的”标题与返回按钮。身份行从原生状态/胶囊安全空间下开始，微信胶囊区域不得放设置图标或昵称。
- 头像、昵称、简介与三列统计直接排在背景上，用字号、留白和细分隔形成层次；禁止全部套同规格大圆角盒子。
- 身份区右侧使用 `frontend/src/static/illustrations/profile-pass.png`，原创通行证/手柄小插画，透明16:9源图，80～120px宽等比展示，只作轻装饰，不压昵称、不触碰胶囊。
- 我的订单四个快捷项等分排列、图标双轴居中；我的主页/我的俱乐部为紧凑双入口；接单身份为普通开关行；资料、手机号、两协议、管理入口使用细分隔列表。
- 图为已登录示例。访客相同结构正常展示，昵称为“登录后查看”、统计0、手机号未验证；管理员入口仅真正管理员可见，所有入口仍按原业务做必要鉴权。
- 统一16px水平边距、8/12/16/24间距、44/36/28控件规格；底部唯一主导航由主容器提供。

## 生成与实施边界

使用内置ImageGen编辑同路径旧图后等比缩放至390×844。图片用于可读首屏设计，所有界面需实现为真实组件；生成文字误差以本说明和真实业务为准。图中省略的微信胶囊仍必须按系统API真实避让。实际验收截图单独放output，不得替换设计图。

## 完整编辑指令

Use case: ui-mockup. Edit the attached 390x844 Chinese WeChat mini-program design. Output one flat 390:844 portrait UI, no device frame, no outside margins. Preserve brand 集结, dark charcoal-green #0a1110, white #edf4ef, muted graygreen #9dafA3, mint accent #8de8b6, fine restrained dividers, 16px horizontal insets and legible Chinese. Native statusbar and top-right WeChat capsule safe area are real constraints. Avoid generic AI dashboard appearance: no repeated big rounded boxes, no dashed placeholders, no decorative giant buttons, no blue/purple gradients. Official icons aligned centrally. Any game artwork is design-reference only. Primary tabs have only ONE persistent bottom nav 玩家 打手 订单 我的. Redesign the personal-center main screen with distinctive typography and gentle illustration instead of a stack of similar boxes. Absolutely NO top app navbar, NO centered 我的 title and NO backarrow. Native status/capsule safe area reserved; first identity row begins y75: medium52 avatar left, nickname 云间旅人 with small subtle edit chevron, one short muted profile line. A small mint/ivory tactile3D gamebadge + controller illustration appears at upper right BELOW capsule, about70px, adds personality without big banner. Identity/three stats live directly on background, no enclosing roundedcard: 12下单记录 3进行中接单 8完成订单, mint numerals with quiet labels and thin vertical dividers. Under a fine separator, 我的订单 with quiet 全部 right, then four evenly spaced centered officialstyle icons 待付款 进行中 待验收 售后, NO giant outlined card. A single visual row with 我的主页 and 我的俱乐部 as clean compact editorial entries with smallicons and short mutedcaption; can use subtle alternating surfaces but avoid same large roundedboxes. An elegant 接单身份 switch row integrated with thin dividers, same normal44pxrow height. Lower accountlist on dark background with hairline separators: 个人资料, 手机号验证 138****5678, 用户协议, 隐私政策, 平台管理中心 (last only admin). Ensure every icon verticallycentered and chevrons line up. Place no decorative English. Guest variant same layout with login后查看 and stats0, no big loginblock. Bottom 我的 active. Balance distinct hierarchy, comfortable negative space, professional game-community look.

二次编辑：替换为项目正式透明插画，保留布局。
Use case: compositing / ui-mockup. Image1 is edit-target refinedChinese390x844 personalcenter; Image2 is finaltransparentillustration insert of ivorypassport/cardwithsagecontrollerandgentlenaturaltexture. Preserve EXACTLY alltargetUI layout, identity/statistics, finelydivided rows/ordericons/entrylinks/switchandbottomnav,allChineseandcolorhierarchy. Change ONLY upper-right glowinggreenbadge andcontrollerjungleart: replace with matching smallmatteivorycard-and-controller insertfromImage2, preservecompositionandtones,naturallyblendtransparentbackground. Size about120pxwide, avoidrightnativecapsulesafearea (y44-76) and keepidentityleftunobstructed. No glowingneonleafs,no backgroundbanner,no extraobjects. NO appnavbar orcentered我的title. Outputflat390:844portrait,no deviceframe,no purple/bluegradient.
