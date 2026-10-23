# main/player/player · 本轮设计

文件作用：对应前端同路径页面的本轮先行设计与实现依据。使用内置 ImageGen 对已认可设计局部编辑并机械等比缩小至约390×844；不以验收截图替代设计图。旧图和旧说明保存在 `design/history/20261023-before-deposit-ui/`。

统一要求：自然黑绿背景、当前游戏主题、16px水平边距、真实安全区；押金徽标独立采用黑金色，不随游戏主题变化。图中示例用户、金额、订单角标均不是运行数据，运行时只取服务端。生成文字误差以本说明及真实接口为准。正文不缩小来塞首屏，长内容自然滚动。

共享服务卡最新约定：标题在上、认证/游戏/分类标签在下，均为独立封面图层，渐变底部与卡片底色一致；价格在图下紧凑正常流。押金贴右上边缘，不留外边距，黑金色、真实实心盾牌，仅左下圆角；平台短标“押金”、用户向俱乐部实缴短标“俱乐部”，金额版写明对应押金和元。0/未知/退款中不显示。卡片自定义文字至少10px，详情至少14px；六条长文字可换行并适度增高，不缩成几像素或隐藏，保留九宫格意图并避开系统信息和押金角标。

## 页面布局与状态

首页主要结构与已有官方游戏素材不变。服务卡按上述新约定展示。四主视图底部自然流添加“该平台由开机电竞、瓜瓜提供技术支持”，点击共享说明弹层，主容器只挂一个弹层。

## 生成提示词与实施边界

Use case: ui-mockup. Precise local edit of approved Chinese 开机电竞 WeChat mini-program screenshot. Preserve natural charcoal green #0a1110, panel #121d18, mint #8de8b6, white #edf4ef, 16px insets, centered icons, native safe areas. Flat390:844portrait, no frame, no outside margin, no blue/purple gradient. Change only the page requirements described above; keep game imagery and unrelated content. Runtime text and badges stay separate UI layers. Exact Chinese text and amounts follow the page requirements; sample values are illustrative only.

验收：320/390宽度分别核对短/长名称、0/正数/双押金、六条最长中英表情文字和九宫格位置；无碰撞、无水平溢出。真实微信头像、支付/转账到账仍须单独真机与商户验收。

## 品牌名称更新与当前布局优先级（2026-10-23）

首页品牌为开机电竞，口号为“开机即组队，一起赢”；保留左文右发布操作、游戏目录、服务卡及原生安全区。右侧发布需求按钮约96～104px宽、36px高且不收缩，左右至少12px间距；320px时左侧文字自然换行，不能进入微信胶囊区域。游戏素材来自当前game.cover_url和已有官方资源。

游戏目录仍是一行横滑，固定“全部”打开共享GameSelector搜索网格；只在用户显式选择全局游戏时写本地偏好。底栏玩家手柄选中为当前主题accent，未选为次要灰。

共享押金短标以本轮规则为准：封面右上贴边、黑金实心盾牌、仅左下圆角；短标8px字、11px图标、最小高18px，背景rgba(24,21,14,0.76)，金额版保留11px字和24px最小高度。旧图下绿色钱包押金行、标签在标题上均不恢复；标题在上、标签在下。金额与资格只取当前服务端结果。

当前PNG以本分支最新功能图为目标，使用内置ImageGen仅将左上品牌改为开机电竞，再机械缩放至390×844；已逐图确认保留黑金右上角标及标题在标签上方。原图大标题保持输入图原样，实际产品口号以上述“开机即组队，一起赢”为准。不得为品牌回退卡片与押金布局。好友、朋友圈和实时海报继续共用分享入口及真实小程序码。

品牌局部编辑提示词：Use case: text-localization. Change only the top-left product brand 集结 to 开机电竞 in the approved 390x844 design. Preserve the latest top-right black-gold shield badges, title-above-tags cover layers, game artwork, prices, spacing and all other geometry. Do not restore the old image-below deposit row.

## 空态刷新稳定性（2026-10-23）

已成功查询的空服务状态在切标签回来和刷新等待时保持原布局；刷新故障放次要错误提示，不先缩为短加载行。首次、新游戏、分类、排序或搜索查询仍显示首次加载状态。沿用原设计，不新增图形和占位样式。

## 2026-10-24 下拉刷新与触底交互设计（先于实现）

沿用本页已批准PNG的布局、游戏素材、背景和安全区；个人中心以`me.background-v2.png`为当前头部基线。此次仅调整原生列表交互与底部分页状态，不新增装饰设计图、不改变已有卡片层级。

- 主容器继续四个绝对定位常驻scroll-view，非激活项只用visibility/pointer-events隔离；官方refresher仅作用当前可见容器，完成或失败后结束动画。不销毁滚动容器、不绑定scroll-top、不持续回写滚动位置。
- 玩家：下拉按当前游戏、分类、已提交搜索和排序刷新已加载范围；触底继续24条分页。打手：仅刷新当前我的服务／接单大厅／加入俱乐部子页签，触底20条分页；领取或暂停操作中不并发刷新。
- 订单：保留买卖身份、状态筛选，下拉刷新已加载范围，触底20条分页。我的：只刷新本人资料、统计和余额，无分页动作；刷新等待和失败保留已确认金额，不伪造零值。
- 正常“刷新／加载更多”按钮移除；列表末尾共用ListLoadState，普通状态由官方uni-load-more显示加载中、继续上拉或已到底，仅失败状态提供轻量重试。真正空态不堆叠多余的“已到底”。
- 单容器刷新与追加互斥，重复触底只发一次。刷新不先清空原列表或空态；失败保留数据和分页。筛选、游戏、子页签、账号变化或隐藏后拒绝旧响应，原始offset不因显示去重而倒退。
- 320／390宽验收自然下拉与触底、短／空列表、失败重试、切标签和详情返回；核实非零滚动位置保留、背景连续、原生刷新动画结束、无横向溢出。真实手势与程序触发结果分别记录。

列表查询上下文变化（分类／搜索／排序、打手内部页签、订单身份与阶段）立即撤销原容器的旧刷新动画和锁；旧网络回包仍由领域版本守卫拒绝，迟到finally不能关闭后来新发起的刷新。
