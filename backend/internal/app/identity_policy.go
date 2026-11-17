// Package app 独立保存身份校验方式，基础校验不依赖第三方密钥，交易时实时核对当前规则。
package app

import "context"

const identityStrict = "strict"
const identityBasic = "basic"

// identityPolicy 仅保存公开规则，不混入接口密钥或用户证件。
type identityPolicy struct {
	Mode string `json:"mode"`
}

// readIdentityPolicy 读取当前规则；历史缺项默认严格，未知值拒绝放行。
func readIdentityPolicy(ctx context.Context, q querier) (identityPolicy, error) {
	policy, err := get[identityPolicy](ctx, q, "settings", "identity-policy", false)
	if missing(err) {
		return identityPolicy{Mode: identityStrict}, nil
	}
	if err != nil {
		return identityPolicy{}, err
	}
	if policy.Mode == "" {
		policy.Mode = identityStrict
	}
	if policy.Mode != identityStrict && policy.Mode != identityBasic {
		return identityPolicy{}, fail(503, "身份校验方式配置无效，请联系管理员")
	}
	return policy, nil
}
