// Package app 为新用户分配内置非人动物头像，并精确迁移空值及旧演示几何头像，保留自定义图片。
package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"jijie/internal/assets"
)

// randomDefaultAvatar 使用系统安全随机源选择一次新用户头像，调用者将结果持久化后重复登录不再分配。
func randomDefaultAvatar() (string, error) {
	paths := assets.DefaultAvatarPaths()
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(paths))))
	if err != nil {
		return "", err
	}
	return paths[index.Int64()], nil
}

// stableDefaultAvatar 为历史空头像与演示记录提供稳定的动物图，重启和幂等导入不会改变展示。
func stableDefaultAvatar(namespace, id string) string {
	paths := assets.DefaultAvatarPaths()
	sum := sha256.Sum256([]byte(namespace + ":" + id))
	index := (int(sum[0])<<8 | int(sum[1])) % len(paths)
	return paths[index]
}

// defaultClubAvatar 将俱乐部的随机业务编号稳定映射为动物徽章，避免用游戏封面冒充方形头像。
func defaultClubAvatar(clubID string) string { return stableDefaultAvatar("club", clubID) }

// trustedDefaultAvatar 只将明确用作头像的固定清单视为平台素材，封面和用户URL不享有审核豁免。
func trustedDefaultAvatar(raw, purpose string) bool {
	return purpose == "avatar" && assets.IsDefaultAvatar(raw)
}

// legacyGeometricAvatar 只识别原演示账号本人持有、来源字段及实际字节均匹配旧生成器的默认图。
func legacyGeometricAvatar(ctx context.Context, q querier, user User) (bool, error) {
	suffix, ok := strings.CutPrefix(user.Avatar, "/api/media/demo_avatar_")
	if !ok {
		return false, nil
	}
	number, err := strconv.Atoi(suffix)
	if err != nil || number < 1 || number > 60 || suffix != strings.TrimPrefix(showcaseID("avatar", number-1), "demo_avatar_") || user.ID != showcaseID("user", number-1) {
		return false, nil
	}
	metadata, err := get[map[string]any](ctx, q, "users", user.ID, false)
	if err != nil {
		return false, err
	}
	if metadata["is_demo"] != true || metadata["demo_version"] != float64(1) {
		return false, nil
	}
	type legacyImage struct {
		PublicImage
		IsDemo      bool `json:"is_demo"`
		DemoVersion int  `json:"demo_version"`
	}
	v, err := get[legacyImage](ctx, q, "evidence", showcaseID("avatar", number-1), false)
	if missing(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !v.IsDemo || v.DemoVersion != 1 || v.ID != showcaseID("avatar", number-1) || v.OwnerID != user.ID || v.Purpose != "avatar" || !v.Public || v.OrderID != "" || v.ClubID != "" || v.MIME != "image/png" {
		return false, nil
	}
	old, err := legacyShowcaseAvatar(number - 1)
	if err != nil {
		return false, err
	}
	expected, actual := sha256.Sum256(old), sha256.Sum256(v.Data)
	return expected == actual && v.SHA256 == hex.EncodeToString(expected[:]), nil
}

// migrateDefaultAvatars 在初始化事务内一次性升级头像，保留未知资料和自定义图片，后续启动无需全表扫描。
func migrateDefaultAvatars(ctx context.Context, q querier) error {
	const marker = "default-avatars-fluent-v1"
	if _, err := get[map[string]any](ctx, q, "settings", marker, false); err == nil {
		return nil
	} else if !missing(err) {
		return err
	}
	users, err := list[User](ctx, q, "users", "WHERE COALESCE(data->>'avatar','')='' OR data->>'avatar' LIKE '/api/media/demo_avatar_%' FOR UPDATE")
	if err != nil {
		return err
	}
	for _, user := range users {
		migrate := user.Avatar == ""
		if !migrate {
			migrate, err = legacyGeometricAvatar(ctx, q, user)
			if err != nil {
				return err
			}
		}
		if migrate {
			if _, err = q.Exec(ctx, "UPDATE users SET data=jsonb_set(data,'{avatar}',to_jsonb($2::text),true) WHERE id=$1 AND COALESCE(data->>'avatar','')=$3", user.ID, stableDefaultAvatar("user", user.ID), user.Avatar); err != nil {
				return err
			}
		}
	}
	clubs, err := list[Club](ctx, q, "clubs", "WHERE COALESCE(data->>'avatar_url','')='' FOR UPDATE")
	if err != nil {
		return err
	}
	for _, club := range clubs {
		if _, err = q.Exec(ctx, "UPDATE clubs SET data=jsonb_set(data,'{avatar_url}',to_jsonb($2::text),true) WHERE id=$1 AND COALESCE(data->>'avatar_url','')=''", club.ID, defaultClubAvatar(club.ID)); err != nil {
			return err
		}
	}
	return seedInsert(ctx, q, "settings", marker, map[string]string{"description": "仅补空头像和校验字节一致的旧演示几何图，保留自定义与原有字段", "applied_at": now()})
}
