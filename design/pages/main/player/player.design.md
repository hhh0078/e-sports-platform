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
