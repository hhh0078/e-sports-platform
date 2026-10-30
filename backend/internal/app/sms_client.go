// Package app 使用腾讯云短信官方 TC3 签名协议发送真实验证码，固定上游且不记录密钥、号码和验证码。
package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// phoneSMSConfig 为服务器独立环境配置；短信模板仅包含一个六位验证码变量。
type phoneSMSConfig struct{ SecretID, SecretKey, Region, AppID, SignName, TemplateID string }

// readPhoneSMSConfig 读取独立短信凭证，缺配置时明确停止发送与验证，不提供模拟验证码。
func readPhoneSMSConfig() (phoneSMSConfig, error) {
	c := phoneSMSConfig{SecretID: strings.TrimSpace(os.Getenv("TENCENT_SMS_SECRET_ID")), SecretKey: os.Getenv("TENCENT_SMS_SECRET_KEY"), Region: env("TENCENT_SMS_REGION", "ap-guangzhou"), AppID: strings.TrimSpace(os.Getenv("TENCENT_SMS_SDK_APP_ID")), SignName: strings.TrimSpace(os.Getenv("TENCENT_SMS_SIGN_NAME")), TemplateID: strings.TrimSpace(os.Getenv("TENCENT_SMS_TEMPLATE_ID"))}
	if c.SecretID == "" || c.SecretKey == "" || c.AppID == "" || c.SignName == "" || c.TemplateID == "" {
		return c, fail(503, "短信验证尚未配置，请联系平台管理员")
	}
	return c, nil
}

// smsHMAC 计算二进制 HMAC-SHA256，供官方派生签名与短期验证码摘要使用。
func smsHMAC(key []byte, message string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return h.Sum(nil)
}

// smsSHA256 生成无原文的十六进制摘要，用于签名和 Redis 私密索引。
func smsSHA256(message string) string {
	v := sha256.Sum256([]byte(message))
	return hex.EncodeToString(v[:])
}

// smsAuthorization 按官方 v3 规范对最终 JSON 字节和三个固定请求头签名。
func smsAuthorization(c phoneSMSConfig, at time.Time, body []byte) string {
	date := at.UTC().Format("2006-01-02")
	headers := "content-type:application/json; charset=utf-8\nhost:sms.tencentcloudapi.com\nx-tc-action:sendsms\n"
	canonical := "POST\n/\n\n" + headers + "\ncontent-type;host;x-tc-action\n" + smsSHA256(string(body))
	scope := date + "/sms/tc3_request"
	toSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(at.Unix(), 10) + "\n" + scope + "\n" + smsSHA256(canonical)
	key := smsHMAC([]byte("TC3"+c.SecretKey), date)
	key = smsHMAC(key, "sms")
	key = smsHMAC(key, "tc3_request")
	return "TC3-HMAC-SHA256 Credential=" + c.SecretID + "/" + scope + ", SignedHeaders=content-type;host;x-tc-action, Signature=" + hex.EncodeToString(smsHMAC(key, toSign))
}

// deliverPhoneSMS 只向腾讯云固定 HTTPS 地址发送请求，核验唯一号码的受理结果后才允许保存验证码。
func (a *App) deliverPhoneSMS(ctx context.Context, c phoneSMSConfig, phone, code string) error {
	body, _ := json.Marshal(map[string]any{"PhoneNumberSet": []string{"+86" + phone}, "SmsSdkAppId": c.AppID, "SignName": c.SignName, "TemplateId": c.TemplateID, "TemplateParamSet": []string{code}})
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", "https://sms.tencentcloudapi.com/", bytes.NewReader(body))
	if err != nil {
		return fail(502, "短信服务请求无效")
	}
	at := time.Now()
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", "2021-01-11")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(at.Unix(), 10))
	req.Header.Set("X-TC-Region", c.Region)
	req.Header.Set("Authorization", smsAuthorization(c, at, body))
	if a.HTTPClient == nil {
		return fail(503, "短信服务暂时不可用")
	}
	client := *a.HTTPClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return fail(502, "短信发送结果未确认，请一分钟后重试")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	var result struct {
		Response struct {
			Error *struct {
				Code string `json:"Code"`
			} `json:"Error"`
			SendStatusSet []struct {
				Code        string `json:"Code"`
				PhoneNumber string `json:"PhoneNumber"`
				SerialNo    string `json:"SerialNo"`
			} `json:"SendStatusSet"`
		} `json:"Response"`
	}
	if err != nil || len(raw) > 1024*1024 || response.StatusCode != 200 || json.Unmarshal(raw, &result) != nil {
		return fail(502, "短信服务响应异常，请稍后重试")
	}
	if result.Response.Error != nil || len(result.Response.SendStatusSet) != 1 {
		return fail(502, "短信服务暂未受理，请联系平台管理员")
	}
	status := result.Response.SendStatusSet[0]
	if status.Code != "Ok" || status.PhoneNumber != "+86"+phone || status.SerialNo == "" {
		return fail(502, "短信服务暂未受理，请稍后重试")
	}
	return nil
}

// phoneSMSDigest 绑定账号和号码生成验证码摘要，Redis 不保存验证码明文。
func phoneSMSDigest(c phoneSMSConfig, userID, phone, code string) string {
	return hex.EncodeToString(smsHMAC([]byte(c.SecretKey), fmt.Sprintf("phone-sms-v1\x00%s\x00%s\x00%s", userID, phone, code)))
}
