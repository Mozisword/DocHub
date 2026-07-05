package pay

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"
)

//微信下单参数
type WechatOrder struct {
	OrderNo    string //商户订单号
	Body       string //商品描述
	TotalFee   int    //金额(分)
	NotifyURL  string //异步通知地址
	ClientIP   string //终端IP
}

//微信统一下单响应
type wechatUnifiedResponse struct {
	XMLName    xml.Name `xml:"xml"`
	ReturnCode string   `xml:"return_code"`
	ReturnMsg  string   `xml:"return_msg"`
	ResultCode string   `xml:"result_code"`
	ErrCode    string   `xml:"err_code"`
	ErrCodeDes string   `xml:"err_code_des"`
	PrepayID   string   `xml:"prepay_id"`
	CodeURL    string   `xml:"code_url"`
}

//微信下单结果
type WechatResult struct {
	CodeURL string //二维码链接，前端据此生成二维码
}

//微信通知数据
type WechatNotifyData struct {
	OutTradeNo string //商户订单号
	TotalFee   int    //金额(分)
	TradeNo    string //微信支付订单号
	ResultCode string //业务结果 SUCCESS/FAIL
}

//MD5签名：参数按键排序拼接 + &key=APIKey，MD5后大写
func wechatSign(params map[string]string, apiKey string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(params[k])
		buf.WriteByte('&')
	}
	buf.WriteString("key=")
	buf.WriteString(apiKey)
	h := md5.New()
	h.Write([]byte(buf.String()))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

//生成随机字符串
func wechatNonceStr() string {
	return fmt.Sprintf("wx%d%d", time.Now().UnixNano(), time.Now().Unix())
}

//map转XML
func mapToXML(params map[string]string) string {
	var buf strings.Builder
	buf.WriteString("<xml>")
	for k, v := range params {
		buf.WriteString(fmt.Sprintf("<%s><![CDATA[%s]]></%s>", k, v, k))
	}
	buf.WriteString("</xml>")
	return buf.String()
}

//微信Native统一下单：返回二维码code_url
//@param            order       订单参数
//@return           result      下单结果
//@return           err         错误
func WechatUnifiedOrder(order WechatOrder) (result WechatResult, err error) {
	c := GetConfig()
	if !c.WechatEnable || c.WechatAppID == "" || c.WechatMchID == "" || c.WechatAPIKey == "" {
		return result, errors.New("微信支付未正确配置")
	}

	params := map[string]string{
		"appid":            c.WechatAppID,
		"mch_id":           c.WechatMchID,
		"nonce_str":        wechatNonceStr(),
		"body":             order.Body,
		"out_trade_no":     order.OrderNo,
		"total_fee":        fmt.Sprintf("%d", order.TotalFee),
		"spbill_create_ip": order.ClientIP,
		"notify_url":       order.NotifyURL,
		"trade_type":       "NATIVE",
	}
	params["sign"] = wechatSign(params, c.WechatAPIKey)

	xmlStr := mapToXML(params)
	resp, err := http.Post("https://api.mch.weixin.qq.com/pay/unifiedorder", "text/xml", bytes.NewBufferString(xmlStr))
	if err != nil {
		return result, fmt.Errorf("请求微信下单接口失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var uni wechatUnifiedResponse
	if err = xml.Unmarshal(body, &uni); err != nil {
		return result, fmt.Errorf("解析微信响应失败: %v", err)
	}
	if uni.ReturnCode != "SUCCESS" {
		return result, fmt.Errorf("微信下单失败: %s", uni.ReturnMsg)
	}
	if uni.ResultCode != "SUCCESS" {
		return result, fmt.Errorf("微信下单业务失败: %s %s", uni.ErrCode, uni.ErrCodeDes)
	}
	if uni.CodeURL == "" {
		return result, errors.New("微信未返回code_url")
	}
	result.CodeURL = uni.CodeURL
	return result, nil
}

//微信异步通知验签与解析
//@param            xmlBody     通知XML原文
//@return           data        解析后的通知数据
//@return           err         验签失败返回错误
func WechatVerifyNotify(xmlBody []byte) (data WechatNotifyData, err error) {
	c := GetConfig()
	if c.WechatAPIKey == "" {
		return data, errors.New("微信APIKey未配置")
	}

	//解析XML为map
	m, err := xmlToMap(xmlBody)
	if err != nil {
		return data, fmt.Errorf("解析通知XML失败: %v", err)
	}

	sign := m["sign"]
	if sign == "" {
		return data, errors.New("通知缺少sign")
	}

	//验签
	expected := wechatSign(m, c.WechatAPIKey)
	if !strings.EqualFold(expected, sign) {
		return data, errors.New("微信通知验签失败")
	}

	data.OutTradeNo = m["out_trade_no"]
	data.TradeNo = m["transaction_id"]
	fmt.Sscanf(m["total_fee"], "%d", &data.TotalFee)
	data.ResultCode = m["result_code"]
	return data, nil
}

//构建微信通知成功响应XML
func WechatNotifyOK() string {
	return "<xml><return_code><![CDATA[SUCCESS]]></return_code></xml>"
}

//构建微信通知失败响应XML
func WechatNotifyFail(msg string) string {
	return fmt.Sprintf("<xml><return_code><![CDATA[FAIL]]></return_code><return_msg><![CDATA[%s]]></return_msg></xml>", msg)
}

//XML转map
func xmlToMap(xmlData []byte) (map[string]string, error) {
	result := make(map[string]string)
	dec := xml.NewDecoder(bytes.NewReader(xmlData))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			var val string
			if err := dec.DecodeElement(&val, &se); err == nil {
				if se.Name.Local != "xml" {
					result[se.Name.Local] = val
				}
			}
		}
	}
	if len(result) == 0 {
		return nil, errors.New("XML解析结果为空")
	}
	return result, nil
}
