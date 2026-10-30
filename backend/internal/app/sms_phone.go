// Package app 实现公众号手机号短信验证的发送限流、五分钟过期、错误次数限制和一次性消费。
package app

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var phoneSMSCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// phoneSMSRateScript 原子限制账号与号码一分钟一次、号码日五次、账号日十次、来源地址每小时二十次。
const phoneSMSRateScript = `
if redis.call('EXISTS',KEYS[1])==1 or redis.call('EXISTS',KEYS[2])==1 then return 0 end
if tonumber(redis.call('GET',KEYS[3]) or '0')>=5 or tonumber(redis.call('GET',KEYS[4]) or '0')>=10 or tonumber(redis.call('GET',KEYS[5]) or '0')>=20 then return -1 end
redis.call('SET',KEYS[1],'1','EX',60)
redis.call('SET',KEYS[2],'1','EX',60)
for i=3,5 do
 local count=redis.call('INCR',KEYS[i])
 if count==1 then if i==5 then redis.call('EXPIRE',KEYS[i],3600) else redis.call('EXPIRE',KEYS[i],86400) end end
end
return 1`

// phoneSMSVerifyScript 在 Redis 原子核对归属、摘要及次数，成功或五次错误即删除，防止并发重放。
const phoneSMSVerifyScript = `
if redis.call('EXISTS',KEYS[1])==0 then return -1 end
local tries=tonumber(redis.call('HGET',KEYS[1],'attempts') or '0')+1
if tries>5 then redis.call('DEL',KEYS[1]);return -1 end
local matched=redis.call('HGET',KEYS[1],'phone')==ARGV[1] and redis.call('HGET',KEYS[1],'digest')==ARGV[2]
if matched then redis.call('DEL',KEYS[1]);return 1 end
redis.call('HSET',KEYS[1],'attempts',tries)
if tries>=5 then redis.call('DEL',KEYS[1]) end
return 0`

// sendPhoneSMS 仅向已登录账号发送真实验证码，所有限流先于外部付费请求生效。
func (a *App) sendPhoneSMS(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Phone string `json:"phone_number"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	in.Phone = strings.TrimSpace(in.Phone)
	if !localMobile.MatchString(in.Phone) {
		return nil, fail(400, "请输入11位大陆手机号")
	}
	c, err := readPhoneSMSConfig()
	if err != nil {
		return nil, err
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	keys := []string{a.redisKey("phone-sms:cool-user:" + u.ID), a.redisKey("phone-sms:cool-phone:" + smsSHA256(in.Phone)), a.redisKey("phone-sms:day-phone:" + smsSHA256(in.Phone)), a.redisKey("phone-sms:day-user:" + u.ID), a.redisKey("phone-sms:hour-ip:" + smsSHA256(ip))}
	allowed, err := a.Redis.Eval(r.Context(), phoneSMSRateScript, keys).Int()
	if err != nil {
		return nil, fail(503, "短信验证暂时不可用")
	}
	if allowed == 0 {
		return nil, fail(429, "验证码发送过于频繁，请60秒后重试")
	}
	if allowed < 0 {
		return nil, fail(429, "验证码发送次数已达上限，请稍后重试")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, fail(503, "验证码生成失败，请稍后重试")
	}
	code := fmt.Sprintf("%06d", value.Int64())
	if err = a.deliverPhoneSMS(r.Context(), c, in.Phone, code); err != nil {
		return nil, err
	}
	key := a.redisKey("phone-sms:code:" + u.ID)
	_, err = a.Redis.TxPipelined(r.Context(), func(pipe redis.Pipeliner) error {
		pipe.HSet(r.Context(), key, "phone", in.Phone, "digest", phoneSMSDigest(c, u.ID, in.Phone, code), "attempts", 0)
		pipe.Expire(r.Context(), key, 5*time.Minute)
		return nil
	})
	if err != nil {
		return nil, fail(503, "验证码暂时无法保存，请一分钟后重新获取")
	}
	w.Header().Set("Cache-Control", "no-store")
	return map[string]int{"expires_in": 300, "retry_after": 60}, nil
}

// verifyPhoneSMS 将正确且未消费的验证码转换为真实手机号认证，开发环境同样不接受模拟验证码。
func (a *App) verifyPhoneSMS(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Phone string `json:"phone_number"`
		Code  string `json:"code"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	in.Phone = strings.TrimSpace(in.Phone)
	in.Code = strings.TrimSpace(in.Code)
	if !localMobile.MatchString(in.Phone) || !phoneSMSCodePattern.MatchString(in.Code) {
		return nil, fail(400, "请输入有效手机号和6位验证码")
	}
	c, err := readPhoneSMSConfig()
	if err != nil {
		return nil, err
	}
	matched, err := a.Redis.Eval(r.Context(), phoneSMSVerifyScript, []string{a.redisKey("phone-sms:code:" + u.ID)}, in.Phone, phoneSMSDigest(c, u.ID, in.Phone, in.Code)).Int()
	if err != nil {
		return nil, fail(503, "短信验证暂时不可用")
	}
	if matched < 0 {
		return nil, fail(422, "验证码已过期或尝试次数过多，请重新获取")
	}
	if matched == 0 {
		return nil, fail(422, "验证码不正确，请重新输入")
	}
	w.Header().Set("Cache-Control", "no-store")
	return a.storeVerifiedPhone(r.Context(), u.ID, in.Phone, "86", "sms_tencent")
}
