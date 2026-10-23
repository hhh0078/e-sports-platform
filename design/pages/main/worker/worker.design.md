# main/worker/worker · 本轮设计

文件作用：对应前端同路径页面的本轮先行设计与实现依据。使用内置 ImageGen 对已认可设计局部编辑并机械等比缩小至约390×844；不以验收截图替代设计图。旧图和旧说明保存在 `design/history/20261023-before-deposit-ui/`。

统一要求：自然黑绿背景、当前游戏主题、16px水平边距、真实安全区；押金徽标独立采用黑金色，不随游戏主题变化。图中示例用户、金额、订单角标均不是运行数据，运行时只取服务端。生成文字误差以本说明及真实接口为准。正文不缩小来塞首屏，长内容自然滚动。

共享服务卡最新约定：标题在上、认证/游戏/分类标签在下，均为独立封面图层，渐变底部与卡片底色一致；价格在图下紧凑正常流。押金贴右上边缘，不留外边距，黑金色、真实实心盾牌，仅左下圆角；平台短标“押金”、用户向俱乐部实缴短标“俱乐部”，金额版写明对应押金和元。0/未知/退款中不显示。卡片自定义文字至少10px，详情至少14px；六条长文字可换行并适度增高，不缩成几像素或隐藏，保留九宫格意图并避开系统信息和押金角标。

## 页面布局与状态

保留用户已删除顶部身份、统计和大插画的改动，不恢复旧元素。三个同级页签“我的服务 / 接单大厅 / 加入俱乐部”；第三页签展示56px非人像俱乐部头像、名称、人数、一行介绍省略和箭头；移除原底部发现俱乐部大banner。公开俱乐部无需登录即可浏览，分页与切号保护保留。

## 生成提示词与实施边界

Use case: ui-mockup. Precise local edit of approved Chinese 开机电竞 WeChat mini-program screenshot. Preserve natural charcoal green #0a1110, panel #121d18, mint #8de8b6, white #edf4ef, 16px insets, centered icons, native safe areas. Flat390:844portrait, no frame, no outside margin, no blue/purple gradient. Change only the page requirements described above; keep game imagery and unrelated content. Runtime text and badges stay separate UI layers. Exact Chinese text and amounts follow the page requirements; sample values are illustrative only.

验收：320/390宽度分别核对短/长名称、0/正数/双押金、六条最长中英表情文字和九宫格位置；无碰撞、无水平溢出。真实微信头像、支付/转账到账仍须单独真机与商户验收。

## 品牌名称更新与当前布局优先级（2026-10-23）

品牌说明统一为“在开机电竞，用游戏技能赚取真实回报”。只替换公共名称，不恢复用户已删除的顶部身份、经营统计、大插画或底部俱乐部banner；继续使用“我的服务 / 接单大厅 / 加入俱乐部”三个同级页签与本轮轻量俱乐部条目。当前PNG按最新功能结构优先；若小字仍有旧品牌，以本说明文案为准，不能选用旧布局图片覆盖。

所有用户默认具备接单身份且不可关闭，无休息中、去开启或账户启停开关；单个服务的审核、上架与暂停仍是独立业务。访客能浏览公开服务及俱乐部，不做整页登录墙；手机号、冻结、资料和实际交易权限继续由服务端核验。

接单大厅游戏选择复用共享GameSelector与已批准首页弹框风格，限宽滚动，不由组件擅自改写全局偏好。底栏只有打手为当前主题accent，图标双轴居中。

品牌局部编辑提示词：Use case: text-localization. Replace only 在集结，用游戏技能赚取真实回报 with 在开机电竞，用游戏技能赚取真实回报. Preserve the latest three tabs, club rows, compact publish entry and geometry. Do not restore profile statistics, giant illustration or club banner.
