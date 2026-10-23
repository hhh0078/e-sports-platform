# main/orders/orders · 本轮设计

文件作用：对应前端同路径页面的本轮先行设计与实现依据。使用内置 ImageGen 对已认可设计局部编辑并机械等比缩小至约390×844；不以验收截图替代设计图。旧图和旧说明保存在 `design/history/20261023-before-deposit-ui/`。

统一要求：自然黑绿背景、当前游戏主题、16px水平边距、真实安全区；押金徽标独立采用黑金色，不随游戏主题变化。图中示例用户、金额、订单角标均不是运行数据，运行时只取服务端。生成文字误差以本说明及真实接口为准。正文不缩小来塞首屏，长内容自然滚动。

共享服务卡最新约定：标题在上、认证/游戏/分类标签在下，均为独立封面图层，渐变底部与卡片底色一致；价格在图下紧凑正常流。押金贴右上边缘，不留外边距，黑金色、真实实心盾牌，仅左下圆角；平台短标“押金”、用户向俱乐部实缴短标“俱乐部”，金额版写明对应押金和元。0/未知/退款中不显示。卡片自定义文字至少10px，详情至少14px；六条长文字可换行并适度增高，不缩成几像素或隐藏，保留九宫格意图并避开系统信息和押金角标。

## 页面布局与状态

订单空态改用新透明轻插画empty-orders-light.png，180×120px盒子，只有小纸票和小手柄，低饱和、不发光；标题18px，说明13px。保留身份/状态筛选、小字登录和去发现服务入口。插画只用于真正空态，真实列表不变。

## 生成提示词与实施边界

Use case: ui-mockup. Precise local edit of approved Chinese 开机电竞 WeChat mini-program screenshot. Preserve natural charcoal green #0a1110, panel #121d18, mint #8de8b6, white #edf4ef, 16px insets, centered icons, native safe areas. Flat390:844portrait, no frame, no outside margin, no blue/purple gradient. Change only the page requirements described above; keep game imagery and unrelated content. Runtime text and badges stay separate UI layers. Exact Chinese text and amounts follow the page requirements; sample values are illustrative only.

验收：320/390宽度分别核对短/长名称、0/正数/双押金、六条最长中英表情文字和九宫格位置；无碰撞、无水平溢出。真实微信头像、支付/转账到账仍须单独真机与商户验收。

## 合并后的主视图与品牌边界（2026-10-23）

公共品牌为开机电竞。订单主视图不显示应用顶栏、返回箭头或重复居中页名；在状态栏及胶囊安全区下放“我下的单 / 我接的单”身份切换，状态筛选限宽滚动，唯一底栏由主容器承载。访客可浏览，以小字“请登录后查看”提示。

空态继续使用本轮轻量180×120插画及轻入口，不恢复主分支较大、较重的工作台场景。真实订单列表、售后状态、调价记录和资金明细仍只用服务端数据；不得将设计示例当成真实交易。
