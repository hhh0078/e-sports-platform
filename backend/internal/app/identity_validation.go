// Package app 校验大陆居民身份证格式、校验位与上海自然日满十八周岁的年龄边界。
package app

import (
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const identityAPIURL = "https://v2.xxapi.cn/api/idCardCheck"

var identityCardPattern = regexp.MustCompile(`^[1-9][0-9]{16}[0-9X]$`)
var identityRequestPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// identityAdult 校验身份证有效结构，并按上海日期判断是否已经满十八周岁。
// card 为规范化证件号，at 为判断时刻；非法证件返回错误，合法证件返回成年状态。
func identityAdult(card string, at time.Time) (bool, error) {
	if !identityCardPattern.MatchString(card) {
		return false, fail(http.StatusBadRequest, "请输入有效的18位身份证号")
	}
	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	sum := 0
	for i, weight := range weights {
		sum += int(card[i]-'0') * weight
	}
	if card[17] != "10X98765432"[sum%11] || card[14:17] == "000" {
		return false, fail(http.StatusBadRequest, "身份证号校验失败，请检查后重新输入")
	}
	birthday, err := time.Parse("20060102", card[6:14])
	shanghai := at.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	if err != nil || birthday.Year() < 1900 || card[6:14] > shanghai.Format("20060102") {
		return false, fail(http.StatusBadRequest, "身份证出生日期无效，请检查后重新输入")
	}
	years := shanghai.Year() - birthday.Year()
	if shanghai.Format("0102") < birthday.Format("0102") {
		years--
	}
	return years >= 18, nil
}

// validateIdentityName 限制姓名为必要长度且不包含控制字符，防止无效调用及异常提示注入。
func validateIdentityName(name string) error {
	if !length(name, 2, 50) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fail(http.StatusBadRequest, "请输入2至50字的真实姓名")
	}
	return nil
}
