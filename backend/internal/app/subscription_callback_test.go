// Package app 验证微信官方订阅回调的签名、AES安全模式、幂等与拒收撤销，不产生真实微信消息。
package app

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSubscriptionMessageCrypto 验证微信AES解密、AppID绑定与签名，确保不能透传攻击者加密消息。
func TestSubscriptionMessageCrypto(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	encodedKey := strings.TrimRight(base64.StdEncoding.EncodeToString(key), "=")
	body := []byte("<xml><Event>subscribe_msg_change_event</Event></xml>")
	plain := append(bytes.Repeat([]byte{3}, 16), make([]byte, 4)...)
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(body)))
	plain = append(plain, body...)
	plain = append(plain, []byte("wx_test")...)
	padding := 32 - len(plain)%32
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, _ := aes.NewCipher(key)
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:16]).CryptBlocks(out, plain)
	encoded := base64.StdEncoding.EncodeToString(out)
	decoded, err := decryptSubscriptionMessage(encoded, encodedKey, "wx_test")
	if err != nil || !bytes.Equal(decoded, body) {
		t.Fatal(err)
	}
	if _, err = decryptSubscriptionMessage(encoded, encodedKey, "other_app"); err == nil {
		t.Fatal("错误appid应拒绝")
	}
	if !validSubscriptionSignature(subscriptionSignature("token", "123", "nonce"), "token", "123", "nonce") {
		t.Fatal("签名错误")
	}
}

// TestIntegrationSubscriptionCallback 验证官方回调签名、重放幂等和拒收撤销，不依赖真正微信通知。
func TestIntegrationSubscriptionCallback(t *testing.T) {
	api := newTestAPI(t)
	subscriptionTestReset(t, api)
	api.App.Config.WechatMessageToken = "callback-test-token"
	subscriptionTestAuthorize(api, "callback-auth-123", "accept")
	body := fmt.Sprintf(`<xml><ToUserName>gh_test</ToUserName><FromUserName>test-private-openid</FromUserName><CreateTime>%d</CreateTime><MsgType>event</MsgType><Event>subscribe_msg_change_event</Event><SubscribeMsgChangeEvent><List><TemplateId>test_template_new_order</TemplateId><SubscribeStatusString>reject</SubscribeStatusString></List></SubscribeMsgChangeEvent></xml>`, time.Now().Unix())
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	query := url.Values{"timestamp": {stamp}, "nonce": {"testnonce"}, "signature": {subscriptionSignature(api.App.Config.WechatMessageToken, stamp, "testnonce")}}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/wechat/messages?"+query.Encode(), strings.NewReader(body))
		rec := httptest.NewRecorder()
		api.App.subscriptionCallback(rec, req)
		if rec.Code != 200 || rec.Body.String() != "success" {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	}
	var count int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM subscription_callbacks").Scan(&count)
	if count != 1 {
		t.Fatalf("重放重复%d", count)
	}
	query.Set("signature", "forged")
	req := httptest.NewRequest("POST", "/api/wechat/messages?"+query.Encode(), strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.App.subscriptionCallback(rec, req)
	if rec.Code != 403 {
		t.Fatal("未验签")
	}
	queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("callback-denial"))
	subscriptionTestDrain(t, api)
	if d := subscriptionTestDelivery(t, api); d.Status != "skipped_no_authorization" {
		t.Fatal(d.Status)
	}
}

// subscriptionTestEncrypted 将受控回调正文按微信协议封装，仅用于AES验签集成测试。
func subscriptionTestEncrypted(body []byte, keyBytes []byte, appID string) string {
	plain := append(bytes.Repeat([]byte{8}, 16), make([]byte, 4)...)
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(body)))
	plain = append(plain, body...)
	plain = append(plain, []byte(appID)...)
	pad := 32 - len(plain)%32
	plain = append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
	block, _ := aes.NewCipher(keyBytes)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, keyBytes[:16]).CryptBlocks(encrypted, plain)
	return base64.StdEncoding.EncodeToString(encrypted)
}

// TestIntegrationSubscriptionEncryptedCallback 验证安全模式URL验证和事件回调，不接受明文绕过或错误AppID。
func TestIntegrationSubscriptionEncryptedCallback(t *testing.T) {
	api := newTestAPI(t)
	subscriptionTestReset(t, api)
	api.App.Config.WechatMessageToken = "encrypted-callback-token"
	key := bytes.Repeat([]byte{5}, 32)
	api.App.Config.WechatMessageAESKey = strings.TrimRight(base64.StdEncoding.EncodeToString(key), "=")
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	encrypt := subscriptionTestEncrypted([]byte("encrypted-echo"), key, api.App.Config.WechatAppID)
	query := url.Values{"timestamp": {stamp}, "nonce": {"nonce-test"}, "encrypt_type": {"aes"}, "echostr": {encrypt}, "msg_signature": {subscriptionSignature(api.App.Config.WechatMessageToken, stamp, "nonce-test", encrypt)}}
	req := httptest.NewRequest("GET", "/api/wechat/messages?"+query.Encode(), nil)
	rec := httptest.NewRecorder()
	api.App.subscriptionCallback(rec, req)
	if rec.Code != 200 || rec.Body.String() != "encrypted-echo" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	body := fmt.Sprintf(`<xml><FromUserName>test-private-openid</FromUserName><CreateTime>%d</CreateTime><MsgType>event</MsgType><Event>subscribe_msg_sent_event</Event><SubscribeMsgSentEvent><List><TemplateId>test_template_new_order</TemplateId><MsgID>mock-message-id</MsgID><ErrorCode>0</ErrorCode><ErrorStatus>success</ErrorStatus></List></SubscribeMsgSentEvent></xml>`, time.Now().Unix())
	encrypt = subscriptionTestEncrypted([]byte(body), key, api.App.Config.WechatAppID)
	query.Del("echostr")
	query.Set("msg_signature", subscriptionSignature(api.App.Config.WechatMessageToken, stamp, "nonce-test", encrypt))
	req = httptest.NewRequest("POST", "/api/wechat/messages?"+query.Encode(), strings.NewReader("<xml><Encrypt>"+encrypt+"</Encrypt></xml>"))
	rec = httptest.NewRecorder()
	api.App.subscriptionCallback(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var count int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM subscription_callbacks").Scan(&count)
	if count != 1 {
		t.Fatalf("未记录官方发送结果%d", count)
	}
	query.Set("encrypt_type", "")
	query.Set("signature", subscriptionSignature(api.App.Config.WechatMessageToken, stamp, "nonce-test"))
	req = httptest.NewRequest("POST", "/api/wechat/messages?"+query.Encode(), strings.NewReader(body))
	rec = httptest.NewRecorder()
	api.App.subscriptionCallback(rec, req)
	if rec.Code != 403 {
		t.Fatal("安全模式不能接受明文绕过")
	}
}
