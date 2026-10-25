# 公共信息与文章接口契约

文件作用：固定本次公共信息、文章、跳转搜索和公告已读接口，供前后端共同实现；所有响应仍采用既有 `{ data: ... }` 外层，错误沿用既有HTTP状态及error字段。

## 公共模型

```ts
type ContentTarget = { kind: 'service' | 'request' | 'article'; id: string }
type ContentSlide = {
  id: string
  kind: 'image' | 'game_slot'
  image_url: string
  labels: CoverLabel[] // 沿用现有独立图层，最多6条；游戏槽必须为空
  target: ContentTarget | null
}
type ContentAnnouncement = {
  id: string // 固定home；版本由服务器维护
  version: number
  enabled: boolean
  frequency: 'always' | 'once'
  mode: 'image' | 'text'
  image_url: string
  title: string
  body: string // 纯文本，保留换行
  target: ContentTarget | null
}
type PageTip = { text: string; article_id: string } // 空article_id表示无跳转
type PageTips = Record<'player' | 'worker' | 'orders' | 'me', PageTip>
type AdminPublicContent = {
  revision: number
  global_slides: ContentSlide[]
  game_slides: Record<string, ContentSlide[]>
  announcement: ContentAnnouncement
  page_tips: PageTips
  service_notice: string
  request_notice: string
  updated_at: string
}
type PublicContent = {
  revision: number
  game_id: string
  slides: ContentSlide[] // 已按全局排序展开当前游戏槽，只有image项
  announcement: ContentAnnouncement
  page_tips: PageTips
  service_notice: string
  request_notice: string
}
```

全局最多12项，`game_slot`最多一个；各游戏最多12张。数组顺序就是排序，图片项必须保留独立labels，不能合成进图片。默认一个游戏槽；当前游戏没有配置图片时只回退当前游戏的CoverURL，不使用服务封面池轮播。显式保存空全局数组表示不显示轮播。

仅三个跳转类型；无跳转用null。图片项id可由客户端提供稳定标识，空值由服务器生成。game_slot不允许图片、文字或跳转。平台与游戏轮播图片使用 `promotion_banner` 用途，公告图片使用 `announcement_image`，保留原比例且不裁剪；轮播按16:9官方裁剪。

页面提示每项最多300字；两项注意事项各最多2000字，保留换行；公告标题最多60字、正文最多3000字。文字和图片均接入现有后置审核。

## 运营配置接口

| 接口 | 权限 | 数据 |
|---|---|---|
| `GET /api/public-content?game_id=delta` | 访客 | PublicContent；缺省game_id选第一个启用游戏，明确无效游戏返回404 |
| `GET /api/admin/public-content` | 平台管理员 | AdminPublicContent |
| `PATCH /api/admin/public-content` | 平台管理员 | 必填revision；其它顶层字段可选，合并已提供字段；game_slides按已提供游戏键合并，空数组清空该游戏；返回完整AdminPublicContent |
| `GET /api/me/announcement-state?version=N` | 登录 | `{id:'home',version:number,seen:boolean}`；不写已读 |
| `POST /api/me/announcement-ack` | 登录 | `{id:'home',version:N}`；幂等返回`{ok:true}` |

修改公告正文、图片、标题、模式、频率或跳转会生成新版本；只开关enabled不改变版本。客户端不控制公告版本。版本冲突返回409，前端刷新并保留草稿。公告真正展示成功后才ack；访客按版本保存本地标记；登录后的已读记录按账号在服务器保留。frequency=always忽略seen；frequency=once使用seen。双设备同时首次打开可能各显示一次。

## 文章模型与接口

```ts
type ArticleStatus = 'draft' | 'published' | 'hidden'
type ArticleSummary = {
  id: string; title: string; summary: string; cover_url: string
  status: ArticleStatus; revision: number
  created_at: string; updated_at: string; published_at: string
}
type AdminArticle = ArticleSummary & { body_markdown: string }
type PublicArticle = ArticleSummary & { body_html: string }
type ArticleInput = {
  title: string; summary: string; body_markdown: string
  cover_url: string; status: ArticleStatus; revision?: number
}
type ArticlePage = { items: ArticleSummary[]; total: number }
```

| 接口 | 权限 | 数据 |
|---|---|---|
| `GET /api/articles?q=&limit=20&offset=0` | 访客 | ArticlePage，仅published |
| `GET /api/articles/{id}` | 访客 | PublicArticle；草稿、隐藏、缺失均404 |
| `GET /api/admin/articles?q=&status=&limit=20&offset=0` | 平台管理员 | ArticlePage，状态可为draft/published/hidden或不填 |
| `GET /api/admin/articles/{id}` | 平台管理员 | AdminArticle |
| `POST /api/admin/articles` | 平台管理员 | ArticleInput；返回AdminArticle，revision从1开始 |
| `PATCH /api/admin/articles/{id}` | 平台管理员 | 完整ArticleInput及必填revision；返回AdminArticle；下架使用status=hidden |
| `POST /api/admin/articles/preview` | 平台管理员 | `{body_markdown:string}` → `{body_html:string}`，不保存、不创建审核任务 |

标题2—60字、摘要0—300字；Markdown正文至少1字且UTF-8不超过48KiB；最多20张正文图。图片可留空；文章封面使用`article_cover`（16:9），正文图使用`article_image`（保留原比例，宽不超过1280），均走现有媒体上传入口。

仅文章创建、修改和预览接口的JSON传输预算为304KiB（正文最坏六倍JSON转义加16KiB元数据），解码后正文仍严格限制48KiB UTF-8。其它普通接口保持64KiB；两种预算使用相同的未知字段、尾部JSON拒绝规则。普通中文换行安全渲染为`br`，不要求输入额外空格。

Markdown使用成熟解析器；禁止原始HTML、危险URL、外部图片和私密证据图片。正文图只允许本站内置图片或本用途托管图。公开接口仅返回当前内容生成的安全body_html，不返回编辑者私密信息或旧HTML缓存。普通HTTPS文本链接可安全呈现；运营跳转目标仍严格限定三类。

## 跳转目标搜索

`GET /api/admin/content-targets?kind=service|request|article&q=关键词&limit=20&offset=0`，仅平台管理员。返回 `{items: ContentTargetOption[], total:number}`：

```ts
type ContentTargetOption = ContentTarget & {
  title: string; image_url: string; game_id: string; subtitle: string
}
```

搜索在数据库分页前完成，支持标题和ID；仅返回目前可公开访问的正式服务、已付款待接单需求、已发布文章。目标下架或失效后，公开配置会撤销其跳转；保存时同一字段／同一游戏及轮播ID未修改的旧失效跳转会自动清除，不阻断其它分节保存，新增或改变的跳转（包括复制到新对象）仍须严格校验；目标详情再次校验公开状态，不泄露订单参与者、完整原始需求或支付资料。

## 实际管理俱乐部

现有 `GET /api/clubs` 新增 `managed=1`，要求登录，仅返回该账号实际负责或持有效副管理员身份的俱乐部；平台管理员身份本身不等同于负责全部组织。沿用q、limit和offset，先筛选完整管理集合再分页，返回既有Club数组，membership_status为owner或deputy。旧mine=1仍是创建及成员关系，不与managed混用。
