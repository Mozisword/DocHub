package pay

import (
	"fmt"
	"strings"

	"github.com/astaxie/beego"
)

//支付方式
const (
	PayTypeAlipay = "alipay"
	PayTypeWechat = "wechat"
)

//全局支付配置
type Config struct {
	Enable  bool //是否启用支付
	CoinRate int  //1元=N金币

	AlipayEnable bool   //支付宝是否启用
	AlipayAppID  string //支付宝应用ID
	AlipayPrivateKey string //应用私钥
	AlipayPublicKey  string //支付宝公钥
	AlipayGateway    string //支付宝网关

	WechatEnable bool   //微信支付是否启用
	WechatAppID  string //微信应用AppID(公众号/小程序)
	WechatMchID  string //微信商户号
	WechatAPIKey string //微信API密钥
}

var cfg Config

//加载支付配置（从app.conf读取）
func LoadConfig() Config {
	cfg = Config{
		Enable:  beego.AppConfig.DefaultBool("pay::enable", false),
		CoinRate: beego.AppConfig.DefaultInt("pay::coinrate", 10),

		AlipayEnable: beego.AppConfig.DefaultBool("alipay::enable", false),
		AlipayAppID:  strings.TrimSpace(beego.AppConfig.String("alipay::appid")),
		AlipayPrivateKey: strings.TrimSpace(beego.AppConfig.String("alipay::privatekey")),
		AlipayPublicKey:  strings.TrimSpace(beego.AppConfig.String("alipay::publickey")),
		AlipayGateway:    beego.AppConfig.DefaultString("alipay::gateway", "https://openapi.alipay.com/gateway.do"),

		WechatEnable: beego.AppConfig.DefaultBool("wechatpay::enable", false),
		WechatAppID:  strings.TrimSpace(beego.AppConfig.String("wechatpay::appid")),
		WechatMchID:  strings.TrimSpace(beego.AppConfig.String("wechatpay::mchid")),
		WechatAPIKey: strings.TrimSpace(beego.AppConfig.String("wechatpay::apikey")),
	}
	return cfg
}

//获取配置（懒加载）
func GetConfig() Config {
	if cfg.CoinRate == 0 && !cfg.Enable {
		LoadConfig()
	}
	return cfg
}

//判断是否处于测试模式（未配置真实凭证时）
func IsTestMode() bool {
	c := GetConfig()
	return !c.AlipayEnable && !c.WechatEnable
}

//构建通知URL
func NotifyURL(path string) string {
	domain := beego.AppConfig.DefaultString("pay::sitedomain", "")
	if domain == "" {
		domain = beego.AppConfig.DefaultString("domain::pc", "")
	}
	domain = strings.TrimRight(domain, "/")
	if domain == "" {
		return path
	}
	return fmt.Sprintf("%s%s", domain, path)
}
