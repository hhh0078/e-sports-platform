# 游戏轮播默认文案与显式补齐

文件作用：说明13款游戏两段独立文字、只补缺失内容的后端规则、一次性运维命令及审核和缓存边界。

默认游戏槽没有配置游戏轮播时，使用当前游戏封面和两段固定短文案。文字始终是`CoverLabel`，不合入图片；主文案左上24，副文案右下16，背景均为`transparent`。当前内置封面使用已确认对比色；未知游戏仅使用固定通用短句，不拼接可编辑的游戏名称。

| 游戏ID | 主文案 | 副文案 | 主色 / 副色 |
| --- | --- | --- | --- |
| honor | 峡谷开黑有队友 | 双排陪练一起练好配合 | #FFFFFF / #FFF0D1 |
| peace | 战术组队一起出发 | 刚枪跑图都能找到搭子 | #102027 / #DDF7FF |
| delta | 撤离路上有人同行 | 跑图练枪任务一起协作 | #FFFFFF / #00E59B |
| tft | 双人作战默契上场 | 阵容运营陪你一起琢磨 | #172019 / #FFF6DE |
| valorant | 配合打好每个回合 | 练枪战术一起耐心复盘 | #FFFFFF / #FFABB2 |
| lol | 召唤师峡谷找队友 | 对线团战都有陪练搭子 | #FFFFFF / #C9AA71 |
| valorantm | 掌上集结配合开局 | 地图战术陪你逐局练习 | #FFFFFF / #FF7880 |
| cfm | 火线集结并肩开黑 | 枪法地图一起边玩边练 | #FFFFFF / #F28C51 |
| arena | 暗区探索结伴同行 | 熟悉地图练好撤离配合 | #FFFFFF / #E0AA62 |
| codm | 战术小队随时集结 | 枪法身法一起练好配合 | #FFFFFF / #D5D96A |
| identityv | 庄园相约默契配合 | 追逃练习一起复盘细节 | #FFFFFF / #C4B598 |
| narakam | 江湖结伴切磋同行 | 连招身法都有耐心陪练 | #FFFFFF / #D68D72 |
| eggy | 蛋搭子一起闯关 | 地图探索欢乐组队同行 | #2B1D24 / #FFF0DC |

已有显式配置的轮播在公开读取时保持原样，包括明确无字图片。只有没有游戏分组或空分组才使用默认回退；当前审核拒绝的分组不通过回退重新填字。

游戏轮播可引用对应游戏当前配置的公开托管封面；显式补齐、默认初始化和普通后台保存共用同一图片校验。游戏后来换封面时，同分组同图层的未改旧引用继续保留，但每次仍检查原图公开、非订单、未撤销且有实际字节。新增图层、改图或跨游戏复制仍校验运营上传用途和归属，平台轮播与公告不享受游戏封面例外。

## 显式命令

```sh
docker compose exec -T api /app/jijie-default-content fill-game-captions --actor-id <已有平台根管理员ID>
```

命令不启动API、不创建登录会话、不操作交易、微信身份和资金。先锁公共配置，再事务内重核根管理员身份；读取当前启用游戏并锁定源游戏。仅向没有有效文字的图片补两条`Labels`，保留图片、旧跳转、图层顺序和全部平台配置。空分组创建单个`game-default-<gameID>`图片。任一非空文字都视为既有内容，包含审核失败提示；只有全空白文字可视为缺失。当前拒绝分组跳过，格式或图片校验失败整笔回滚。

标准字段审核与配置保存同事务入队，版本递增使旧管理草稿返回409；旧审核结果不能覆盖新编辑。新装默认`GameSlides`走同一后置审核路径。提交后主动失效公共缓存，Redis故障仍保留持久outbox待恢复。

结果结构：

```json
{
  "applied": true,
  "already_applied": false,
  "revision": 11,
  "backup_id": "game-carousel-captions-backup-v1",
  "filled_games": ["arena", "cfm", "codm", "delta", "eggy", "honor", "identityv", "lol", "narakam", "peace", "tft", "valorant", "valorantm"],
  "filled_slides": 13,
  "preserved_slides": 0,
  "rejected_games": [],
  "cache_invalidation_pending": false
}
```

数字为原修订10、13个空游戏分组时的示例；实际以返回值为准。幂等标记为`settings/game-carousel-captions-v1`。备份为`settings/game-carousel-captions-backup-v1`，data包含`created_at`、`existed`和`public_content`，后者保存整份原公共配置，不写到命令输出。

成功后再次执行返回`already_applied=true`和`filled_slides=0`，不再加图、重复审核或覆盖管理员后来清空的内容。此版本标记仅处理首次执行时的启用游戏；以后新增游戏通过通用回退或后台单独配置。新安装初始化一次，重启不再覆盖。

本地真实PG/Redis测试覆盖13游戏回退、原图与平台配置保留、幂等、权限、回滚、审核、旧版本CAS和缓存失效；托管游戏封面补齐后的正常保存、换封面保留、跨分组/新增ID拒绝和原图撤销也有回归。320/390可读性与生产MCP结果须使用实际截图及执行报告另行记录，不能用Go测试代替。
