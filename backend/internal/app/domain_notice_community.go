// Package app 将新的入会申请和押金条件满足后的组织申请分别通知负责人及平台管理员。
package app

import (
	"context"
	"net/url"
	"strings"
)

// noticeMemberChange 仅在新申请进入pending时通知当前负责人，认证结果及普通资料修改不重复提醒。
func noticeMemberChange(ctx context.Context, q querier, before, after ClubMember, existed bool) error {
	if after.Status != "pending" || existed && before.Status == "pending" || after.ClubID == "" || after.UserID == "" || strings.HasPrefix(after.ClubID, "demo_") || strings.HasPrefix(after.UserID, "demo_") {
		return nil
	}
	managers, err := clubManagementRecipients(ctx, q, after.ClubID)
	if err != nil {
		return err
	}
	n := domainNotice("member:"+after.ClubID+":"+after.UserID+":applied:"+after.CreatedAt, "club_member", "member_applied", "收到新的入会申请", "有玩家申请加入俱乐部，请查看申请、考核及托管押金状态后处理。", "/pages/community/clubs/console?id="+url.QueryEscape(after.ClubID)+"&member_id="+url.QueryEscape(after.UserID), managers...)
	return queueDomainNotice(ctx, q, n)
}

// noticeClubChange 仅提醒已进入组织审核且满足当前创建押金要求的申请，未缴押金草稿不通知管理员。
func noticeClubChange(ctx context.Context, q querier, before, after Club, existed bool) error {
	if after.ID == "" || after.Status != "pending" || existed && before.Status == "pending" || strings.HasPrefix(after.ID, "demo_") {
		return nil
	}
	settings, err := financialSettings(ctx, q)
	if err != nil {
		return err
	}
	wallet, err := accountBalance(ctx, q, clubDepositAccount(after.ID))
	if err != nil {
		return err
	}
	if wallet.DepositRefundingCents > 0 || wallet.DepositCents < clubRequiredDeposit(after, settings) {
		return nil
	}
	n := domainNotice("club:"+after.ID+":application:"+after.CreatedAt, "admin_club", "admin_club", "有俱乐部申请待审核", "新的俱乐部申请已满足创建押金条件，请查看组织资料并处理认证。", "/pages/manage/clubs?id="+url.QueryEscape(after.ID))
	n.Admins = true
	return queueDomainNotice(ctx, q, n)
}
