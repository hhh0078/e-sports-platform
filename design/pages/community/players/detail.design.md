# community/players/detail · 本轮设计

文件作用：对应前端同路径页面的本轮先行设计与实现依据。使用内置 ImageGen 对已认可设计局部编辑并机械等比缩小至约390×844；不以验收截图替代设计图。旧图和旧说明保存在 `design/history/20261023-before-deposit-ui/`。

统一要求：自然黑绿背景、当前游戏主题、16px水平边距、真实安全区；押金徽标独立采用黑金色，不随游戏主题变化。图中示例用户、金额、订单角标均不是运行数据，运行时只取服务端。生成文字误差以本说明及真实接口为准。正文不缩小来塞首屏，长内容自然滚动。

共享服务卡最新约定：标题在上、认证/游戏/分类标签在下，均为独立封面图层，渐变底部与卡片底色一致；价格在图下紧凑正常流。押金贴右上边缘，不留外边距，黑金色、真实实心盾牌，仅左下圆角；平台短标“押金”、用户向俱乐部实缴短标“俱乐部”，金额版写明对应押金和元。0/未知/退款中不显示。卡片自定义文字至少10px，详情至少14px；六条长文字可换行并适度增高，不缩成几像素或隐藏，保留九宫格意图并避开系统信息和押金角标。

## 页面布局与状态

无封面时保留普通返回顶栏。身份区昵称下显示已认证俱乐部，只有无俱乐部时才写自由打手。身份下独立显示平台实缴押金和各俱乐部实缴押金（带俱乐部名称与跳转）；多个俱乐部自然换行，不把加入门槛当实缴。共享服务卡按最新约定展示，不添加假统计。

## 生成提示词与实施边界

Use case: ui-mockup. Precise local edit of approved Chinese 集结 WeChat mini-program screenshot. Preserve natural charcoal green #0a1110, panel #121d18, mint #8de8b6, white #edf4ef, 16px insets, centered icons, native safe areas. Flat390:844portrait, no frame, no outside margin, no blue/purple gradient. Change only the page requirements described above; keep game imagery and unrelated content. Runtime text and badges stay separate UI layers. Exact Chinese text and amounts follow the page requirements; sample values are illustrative only.

验收：320/390宽度分别核对短/长名称、0/正数/双押金、六条最长中英表情文字和九宫格位置；无碰撞、无水平溢出。真实微信头像、支付/转账到账仍须单独真机与商户验收。

## 2026-10-24 原生下拉刷新与触底加载

沿用本页已批准设计图、原生页面滚动、主题与版式，不新增嵌套滚动容器。下拉只刷新当前可读数据，所有成功、错误、访客、权限拒绝和被保护的忙碌状态都结束原生刷新动画；不调用支付、审核、创建、撤销等写接口。已有数据刷新期间保留，失败可重试；同一列表触底只允许一条请求，刷新或筛选后的旧页不得追加。分页页脚统一使用官方uni-load-more的轻状态，只有失败时显示重试入口，移除普通“加载更多”按钮。
玩家服务按服务器原始offset分页并去重，展示资料在追加时不反复重建。失败保留既有名片与服务，匿名朋友圈继续只调公开GET。

错误重试统一恢复刚才失败的首读或追加请求，不根据旧页脚推断；搜索输入是草稿，触底不得自动提交新关键词，需明确搜索后才应用。页面身份和筛选变化同步撤销旧手势加载状态，原业务版本守卫继续隔离迟到回包。
