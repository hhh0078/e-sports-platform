// Package app 显式生成带命名空间和标记的虚构展示数据，不创建登录凭证或真实支付记录。
package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"
)

var showcaseTables = []string{"users", "clubs", "members", "services", "orders", "messages", "reviews", "evidence"}

// showcaseRecord 表示仅由演示命令写入的领域记录。
type showcaseRecord struct {
	table string
	id    string
	value any
}

// showcaseID 构造可辨识且可重复导入的稳定演示编号。
func showcaseID(kind string, index int) string { return fmt.Sprintf("demo_%s_%03d", kind, index+1) }

// showcaseAvatar 生成自然色系几何头像，无需网络、个人照片或新增素材依赖。
func showcaseAvatar(index int) ([]byte, error) {
	palette := []color.RGBA{{57, 146, 116, 255}, {174, 132, 67, 255}, {78, 137, 157, 255}, {160, 91, 84, 255}, {139, 146, 82, 255}, {139, 110, 78, 255}}
	base := palette[index%len(palette)]
	canvas := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.RGBA{21, 30, 27, 255}}, image.Point{}, draw.Src)
	bits := sha256.Sum256([]byte(showcaseID("avatar", index)))
	for y := 0; y < 7; y++ {
		for x := 0; x < 4; x++ {
			if bits[(y*4+x)%32]%3 == 0 {
				continue
			}
			for _, column := range []int{x, 6 - x} {
				box := image.Rect(23+column*30, 23+y*30, 48+column*30, 48+y*30)
				draw.Draw(canvas, box, &image.Uniform{base}, image.Point{}, draw.Src)
			}
		}
	}
	var result bytes.Buffer
	err := png.Encode(&result, canvas)
	return result.Bytes(), err
}

// showcaseRecords 根据现有十三款游戏配置建立完整虚构关系，不改动管理员配置。
func showcaseRecords(ctx context.Context, q querier) ([]showcaseRecord, error) {
	games := make([]Game, 0, 13)
	for _, defaultGame := range defaultGames() {
		game, err := get[Game](ctx, q, "games", defaultGame.ID, false)
		if err != nil {
			return nil, fmt.Errorf("游戏 %s 未初始化，请先启动 API：%w", defaultGame.ID, err)
		}
		if len(game.Covers) == 0 || len(game.Categories) == 0 {
			return nil, fmt.Errorf("游戏 %s 缺少封面或服务分类，无法生成演示", game.Name)
		}
		games = append(games, game)
	}
	started := time.Now().UTC().Add(-48 * time.Hour)
	stamp := func(minutes int) string {
		return started.Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339Nano)
	}
	users := make([]User, 60)
	clubs := make([]Club, 6)
	services := make([]Service, 0, len(games)*10)
	records := []showcaseRecord{}
	first := []string{"青柠", "星野", "木槿", "白露", "长风", "溪云", "初晴", "山岚", "南栀", "晚舟"}
	second := []string{"同学", "队友", "小鹿", "阿鱼", "北北", "队长"}
	for i := range users {
		game := games[i%len(games)]
		u := User{ID: showcaseID("user", i), Nickname: first[i%len(first)] + second[i/len(first)] + "·演示", Avatar: "/api/media/" + showcaseID("avatar", i), ProfileCover: game.CoverURL, Bio: "演示账号，非真实用户。喜欢" + game.Name + "，偏好耐心沟通、团队配合与轻松组队。", Role: "user", Status: "active", ProviderEnabled: i < 42, CreatedAt: stamp(i)}
		users[i] = u
		records = append(records, showcaseRecord{"users", u.ID, u})
		raw, err := showcaseAvatar(i)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		avatar := PublicImage{Evidence: Evidence{ID: showcaseID("avatar", i), OwnerID: u.ID, SHA256: hex.EncodeToString(sum[:]), MIME: "image/png", Data: raw, CreatedAt: stamp(i), Public: true}, Purpose: "avatar", Width: 256, Height: 256}
		records = append(records, showcaseRecord{"evidence", avatar.ID, avatar})
	}
	clubNames := []string{"山野集结", "逐光同行", "松风小队", "星河搭子", "原野同盟", "海岸玩家"}
	gameIDs := make([]string, len(games))
	for i, g := range games {
		gameIDs[i] = g.ID
	}
	for i := range clubs {
		assessment := "完成一局配合考核，确认沟通态度和服务范围。演示规则，不涉及真实招募。"
		club := Club{ID: showcaseID("club", i), Name: clubNames[i] + "·演示", Description: "这是源码演示用的虚构俱乐部。展示多游戏服务、成员挂靠、考核说明与订单协作，不代表真实机构或平台背书。", OwnerID: users[i].ID, OwnerName: users[i].Nickname, GameIDs: gameIDs, Status: "active", Verified: i%2 == 0, CoverURL: games[i].CoverURL, DetailBlocks: []CommunityBlock{{Type: "text", Text: "欢迎查看演示俱乐部。这里展示图文介绍、游戏范围与认证成员，所有人物、订单及评价均为虚构示例。"}, {Type: "image", ImageURL: games[i].CoverURL}, {Type: "text", Text: "服务约定：先沟通需求，再确认服务范围。资料中的押金仅为规则展示，不代表平台已收款或托管。"}}, DepositCents: int64(i%3) * 20000, Assessment: assessment, CreatedAt: stamp(i)}
		clubs[i] = club
		records = append(records, showcaseRecord{"clubs", club.ID, club})
	}
	for i := 6; i < 42; i++ {
		club := clubs[(i-6)%len(clubs)]
		member := ClubMember{UserID: users[i].ID, Nickname: users[i].Nickname, ClubID: club.ID, Status: "active", CreatedAt: stamp(60 + i), ApplicationNote: "演示挂靠申请，不涉及真实身份或押金。", RequirementsAccepted: true, AppliedDepositCents: club.DepositCents, AppliedAssessment: club.Assessment, AssessmentPassed: true, ReviewNote: "演示考核已通过，非真实认证。"}
		records = append(records, showcaseRecord{"members", club.ID + ":" + member.UserID, member})
		if i < 12 {
			other := clubs[(i-5)%len(clubs)]
			member.ClubID, member.Status, member.AppliedDepositCents, member.AppliedAssessment, member.AssessmentPassed, member.ReviewNote = other.ID, "pending", other.DepositCents, other.Assessment, false, ""
			records = append(records, showcaseRecord{"members", other.ID + ":" + member.UserID, member})
		}
	}
	features := []string{"耐心陪练", "轻松组队", "新手指导", "战术沟通", "周末开黑", "默契双排", "任务协作", "细节复盘", "语音交流", "晚间队友"}
	for gi, game := range games {
		for n := 0; n < 10; n++ {
			index := len(services)
			providerIndex := 6 + (gi*10+n)%36
			provider, club := users[providerIndex], clubs[(providerIndex-6)%len(clubs)]
			service := Service{ID: showcaseID("service", index), GameID: game.ID, ProviderID: provider.ID, ProviderName: provider.Nickname, ProviderAvatar: provider.Avatar, ClubID: club.ID, ClubName: club.Name, ClubVerified: club.Verified, Title: game.ShortName + "·" + features[n] + "·演示", Description: "此为源码展示服务，不可付款下单。" + features[n] + "，提前约好游戏模式、在线时段和本次服务范围；注重沟通与配合，不承诺固定胜率或收益。所有图片文字均为可编辑展示示例。", Category: game.Categories[n%len(game.Categories)], PriceCents: int64(900 + gi%3*500 + n*300), Unit: "小时", CoverURL: game.Covers[n%len(game.Covers)], Labels: []CoverLabel{{ID: "demo_label_a", Text: features[n], Color: game.PrimaryColor, Background: "#101614", Position: "bottom-left", FontSize: 16}, {ID: "demo_label_b", Text: "演示展示", Color: "#FFFFFF", Background: "#353C38", Position: "bottom-left", FontSize: 12}}, Tags: []string{"演示数据", "耐心沟通"}, Status: "active", CreatedAt: stamp(120 + index)}
			if err := validateService(&service, game); err != nil {
				return nil, err
			}
			services = append(services, service)
		}
	}
	states := []string{"pending_payment", "open", "accepted", "in_progress", "delivered", "completed", "completed", "cancelled", "refunded", "in_progress", "in_progress", "in_progress", "in_progress"}
	ratingSums := make([]int, len(services))
	for i := 0; i < len(games)*4; i++ {
		gameIndex, slot := i/4, i%4
		serviceIndex := gameIndex*10 + slot*2
		service := services[serviceIndex]
		provider := users[6+(gameIndex*10+slot*2)%36]
		buyer := users[42+i%17]
		club := clubs[(6+(gameIndex*10+slot*2)%36-6)%6]
		state := states[i%len(states)]
		order := Order{ID: showcaseID("order", i), OrderNo: fmt.Sprintf("DEMO-JJ-%04d", i+1), GameID: service.GameID, GameName: games[gameIndex].Name, ServiceID: service.ID, BuyerID: buyer.ID, BuyerName: buyer.Nickname, ProviderID: provider.ID, ProviderName: provider.Nickname, ClubID: club.ID, Title: service.Title, Requirements: "演示约定：今晚一起进行两小时练习，开始前确认游戏区服和服务范围。没有真实交易。", Quantity: 2, AmountCents: service.PriceCents * 2, Status: state, PaymentStatus: "paid", RefundStatus: "none", CreatedAt: stamp(300 + i*10), UpdatedAt: stamp(307 + i*10), ServiceDescription: service.Description, Unit: service.Unit, UnitPriceCents: service.PriceCents}
		if state == "pending_payment" || state == "cancelled" {
			order.PaymentStatus = "unpaid"
		}
		if state == "delivered" || state == "completed" {
			order.DeliveryNote = "演示交付：完成约定时段，已给出练习建议和本局复盘。"
		}
		if state == "completed" {
			order.CompletedAt = stamp(307 + i*10)
			rating := 4 + i%2
			review := Review{ID: showcaseID("review", i), OrderID: order.ID, Rating: rating, Content: "演示评价：沟通及时，服务范围说明清楚，下次还会一起练习。该评价为虚构示例。", BuyerName: buyer.Nickname, CreatedAt: stamp(308 + i*10)}
			records = append(records, showcaseRecord{"reviews", review.ID, review})
			services[serviceIndex].CompletedCount++
			ratingSums[serviceIndex] += rating
		}
		if state == "refunded" {
			order.PaymentStatus, order.RefundStatus, order.RefundReason = "refunded", "approved", "演示退款：双方协商取消本次安排。"
		}
		if i%len(states) >= 9 {
			order.RefundStatus = []string{"requested", "rejected", "escalated", "processing"}[i%len(states)-9]
			order.RefundReason = "演示售后：对本次服务完成范围存在分歧，请按聊天记录核对。"
			if order.RefundStatus != "requested" {
				order.RefundReply = "演示答复：我已按约定完成首阶段，希望根据已交付内容协商。"
			}
			if order.RefundStatus == "escalated" {
				order.DisputeReason = "演示介入：买卖双方协商未达成一致，请平台审查沟通与交付记录。"
			}
		}
		records = append(records, showcaseRecord{"orders", order.ID, order})
		roles := []struct{ id, name, role, content, kind string }{
			{"demo_system", "集结平台·演示", "system", "本订单为虚构演示，无真实资金流；聊天内容用于展示证据链。", "system"},
			{buyer.ID, buyer.Nickname, "buyer", "演示沟通：我希望先确认模式和时段，服务内容以订单说明为准。", "text"},
			{provider.ID, provider.Nickname, "provider", "演示沟通：收到，我会按约定协作，交付后把完成情况写在这里。", "text"},
			{club.OwnerID, club.OwnerName, "club_admin", "演示沟通：俱乐部已看到约定，如有分歧请保留完整记录。", "text"},
			{users[59].ID, "平台协助员·演示", "admin", "演示提示：平台根据订单约定、沟通与交付材料处理售后，避免私下转账。", "text"},
		}
		for j, role := range roles {
			message := Message{ID: showcaseID("message", i*5+j), OrderID: order.ID, SenderID: role.id, SenderName: role.name, SenderRole: role.role, Content: role.content, Kind: role.kind, CreatedAt: stamp(301 + i*10 + j)}
			records = append(records, showcaseRecord{"messages", message.ID, message})
		}
	}
	for i, service := range services {
		if service.CompletedCount > 0 {
			service.Rating = float64(ratingSums[i]) / float64(service.CompletedCount)
		}
		records = append(records, showcaseRecord{"services", service.ID, service})
	}
	return records, nil
}

// insertShowcaseRecord 只插入空缺演示记录，遇到未标记的同名记录拒绝整笔导入。
func insertShowcaseRecord(ctx context.Context, q querier, record showcaseRecord) (int, error) {
	if !strings.HasPrefix(record.id, "demo_") {
		return 0, fmt.Errorf("拒绝写入非演示编号：%s", record.id)
	}
	old, err := get[map[string]any](ctx, q, record.table, record.id, false)
	if err == nil {
		if old["is_demo"] != true || old["demo_version"] != float64(1) {
			return 0, fmt.Errorf("编号 %s 已被非演示记录占用，导入未执行", record.id)
		}
		return 0, nil
	}
	if !missing(err) {
		return 0, err
	}
	raw, err := json.Marshal(record.value)
	if err != nil {
		return 0, err
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	value["is_demo"], value["demo_version"] = true, 1
	raw, err = json.Marshal(value)
	if err != nil {
		return 0, err
	}
	result, err := q.Exec(ctx, "INSERT INTO "+validTable(record.table)+"(id,data) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", record.id, raw)
	if err != nil {
		return 0, err
	}
	if result.RowsAffected() == 0 {
		current, e := get[map[string]any](ctx, q, record.table, record.id, false)
		if e != nil {
			return 0, e
		}
		if current["is_demo"] != true || current["demo_version"] != float64(1) {
			return 0, fmt.Errorf("编号 %s 被并发写入的非演示记录占用，导入已回滚", record.id)
		}
	}
	return int(result.RowsAffected()), nil
}

// SeedShowcase 在一个事务内幂等导入演示数据，绝不创建会话、微信身份或商户流水。
func (a *App) SeedShowcase(ctx context.Context) (map[string]int, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173040)"); err != nil {
		return nil, err
	}
	records, err := showcaseRecords(ctx, tx)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, record := range records {
		inserted, e := insertShowcaseRecord(ctx, tx, record)
		if e != nil {
			return nil, e
		}
		counts[record.table] += inserted
	}
	return counts, tx.Commit(ctx)
}

// ShowcaseStatus 只统计演示命名空间内且同时带本版本标记的记录。
func (a *App) ShowcaseStatus(ctx context.Context) (map[string]int, error) {
	counts := map[string]int{}
	for _, table := range showcaseTables {
		var count int
		if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM "+validTable(table)+" WHERE left(id,5)='demo_' AND data->>'is_demo'='true' AND data->>'demo_version'='1'").Scan(&count); err != nil {
			return nil, err
		}
		counts[table] = count
	}
	return counts, nil
}
