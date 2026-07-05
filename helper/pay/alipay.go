package pay

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

//支付宝下单参数
type AlipayOrder struct {
	OrderNo    string //商户订单号
	Subject    string //订单标题
	TotalAmount string //金额(元)，字符串如 "0.01"
	ReturnURL  string //同步返回地址
	NotifyURL  string //异步通知地址
}

//支付宝下单结果
type AlipayResult struct {
	PayURL string //跳转支付地址(完整URL)
}

//加载RSA私钥（兼容PKCS1和PKCS8）
func loadPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	pemStr = strings.TrimSpace(pemStr)
	//如果不含PEM头，补上
	if !strings.Contains(pemStr, "-----BEGIN") {
		pemStr = "-----BEGIN RSA PRIVATE KEY-----\n" + pemStr + "\n-----END RSA PRIVATE KEY-----"
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("私钥PEM解码失败")
	}
	//先尝试PKCS1
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	//再尝试PKCS8
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败: %v", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("私钥类型不是RSA")
	}
	return rsaKey, nil
}

//加载RSA公钥（支付宝公钥）
func loadPublicKey(pemStr string) (*rsa.PublicKey, error) {
	pemStr = strings.TrimSpace(pemStr)
	if !strings.Contains(pemStr, "-----BEGIN") {
		pemStr = "-----BEGIN PUBLIC KEY-----\n" + pemStr + "\n-----END PUBLIC KEY-----"
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("公钥PEM解码失败")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("公钥解析失败: %v", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥类型不是RSA")
	}
	return rsaPub, nil
}

//对待签名内容进行RSA2(SHA256withRSA)签名，返回base64
func rsa2Sign(privateKey *rsa.PrivateKey, content string) (string, error) {
	h := sha256.New()
	h.Write([]byte(content))
	hashed := h.Sum(nil)
	sig, err := rsa.SignPKCS1v15(nil, privateKey, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

//用支付宝公钥验签
func rsa2Verify(publicKey *rsa.PublicKey, content, sign string) error {
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return fmt.Errorf("签名base64解码失败: %v", err)
	}
	h := sha256.New()
	h.Write([]byte(content))
	hashed := h.Sum(nil)
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed, sig)
}

//把map参数按键ASCII升序拼接成 key=value&key=value 形式（值不做URL编码）
func buildSignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(params[k])
	}
	return buf.String()
}

//支付宝电脑网站支付下单：返回跳转URL
//@param            order       订单参数
//@return           result      下单结果
//@return           err         错误
func AlipayTradePagePay(order AlipayOrder) (result AlipayResult, err error) {
	c := GetConfig()
	if !c.AlipayEnable || c.AlipayAppID == "" || c.AlipayPrivateKey == "" {
		return result, errors.New("支付宝未正确配置")
	}
	priv, err := loadPrivateKey(c.AlipayPrivateKey)
	if err != nil {
		return result, fmt.Errorf("私钥加载失败: %v", err)
	}

	//业务参数biz_content
	biz := map[string]interface{}{
		"out_trade_no": order.OrderNo,
		"total_amount": order.TotalAmount,
		"subject":      order.Subject,
		"product_code": "FAST_INSTANT_TRADE_PAY",
	}
	bizBytes, _ := json.Marshal(biz)
	bizContent := string(bizBytes)

	//公共请求参数
	params := map[string]string{
		"app_id":      c.AlipayAppID,
		"method":      "alipay.trade.page.pay",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
		"notify_url":  order.NotifyURL,
		"return_url":  order.ReturnURL,
	}

	//计算签名
	signContent := buildSignContent(params)
	sign, err := rsa2Sign(priv, signContent)
	if err != nil {
		return result, fmt.Errorf("签名失败: %v", err)
	}
	params["sign"] = sign

	//拼接跳转URL（每个值URL编码）
	var q strings.Builder
	first := true
	for k, v := range params {
		if v == "" {
			continue
		}
		if !first {
			q.WriteByte('&')
		}
		first = false
		q.WriteString(url.QueryEscape(k))
		q.WriteByte('=')
		q.WriteString(url.QueryEscape(v))
	}
	result.PayURL = c.AlipayGateway + "?" + q.String()
	return result, nil
}

//支付宝异步通知验签与解析
//@param            notifyParams   通知收到的所有参数(已URL解码后的原始值)
//@return           orderNo        商户订单号
//@return           tradeNo        支付宝交易号
//@return           amount         实付金额
//@return           tradeStatus    交易状态
//@return           err            验签失败返回错误
func AlipayVerifyNotify(notifyParams map[string]string) (orderNo, tradeNo, amount, tradeStatus string, err error) {
	c := GetConfig()
	if c.AlipayPublicKey == "" {
		return "", "", "", "", errors.New("支付宝公钥未配置")
	}
	pub, err := loadPublicKey(c.AlipayPublicKey)
	if err != nil {
		return "", "", "", "", fmt.Errorf("公钥加载失败: %v", err)
	}

	sign := notifyParams["sign"]
	if sign == "" {
		return "", "", "", "", errors.New("通知缺少sign参数")
	}

	//构建验签内容：除sign和sign_type外的所有非空参数
	verifyMap := make(map[string]string)
	for k, v := range notifyParams {
		if k == "sign" || k == "sign_type" {
			continue
		}
		verifyMap[k] = v
	}
	signContent := buildSignContent(verifyMap)

	if err := rsa2Verify(pub, signContent, sign); err != nil {
		return "", "", "", "", fmt.Errorf("验签失败: %v", err)
	}

	orderNo = notifyParams["out_trade_no"]
	tradeNo = notifyParams["trade_no"]
	amount = notifyParams["total_amount"]
	tradeStatus = notifyParams["trade_status"]
	return
}
