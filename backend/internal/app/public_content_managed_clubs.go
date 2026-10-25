// Package app 从完整组织权限集合查询本人可管理俱乐部，供管理入口使用，不从公开首屏或成员列表推算。
package app

import (
	"net/http"
	"strings"
)

// managedClubs 先按负责人及有效副管理员筛选搜索，再分页，访客及普通挂靠成员不进入管理集合。
func (a *App) managedClubs(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if r.URL.Query().Get("mine") == "1" {
		return nil, fail(400, "管理范围与成员范围不能同时查询")
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if !length(keyword, 0, 100) {
		return nil, fail(400, "搜索关键词最多100字")
	}
	limit, offset := pageLimit(r)
	where := "WHERE " + managedClubWhere("clubs", "$1") + " AND ($2='' OR strpos(lower(concat_ws(' ',data->>'name',data->>'description')),lower($2))>0) ORDER BY data->>'created_at' DESC,id DESC LIMIT $3 OFFSET $4"
	items, err := list[Club](r.Context(), a.DB, "clubs", where, u.ID, keyword, limit, offset)
	if err != nil {
		return nil, err
	}
	for index := range items {
		role, err := clubManagementRole(r.Context(), a.DB, u, items[index])
		if err != nil {
			return nil, err
		}
		items[index] = a.decorateClub(r.Context(), items[index])
		items[index].MembershipStatus = role
		items[index].ManagementRole = role
		items[index].CanManageAdmins = canManageClubAdmins(u, items[index])
	}
	return items, nil
}
