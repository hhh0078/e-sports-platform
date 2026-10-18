// Package app 定义可由管理员覆盖的游戏初始目录，并将旧默认图片一次性迁移为独立官方素材。
package app

import (
	"context"
	"strconv"
)

// defaultGames 按业务推荐顺序提供初始主题，该顺序不是玩家人数或实时热度排名。
func defaultGames() []Game {
	defs := []struct {
		ID, Name, Short, Subtitle, Primary, BG, Surface string
		Categories                                      []string
	}{
		{"delta", "三角洲行动", "三角洲", "安全撤离，每次都有好队友", "#00E59B", "#0B0F0E", "#101614", []string{"护航撤离", "任务协作", "技术陪练", "娱乐组队"}},
		{"honor", "王者荣耀", "王者", "峡谷相逢，一起向胜利出发", "#E9BC71", "#15100B", "#272016", []string{"双排陪玩", "五排组队", "英雄教学", "技术陪练"}},
		{"peace", "和平精英", "和平", "落地有队友，决赛圈见", "#66CBE1", "#09151C", "#132C37", []string{"娱乐组队", "技术陪练", "战术教学", "任务协作"}},
		{"arena", "暗区突围", "暗区", "准备周全，满载而归", "#E0AA62", "#14120D", "#292419", []string{"护航撤离", "地图教学", "任务协作", "娱乐组队"}},
		{"valorant", "无畏契约", "无畏", "默契配合，打出漂亮回合", "#FF6672", "#1A0E14", "#301923", []string{"双排陪玩", "战术教学", "技术陪练", "娱乐组队"}},
		{"cfm", "穿越火线手游", "穿越火线", "热血开局，兄弟并肩", "#F28C51", "#1A110D", "#302017", []string{"技术陪练", "娱乐组队", "地图教学", "任务协作"}},
		{"tft", "金铲铲之战", "金铲铲", "棋逢队友，弈起上场", "#E6C76D", "#17160D", "#2B2919", []string{"双人作战", "阵容教学", "新手指导", "娱乐组队"}},
		{"lol", "英雄联盟", "英雄联盟", "召唤师峡谷，配合成就高光", "#C9AA71", "#0A141B", "#17252F", []string{"双排陪玩", "五排组队", "对线教学", "复盘指导"}},
		{"valorantm", "无畏契约：源能行动", "源能行动", "掌上集结，一起打好每个回合", "#FF7880", "#191213", "#2C1E20", []string{"娱乐组队", "技术陪练", "地图教学", "战术教学"}},
		{"codm", "使命召唤手游", "使命召唤", "战术集结，随时并肩作战", "#D5D96A", "#14170F", "#262D1C", []string{"娱乐组队", "技术陪练", "地图教学", "任务协作"}},
		{"narakam", "永劫无间手游", "永劫手游", "风起聚窟，一起自在出招", "#D68D72", "#1B1310", "#30201B", []string{"双排陪玩", "三排组队", "连招教学", "身法陪练"}},
		{"identityv", "第五人格", "第五人格", "庄园相约，默契逃生", "#C4B598", "#191815", "#2C2924", []string{"娱乐组队", "角色教学", "地图教学", "复盘指导"}},
		{"eggy", "蛋仔派对", "蛋仔派对", "快乐碰头，每一局都热闹", "#E5AB65", "#211A12", "#36291D", []string{"娱乐组队", "地图探索", "闯关陪练", "新手指导"}},
	}
	// 研究仅支持热度分层与业务优先级，不将不同年份、口径的 DAU 拼成排行榜。
	recommended := []string{"honor", "peace", "delta", "tft", "valorant", "lol", "valorantm", "cfm", "arena", "codm", "identityv", "narakam", "eggy"}
	games := make([]Game, 0, len(defs))
	for _, d := range defs {
		sortOrder := 0
		for i, id := range recommended {
			if d.ID == id {
				sortOrder = i
				break
			}
		}
		prefix := "/static/covers/official/" + d.ID
		covers := []string{prefix + "-1.jpg", prefix + "-2.jpg"}
		if d.ID == "delta" || d.ID == "honor" {
			covers = append(covers, prefix+"-3.jpg", prefix+"-4.jpg")
		}
		games = append(games, Game{ID: d.ID, Name: d.Name, ShortName: d.Short, Subtitle: d.Subtitle, PrimaryColor: d.Primary, BgColor: d.BG, SurfaceColor: d.Surface, TextColor: "#F3F5F3", IconURL: prefix + "-icon.jpg", CoverURL: prefix + "-1.jpg", Covers: covers, Categories: d.Categories, Enabled: true, SortOrder: sortOrder})
	}
	return games
}

// migrateDefaultPresentation 仅升级无管理员保存审计且精确匹配旧种子的展示字段，无法区分的记录保持原样。
func migrateDefaultPresentation(ctx context.Context, q querier, defaults []Game) error {
	const marker = "catalog-presentation-v1"
	if _, err := get[map[string]any](ctx, q, "settings", marker, false); err == nil {
		return nil
	} else if !missing(err) {
		return err
	}
	legacyOrder := map[string]int{"delta": 0, "honor": 1, "peace": 2, "arena": 3, "valorant": 4, "cfm": 5, "tft": 6, "lol": 7, "valorantm": 8, "codm": 9, "narakam": 10, "identityv": 11, "eggy": 12}
	for _, d := range defaults {
		g, err := get[Game](ctx, q, "games", d.ID, true)
		if err != nil {
			return err
		}
		var customized bool
		if err = q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM audit WHERE data->>'action'='game.save' AND data->>'target_id'=$1)", g.ID).Scan(&customized); err != nil {
			return err
		}
		if customized {
			continue
		}
		changed := false
		if original, ok := legacyOrder[g.ID]; ok && g.SortOrder == original && g.SortOrder != d.SortOrder {
			g.SortOrder = d.SortOrder
			changed = true
		}
		if g.ID == "delta" && g.PrimaryColor == "#00E59B" && g.BgColor == "#080F0D" && g.SurfaceColor == "#14221D" {
			g.BgColor, g.SurfaceColor = d.BgColor, d.SurfaceColor
			changed = true
		}
		if changed {
			if err = put(ctx, q, "games", g.ID, g); err != nil {
				return err
			}
		}
	}
	return seedInsert(ctx, q, "settings", marker, map[string]string{"description": "仅升级无管理员修改审计且匹配旧种子的排序和三角洲主题，保留所有自定义", "applied_at": now()})
}

// migrateOfficialCovers 只替换已知旧默认路径，保留自定义主题、标签、交易快照及所有非图片字段。
func migrateOfficialCovers(ctx context.Context, q querier, defaults []Game) error {
	const marker = "catalog-official-v1"
	if _, err := get[map[string]any](ctx, q, "settings", marker, false); err == nil {
		return nil
	} else if !missing(err) {
		return err
	}
	for _, d := range defaults {
		g, err := get[Game](ctx, q, "games", d.ID, true)
		if err != nil {
			return err
		}
		changed := false
		// 旧版本每款仅有两张默认素材，新扩展的第三、四张不能推断为历史默认。
		for i, official := range d.Covers[:2] {
			legacy := "/static/covers/" + d.ID + "-" + strconv.Itoa(i+1) + ".jpg"
			if g.CoverURL == legacy {
				g.CoverURL = official
				changed = true
			}
			for j := range g.Covers {
				if g.Covers[j] == legacy {
					g.Covers[j] = official
					changed = true
				}
			}
			if _, err = q.Exec(ctx, "UPDATE services SET data=jsonb_set(data,'{cover_url}',to_jsonb($3::text)) WHERE data->>'game_id'=$1 AND data->>'cover_url'=$2", d.ID, legacy, official); err != nil {
				return err
			}
		}
		if g.IconURL == "" {
			g.IconURL = d.IconURL
			changed = true
		}
		if changed {
			if err = put(ctx, q, "games", g.ID, g); err != nil {
				return err
			}
		}
	}
	return seedInsert(ctx, q, "settings", marker, map[string]string{"description": "旧默认封面已迁移为官方素材，后续重启保留管理员配置", "applied_at": now()})
}

type demoService struct {
	Title, Label string
	Category     int
	PriceCents   int64
}

// demoServices 提供真实零成交的本地示例，不填充虚构销量或评价。
func demoServices(gameID string) []demoService {
	if gameID == "delta" {
		return []demoService{
			{"安全撤离 · 全程护航", "安全撤离", 0, 1900},
			{"任务协作 · 轻松探索", "任务协作", 1, 2900},
			{"新手起步 · 基础陪练", "新手陪练", 2, 1600},
			{"快乐开黑 · 轻松组队", "快乐组队", 3, 1500},
			{"路线复盘 · 撤离规划", "路线规划", 0, 2500},
			{"干员配合 · 战术训练", "战术陪练", 2, 2200},
			{"地图探索 · 任务搭子", "地图探索", 1, 1800},
			{"周末开黑 · 默契队友", "周末开黑", 3, 2000},
		}
	}
	if gameID == "honor" {
		return []demoService{{"峡谷双排 · 默契配合", "双排陪玩", 0, 1900}, {"五排开黑 · 快乐集结", "五排组队", 1, 2900}, {"英雄理解 · 对线教学", "英雄教学", 2, 2200}, {"技能训练 · 耐心陪练", "技术陪练", 3, 1800}}
	}
	return []demoService{{"", "默契组队", 0, 1900}, {"", "耐心陪练", 1, 2900}}
}
