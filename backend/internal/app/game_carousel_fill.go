// Package app 显式补齐无文案游戏轮播，同事务备份、权限重核、标准审核与审计；重复执行保留管理员后续修改。
package app

import (
	"context"
	"strings"
	"time"
)

const gameCaptionMarker = "game-carousel-captions-v1"
const gameCaptionBackup = "game-carousel-captions-backup-v1"

// GameCaptionFillResult 只汇总本次补齐数量与版本，不输出既有运营图文或任何私密设置。
type GameCaptionFillResult struct {
	Applied                  bool     `json:"applied"`
	AlreadyApplied           bool     `json:"already_applied"`
	Revision                 int64    `json:"revision"`
	BackupID                 string   `json:"backup_id"`
	FilledGames              []string `json:"filled_games"`
	FilledSlides             int      `json:"filled_slides"`
	PreservedSlides          int      `json:"preserved_slides"`
	RejectedGames            []string `json:"rejected_games"`
	CacheInvalidationPending bool     `json:"cache_invalidation_pending"`
}

// gameCaptionsRejected 检查当前游戏分组的有效拒绝记录，旧的已取代任务不会永久阻止新内容。
func gameCaptionsRejected(ctx context.Context, q querier, gameID string) (bool, error) {
	var rejected bool
	err := q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM moderation_targets WHERE data->>'target_type'='settings' AND data->>'target_id'=$1 AND data->>'status'='rejected' AND strpos(data->>'field_code',$2)=1)", publicContentID, "game_slide."+gameID+".").Scan(&rejected)
	return rejected, err
}

// fallbackGameSlide 仅在没有显式游戏轮播时生成默认图文；审核拒绝后不通过回退重新显示同类文案。
func fallbackGameSlide(ctx context.Context, q querier, game Game) (ContentSlide, error) {
	slide := ContentSlide{ID: "game-default-" + game.ID, Kind: "image", ImageURL: game.CoverURL, Labels: []CoverLabel{}}
	rejected, err := gameCaptionsRejected(ctx, q, game.ID)
	if err != nil {
		return slide, err
	}
	if !rejected && game.CoverURL != rejectedImageURL {
		slide.Labels = GameCarouselLabels(game)
	}
	return slide, nil
}

// captionSlideHasText 把任意非空文字视为自定义内容；审核失败提示也必须保留，不能拿默认字覆盖。
func captionSlideHasText(slide ContentSlide) bool {
	for _, label := range slide.Labels {
		if strings.TrimSpace(label.Text) != "" {
			return true
		}
	}
	return false
}

// validateCaptionSlide 复用标准图层校验；原跳转始终保留，由公开读取实时隐藏失效行动。
func (a *App) validateCaptionSlide(ctx context.Context, q querier, game Game, slide, previous ContentSlide, actor User) (ContentSlide, error) {
	validated, err := a.validateContentSlides(ctx, q, []ContentSlide{slide}, []ContentSlide{previous}, &game, actor)
	if err != nil {
		return slide, err
	}
	validated[0].Target = previous.Target
	return validated[0], nil
}

// gameCaptionDraft 只补没有有效文案的游戏图和空分组，按ID稳定读取并锁住源游戏，返回独立配置副本。
func (a *App) gameCaptionDraft(ctx context.Context, q querier, previous PublicContentSettings, actor User) (PublicContentSettings, GameCaptionFillResult, error) {
	v := previous
	v.GameSlides = make(map[string][]ContentSlide, len(previous.GameSlides))
	for id, slides := range previous.GameSlides {
		v.GameSlides[id] = append([]ContentSlide{}, slides...)
	}
	result := GameCaptionFillResult{BackupID: gameCaptionBackup, FilledGames: []string{}, RejectedGames: []string{}}
	games, err := list[Game](ctx, q, "games", "WHERE data->>'enabled'='true' ORDER BY id FOR SHARE")
	if err != nil {
		return v, result, err
	}
	for _, game := range games {
		if !gameIDPattern.MatchString(game.ID) {
			return v, result, fail(400, "启用游戏编号不符合配置要求")
		}
		rejected, err := gameCaptionsRejected(ctx, q, game.ID)
		if err != nil {
			return v, result, err
		}
		if rejected || game.CoverURL == rejectedImageURL {
			result.RejectedGames = append(result.RejectedGames, game.ID)
			continue
		}
		slides := v.GameSlides[game.ID]
		if len(slides) == 0 {
			slides = []ContentSlide{{ID: "game-default-" + game.ID, Kind: "image", ImageURL: game.CoverURL, Labels: []CoverLabel{}}}
		}
		if len(slides) > 12 {
			return v, result, fail(400, "每组游戏轮播最多12项")
		}
		seen := map[string]bool{}
		for _, slide := range slides {
			if !length(slide.ID, 1, 80) || seen[slide.ID] {
				return v, result, fail(400, "游戏轮播标识重复或无效")
			}
			seen[slide.ID] = true
		}
		filled := false
		for index, slide := range slides {
			if captionSlideHasText(slide) || slide.ImageURL == rejectedImageURL {
				result.PreservedSlides++
				continue
			}
			candidate := slide
			candidate.Labels = GameCarouselLabels(game)
			candidate, err = a.validateCaptionSlide(ctx, q, game, candidate, slide, actor)
			if err != nil {
				return v, result, err
			}
			slides[index] = candidate
			result.FilledSlides++
			filled = true
		}
		if filled {
			result.FilledGames = append(result.FilledGames, game.ID)
			v.GameSlides[game.ID] = slides
		}
	}
	if len(v.GameSlides) > 100 {
		return v, result, fail(400, "游戏轮播分组过多")
	}
	return v, result, nil
}

// recordGameCaptionFill 在原事务保存不可覆盖配置备份、幂等标记及审计，不单独提交或触发交易通知。
func recordGameCaptionFill(ctx context.Context, q querier, actorID string, before PublicContentSettings, existed bool, result GameCaptionFillResult) error {
	if _, err := get[map[string]any](ctx, q, "settings", gameCaptionBackup, false); err == nil {
		return fail(409, "游戏文案备份已存在，拒绝覆盖历史记录")
	} else if !missing(err) {
		return err
	}
	if err := putRaw(ctx, q, "settings", gameCaptionBackup, map[string]any{"created_at": now(), "existed": existed, "public_content": before}); err != nil {
		return err
	}
	if err := putRaw(ctx, q, "settings", gameCaptionMarker, result); err != nil {
		return err
	}
	return audit(ctx, q, actorID, "game_captions.fill", gameCaptionMarker, "只补无文案游戏轮播；保留现有图片、跳转、顺序和全部平台配置")
}

// FillGameCaptions 显式幂等补齐当前启用游戏，按公共配置锁再根管理员行锁重核权限，提交后立即失效缓存。
func (a *App) FillGameCaptions(ctx context.Context, actorID string) (GameCaptionFillResult, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return GameCaptionFillResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		return GameCaptionFillResult{}, err
	}
	actor, err := lockManagementWriteActor(ctx, tx, actorID)
	if err != nil {
		return GameCaptionFillResult{}, err
	}
	if !isPlatformRoot(actor) {
		return GameCaptionFillResult{}, fail(403, "仅有效平台根管理员可补齐游戏轮播文案")
	}
	previous, err := get[PublicContentSettings](ctx, tx, "settings", publicContentID, true)
	existed := !missing(err)
	if missing(err) {
		previous = defaultPublicContent()
	} else if err != nil {
		return GameCaptionFillResult{}, err
	}
	if _, markerErr := get[GameCaptionFillResult](ctx, tx, "settings", gameCaptionMarker, true); markerErr == nil {
		return GameCaptionFillResult{AlreadyApplied: true, Revision: previous.Revision, BackupID: gameCaptionBackup, FilledGames: []string{}, RejectedGames: []string{}}, nil
	} else if !missing(markerErr) {
		return GameCaptionFillResult{}, markerErr
	}
	v, result, err := a.gameCaptionDraft(ctx, tx, previous, actor)
	if err != nil {
		return result, err
	}
	result.Applied = true
	if result.FilledSlides > 0 {
		v.Revision++
		v.UpdatedAt = now()
		reviewCtx := context.WithValue(ctx, reviewContextKey{}, &reviewRequest{App: a, ActorID: actor.ID})
		if err = put(reviewCtx, tx, "settings", publicContentID, v); err != nil {
			return result, err
		}
	}
	result.Revision = v.Revision
	if err = recordGameCaptionFill(ctx, tx, actor.ID, previous, existed, result); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	if a.Redis != nil && result.FilledSlides > 0 {
		flush, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		result.CacheInvalidationPending = a.drainPublicCache(flush, true) != nil
		cancel()
	}
	return result, nil
}
