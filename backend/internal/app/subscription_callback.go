// Package app 验证微信订阅事件签名和可选AES消息加密，回调仅登记真实事件，不接受客户端伪造无限额度。
package app

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// subscriptionCallbackEvent 表示微信官方订阅授权或发送结果，原始OpenID只用于匹配当前小程序身份。
type subscriptionCallbackEvent struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	Popup        struct {
		List []subscriptionCallbackItem `xml:"List"`
	} `xml:"SubscribeMsgPopupEvent"`
	Change struct {
		List []subscriptionCallbackItem `xml:"List"`
	} `xml:"SubscribeMsgChangeEvent"`
	Sent struct {
		List []subscriptionCallbackItem `xml:"List"`
	} `xml:"SubscribeMsgSentEvent"`
}

// subscriptionCallbackItem 描述官方模板选择与发送结果项，不直接作为任意业务发送命令。
type subscriptionCallbackItem struct {
	TemplateID  string `xml:"TemplateId"`
	Status      string `xml:"SubscribeStatusString"`
	PopupScene  int    `xml:"PopupScene"`
	MsgID       string `xml:"MsgID"`
	ErrorCode   int    `xml:"ErrorCode"`
	ErrorStatus string `xml:"ErrorStatus"`
}

// subscriptionSignature 计算官方排序签名，不在日志输出Token、加密正文或OpenID。
func subscriptionSignature(parts ...string) string {
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

// validSubscriptionSignature 使用常量时间比较校验微信签名。
func validSubscriptionSignature(signature string, parts ...string) bool {
	expected := subscriptionSignature(parts...)
	return len(signature) == len(expected) && subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1
}

// decryptSubscriptionMessage 解密微信AES-CBC消息并验证长度、填充与末尾小程序AppID。
func decryptSubscriptionMessage(encoded, key, appID string) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(key + "=")
	if err != nil || len(keyBytes) != 32 {
		return nil, fmt.Errorf("微信消息加密配置无效")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("微信加密消息格式无效")
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, keyBytes[:aes.BlockSize]).CryptBlocks(plain, raw)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > 32 || padding > len(plain) || !bytes.Equal(plain[len(plain)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		return nil, fmt.Errorf("微信加密消息填充无效")
	}
	plain = plain[:len(plain)-padding]
	if len(plain) < 20 {
		return nil, fmt.Errorf("微信加密消息长度无效")
	}
	size := int(binary.BigEndian.Uint32(plain[16:20]))
	if size < 0 || size > len(plain)-20 || string(plain[20+size:]) != appID {
		return nil, fmt.Errorf("微信加密消息小程序不匹配")
	}
	return plain[20 : 20+size], nil
}

// subscriptionCallback 接受已验签官方回调；重复事件幂等登记，拒收撤销未消费候选，发送结果留独立记录。
func (a *App) subscriptionCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	token := a.Config.WechatMessageToken
	if token == "" || a.Config.WechatAppID == "" {
		http.Error(w, "not configured", 503)
		return
	}
	query := r.URL.Query()
	stamp, err := strconv.ParseInt(query.Get("timestamp"), 10, 64)
	if err != nil || time.Since(time.Unix(stamp, 0)) > 10*time.Minute || time.Until(time.Unix(stamp, 0)) > time.Minute || len(query.Get("nonce")) > 256 {
		http.Error(w, "invalid timestamp", 403)
		return
	}
	encrypted := query.Get("encrypt_type") == "aes"
	if a.Config.WechatMessageAESKey != "" && !encrypted {
		http.Error(w, "encrypted callback required", 403)
		return
	}
	var raw []byte
	if r.Method == "GET" {
		raw = []byte(query.Get("echostr"))
	} else {
		raw, err = io.ReadAll(io.LimitReader(r.Body, 65537))
		if err != nil || len(raw) > 65536 {
			http.Error(w, "invalid body", 400)
			return
		}
	}
	if encrypted {
		encoded := string(raw)
		if r.Method == "POST" {
			var envelope struct {
				Encrypt string `xml:"Encrypt"`
			}
			if xml.Unmarshal(raw, &envelope) != nil {
				http.Error(w, "invalid envelope", 400)
				return
			}
			encoded = envelope.Encrypt
		}
		if !validSubscriptionSignature(query.Get("msg_signature"), token, query.Get("timestamp"), query.Get("nonce"), encoded) {
			http.Error(w, "invalid signature", 403)
			return
		}
		raw, err = decryptSubscriptionMessage(encoded, a.Config.WechatMessageAESKey, a.Config.WechatAppID)
		if err != nil {
			http.Error(w, "invalid encrypted message", 403)
			return
		}
	} else if !validSubscriptionSignature(query.Get("signature"), token, query.Get("timestamp"), query.Get("nonce")) {
		http.Error(w, "invalid signature", 403)
		return
	}
	if r.Method == "GET" {
		w.Write(raw)
		return
	}
	var event subscriptionCallbackEvent
	if xml.Unmarshal(raw, &event) != nil || event.MsgType != "event" || event.FromUserName == "" {
		http.Error(w, "invalid event", 400)
		return
	}
	eventTime := time.Unix(event.CreateTime, 0)
	if event.CreateTime <= 0 || time.Until(eventTime) > time.Minute {
		http.Error(w, "invalid event time", 400)
		return
	}
	items := []subscriptionCallbackItem{}
	switch event.Event {
	case "subscribe_msg_popup_event":
		items = event.Popup.List
	case "subscribe_msg_change_event":
		items = event.Change.List
	case "subscribe_msg_sent_event":
		items = event.Sent.List
	default:
		io.WriteString(w, "success")
		return
	}
	if len(items) > 100 {
		http.Error(w, "too many items", 400)
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	defer tx.Rollback(r.Context())
	var userID string
	err = tx.QueryRow(r.Context(), `SELECT id FROM identities WHERE data->>'openid'=$1 AND data->>'appid'=$2 LIMIT 1`, event.FromUserName, a.Config.WechatAppID).Scan(&userID)
	if missing(err) {
		io.WriteString(w, "success")
		return
	}
	if err != nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	id := "callback_" + subscriptionHash(string(raw), a.Config.WechatAppID)
	var claimed string
	err = tx.QueryRow(r.Context(), `INSERT INTO subscription_receipts(id,data) VALUES($1,jsonb_build_object('created_at',$2::text)) ON CONFLICT(id) DO NOTHING RETURNING id`, id, now()).Scan(&claimed)
	if missing(err) {
		io.WriteString(w, "success")
		return
	}
	if err != nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	for index, item := range items {
		if !subscriptionTemplatePattern.MatchString(item.TemplateID) {
			continue
		}
		recordID := id + "_" + strconv.Itoa(index)
		record := map[string]any{"user_id": userID, "event": event.Event, "template_id": item.TemplateID, "status": item.Status, "error_code": item.ErrorCode, "created_at": now()}
		if err = putRaw(r.Context(), tx, "subscription_callbacks", recordID, record); err != nil {
			http.Error(w, "temporarily unavailable", 503)
			return
		}
		if (event.Event == "subscribe_msg_popup_event" || event.Event == "subscribe_msg_change_event") && (item.Status == "reject" || item.Status == "ban") {
			auth := subscriptionAuthorization{ID: recordID, UserID: userID, AppID: a.Config.WechatAppID, TemplateID: item.TemplateID, Status: item.Status, Source: "wechat_callback", CreatedAt: eventTime.UTC().Format(time.RFC3339Nano), ExpiresAt: time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)}
			if err = putRaw(r.Context(), tx, "subscription_authorizations", recordID, auth); err != nil {
				http.Error(w, "temporarily unavailable", 503)
				return
			}
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	io.WriteString(w, "success")
}
