# 有封面的玩家主页新版设计

文件作用：对应 `frontend/src/pages/community/players/detail.vue` 的先行设计图与实施约束；2026-10-22 根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/`。

- 仅当用户已设置profile_cover时使用本图。取消整条应用顶栏与“玩家主页”重复标题，用户封面从顶部沉浸开始，16:9全宽展示；左上36px圆形细边返回，避开状态栏/胶囊。分享动作同样避让胶囊。
- 封面下16px正向间距后，正常流展示头像、昵称、认证与简介，再显示统计、俱乐部和服务列表；禁止负margin、身份图层叠封面。
- 服务卡统一源图16:9、展示裁切约4:3。图底透明到主题背景不透明渐变，标签在上、标题在下，均为独立图层；自定义文字保持防越界/防碰撞。价格和成交放在图下正常流。
- 山景只是用户自设封面的示意，运行时禁止作为默认封面；无profile_cover时按detail.png的普通统一返回状态展示。
- 页面没有主底栏。使用真实用户/俱乐部/服务数据，保留分享和跳转；16px水平边距与统一图标/文字/控件规格。

## 生成与实施边界

使用内置ImageGen编辑同路径旧图后等比缩放至390×844。图片用于可读首屏设计，所有界面需实现为真实组件；生成文字误差以本说明和真实业务为准。图中省略的微信胶囊仍必须按系统API真实避让。实际验收截图单独放output，不得替换设计图。

## 完整编辑指令

Use case: ui-mockup. Edit the attached 390x844 Chinese WeChat mini-program design. Output one flat 390:844 portrait UI, no device frame, no outside margins. Preserve brand 开机电竞, dark charcoal-green #0a1110, white #edf4ef, muted graygreen #9dafA3, mint accent #8de8b6, fine restrained dividers, 16px horizontal insets and legible Chinese. Native statusbar and top-right WeChat capsule safe area are real constraints. Avoid generic AI dashboard appearance: no repeated big rounded boxes, no dashed placeholders, no decorative giant buttons, no blue/purple gradients. Official icons aligned centrally. Any game artwork is design-reference only. Primary tabs have only ONE persistent bottom nav 玩家 打手 订单 我的. This is a PUBLIC PLAYER PROFILE SECONDARY page with a user-uploadedcover, so NO primary bottomnav. Remove existing full topnavbar and centered 玩家主页 title. User scenic mountain-lake cover begins at y0 fullwidth behind native status/capsule with16:9 approximate ratio, no inset rounded frame. Overlay only circular36px subtle fineborder backbutton at x16 y52 with whitechevron; shareicon can be small belowcapsule, no bar. Below cover leave16px normal positiveflowgap then identity with medium52avatar 北辰, small俱乐部认证 tag, mutedintro 耐心沟通，认真履约; do not overlay avatar or identity oncover. Refined unboxed stats28已完成 5.0好评 3在售. Small singleline clubentry 山海电竞 已认证 with36pxlogo/chevron, mutedsubcaption. 发布的服务 section with two-column SINGLE servicecards. Each card16:9 art has transparent-to-opaque #121d18 verticalgradient at bottom; smallcertification/game labels ABOVE the bold service title, BOTH independently overlaid INSIDE image; below image only mintprice ¥68/小时 and 已完成12 normalflow, never a duplicate title outside. Examples 大坝任务协作 排位陪练 航天基地协作 全面战场上分. All titles/tags safely fitted, no overlap. The mountaincover is only illustrative of user's own asset, never product fallback. Clearimmersiveback and elegant highquality spacing.
