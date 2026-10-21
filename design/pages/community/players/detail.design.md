# 无封面的玩家主页新版设计

文件作用：对应 `frontend/src/pages/community/players/detail.vue` 的先行设计图与实施约束；2026-10-22 根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/`。

- 用户未设置profile_cover时参考detail.png，保留普通统一顶部返回、居中标题及必要分享按钮，避开微信胶囊；不凭空增加默认大封面，无主底栏。
- 用户有profile_cover时改为detail-cover.png及对应说明：封面沉浸起始、图上圆返回、图下身份正常流。两种状态不能同时套顶栏。
- 身份、简介、三列统计与认证俱乐部按正常流展示，保持真实数据和业务权限。
- 服务卡统一源图16:9、展示裁切约4:3。图底透明到主题背景不透明渐变，标签在上、标题在下，均为独立图层；自定义文字保持防越界/防碰撞。价格和成交放在图下正常流，不重复图外标题。
- 游戏图仅示意，使用项目已核验的官方资产；图片文字不合成进资产。统一16px水平边距、8/12/16/24间距、44/36/28控件规格。

## 生成与实施边界

使用内置ImageGen编辑同路径旧图后等比缩放至390×844。图片用于可读首屏设计，所有界面需实现为真实组件；生成文字误差以本说明和真实业务为准。图中省略的微信胶囊仍必须按系统API真实避让。实际验收截图单独放output，不得替换设计图。

## 完整编辑指令

Use case: ui-mockup. Edit the attached exact Chinese 390x844 player profile design. Keep native statusbar, ordinary shared app navbar with back chevron, centered 玩家主页 and small share icon, because this user has NO custom profile cover. Do NOT add a cover, no bottom primarynavigation. Keep the existing identity row 北辰, smallclubcertification, bio, three unboxed stats, single compact 山海电竞 clubentry, 发布的服务 heading. Preserve charcoalgreen#0a1110, panel#121d18, mint#8de8b6,16pxinsets and centered officialstyleicons. Change ONLY the service cards to the latest unified layout: each card is ONE component with game image at top; image bottom has a transparent-at-top to fullyopaque darkthemebackground gradient. Small certification/game labels are independently overlaid in a row ABOVE the service title, and the bold title is INSIDE image nearbottom. Userdefined 耐心沟通 chip remains an independent safe positioned layer, never merged intoasset and never collideswith title/tags. Below image only mintprice ¥68/小时 and 已完成12 normalflow, no duplicatetitle, no certificationblock. Examples 大坝任务协作 排位陪练 航天基地协作 全面战场上分 机密行动协作 跑刀资源陪玩. Keep readable compact cards two columns and all existing information hierarchy. Exactaspect390:844 flat raster UI, no deviceframe,no marginsoutside,no bluepurplegradients. Image is designreference; runtimeart uses existingofficialassets.
