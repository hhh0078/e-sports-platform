# 主页面容器

文件作用：主容器 `pages/main/index.vue` 的设计与挂载契约；视觉参考同目录四个子视图，index.png同步玩家页。

- 唯一主容器常驻玩家、打手、订单、我的四视图，唯一底栏切换激活态，不reLaunch重建。
- 四主视图完全没有应用顶栏、重复页面标题和返回按钮；只统一提供原生状态栏与胶囊安全空间，首行业务内容避开胶囊。
- 服务卡源图16:9、实际图内显示裁切约4:3；标签在上、标题在下独立悬浮在图片底部渐变区，价格和成交正常流位于图下。
- 访客正常查看空数据；各视图状态、滚动、筛选保留。二级页面独立路由，无主底栏。
- 图片开头的展示型二级页使用沉浸圆返回；无用户封面的个人主页及表单/配置页继续普通统一顶部返回。

## 品牌名称更新（2026-10-23）

主容器参考玩家首页：品牌为开机电竞，口号为开机即组队，一起赢，既有布局和主导航不变。 本次只局部替换品牌文字，不新增界面、控件、图片或视觉规则。历史图保留原名称。

编辑提示词：Use case: text-localization. Edit only the product brand text in the attached existing 390x844 design. Use exact Chinese brand 开机电竞. 主容器参考玩家首页：品牌为开机电竞，口号为开机即组队，一起赢，既有布局和主导航不变。 Keep all unrelated layout, artwork, colors, type sizes, fields, amounts, safe areas and dimensions unchanged. No redesign, stretching, added height or new decorations.
