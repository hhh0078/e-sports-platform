# community/clubs/detail · 本轮设计

文件作用：对应前端同路径页面的本轮先行设计与实现依据。使用内置 ImageGen 对已认可设计局部编辑并机械等比缩小至约390×844；不以验收截图替代设计图。旧图和旧说明保存在 `design/history/20261023-before-deposit-ui/`。

统一要求：自然黑绿背景、当前游戏主题、16px水平边距、真实安全区；押金徽标独立采用黑金色，不随游戏主题变化。图中示例用户、金额、订单角标均不是运行数据，运行时只取服务端。生成文字误差以本说明及真实接口为准。正文不缩小来塞首屏，长内容自然滚动。

共享服务卡最新约定：标题在上、认证/游戏/分类标签在下，均为独立封面图层，渐变底部与卡片底色一致；价格在图下紧凑正常流。押金贴右上边缘，不留外边距，黑金色、真实实心盾牌，仅左下圆角；平台短标“押金”、用户向俱乐部实缴短标“俱乐部”，金额版写明对应押金和元。0/未知/退款中不显示。卡片自定义文字至少10px，详情至少14px；六条长文字可换行并适度增高，不缩成几像素或隐藏，保留九宫格意图并避开系统信息和押金角标。

## 页面布局与状态

俱乐部实缴平台押金只在本主页显示；标识为黑金金额版。覆盖游戏放在标签右边，以已有官方游戏圆角图标叠放，最多5项，其余+N；不撑宽页面。成员列表改3列宫格，64px非人像头像、单行昵称省略、认证小角标。加入规则写成员押金、抽成、考核与支持换行的加入备注。公开主页仅保留管理俱乐部轻入口至console；原成员审核/团队订单保留，支持tab=manage/orders路由并由服务端确认权限。已申请或认证用户可进入成员押金页；在线客服联系只提供二维码/电话。

## 生成提示词与实施边界

Use case: ui-mockup. Precise local edit of approved Chinese 开机电竞 WeChat mini-program screenshot. Preserve natural charcoal green #0a1110, panel #121d18, mint #8de8b6, white #edf4ef, 16px insets, centered icons, native safe areas. Flat390:844portrait, no frame, no outside margin, no blue/purple gradient. Change only the page requirements described above; keep game imagery and unrelated content. Runtime text and badges stay separate UI layers. Exact Chinese text and amounts follow the page requirements; sample values are illustrative only.

验收：320/390宽度分别核对短/长名称、0/正数/双押金、六条最长中英表情文字和九宫格位置；无碰撞、无水平溢出。真实微信头像、支付/转账到账仍须单独真机与商户验收。

## 合并后的沉浸布局、品牌与分享（2026-10-23）

公共品牌统一为开机电竞。展示型二级页无整条应用顶栏、无重复“俱乐部主页”标题，也没有主底栏；封面16:9从顶部开始，左上36px圆形细边返回，分享入口避让微信胶囊。封面下至少16px正向间距后展示Logo、名称、主理人与真实统计，不使用负margin让身份侵入图片。

仅使用俱乐部实际上传的封面和图文。覆盖游戏、三列成员宫格、加入备注换行、独立控制台和三类押金按本轮规则实现；旧设计中的“平台不代收押金”已被平台独立托管规则替代，不能恢复。成员押金不计入俱乐部佣金，也不与俱乐部缴给平台的押金混同。

好友、朋友圈和实时海报使用共享分享入口；海报含真实小程序码及分享人头像昵称，不烘焙用户可编辑文案，不用示意二维码冒充可用码。配置缺失或正式页面未发布时明确说明失败。

## 2026-10-23 成员说明字数

申请说明和负责人审核说明维持原布局、明确最多500字，按去首尾空白后的Unicode码点校验。原生maxlength设为-1，避免H5按UTF16码元提前截断扩展字符；超限只轻提示精简，保留已填内容。
