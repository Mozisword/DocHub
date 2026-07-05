package HomeControllers

import (
	"strconv"

	"github.com/TruthHun/DocHub/helper"
	"github.com/TruthHun/DocHub/helper/pay"
	"github.com/TruthHun/DocHub/models"
)

//支付控制器
type PayController struct {
	BaseController
}

//充值页面
func (this *PayController) Recharge() {
	if this.IsLogin == 0 {
		this.Redirect("/user/login", 302)
		return
	}
	uid := this.IsLogin
	this.Xsrf()
	this.Data["IsRecharge"] = true
	this.Data["Seo"] = models.NewSeo().GetByPage("PC-Recharge", "充值金币", "充值,金币", "充值金币", this.Sys.Site)
	this.Data["UserInfo"] = models.NewUser().UserInfo(uid)
	this.Data["PayEnable"] = pay.GetConfig().Enable
	this.Data["AlipayEnable"] = pay.GetConfig().AlipayEnable
	this.Data["WechatEnable"] = pay.GetConfig().WechatEnable
	this.Data["CoinRate"] = pay.GetConfig().CoinRate
	this.Data["TestMode"] = pay.IsTestMode()
	this.Data["PageId"] = "wenku-recharge"
	this.TplName = "recharge.html"
}

//创建订单并跳转支付
func (this *PayController) CreateOrder() {
	if this.IsLogin == 0 {
		this.ResponseJson(false, "请先登录")
	}
	uid := this.IsLogin
	amount, _ := strconv.ParseFloat(this.GetString("amount"), 64)
	payType := this.GetString("paytype")
	if amount < 0.01 {
		this.ResponseJson(false, "充值金额不能小于0.01元")
	}
	if payType != pay.PayTypeAlipay && payType != pay.PayTypeWechat {
		this.ResponseJson(false, "请选择支付方式")
	}

	cfg := pay.GetConfig()
	//测试模式：直接模拟支付成功
	if pay.IsTestMode() || !cfg.Enable {
		order, err := models.NewOrder().Create(uid, amount, payType)
		if err != nil {
			this.ResponseJson(false, "创建订单失败: "+err.Error())
		}
		//模拟支付成功
		tradeNo := "TEST" + order.OrderNo
		if err := models.NewOrder().PaySuccess(order.OrderNo, tradeNo, amount); err != nil {
			this.ResponseJson(false, "支付处理失败: "+err.Error())
		}
		this.ResponseJson(true, "测试模式支付成功，金币已充值", map[string]interface{}{
			"order_no": order.OrderNo,
			"redirect": "/pay/result?order=" + order.OrderNo,
		})
	}

	order, err := models.NewOrder().Create(uid, amount, payType)
	if err != nil {
		this.ResponseJson(false, "创建订单失败: "+err.Error())
	}

	if payType == pay.PayTypeAlipay {
		if !cfg.AlipayEnable {
			this.ResponseJson(false, "支付宝支付未开启")
		}
		result, err := pay.AlipayTradePagePay(pay.AlipayOrder{
			OrderNo:     order.OrderNo,
			Subject:     "充值金币 " + strconv.FormatFloat(amount, 'f', 2, 64) + " 元",
			TotalAmount: strconv.FormatFloat(amount, 'f', 2, 64),
			ReturnURL:   pay.NotifyURL("/pay/alipay/return"),
			NotifyURL:   pay.NotifyURL("/pay/alipay/notify"),
		})
		if err != nil {
			this.ResponseJson(false, "支付宝下单失败: "+err.Error())
		}
		this.ResponseJson(true, "正在跳转支付宝", map[string]interface{}{
			"pay_url": result.PayURL,
		})
	}

	if payType == pay.PayTypeWechat {
		if !cfg.WechatEnable {
			this.ResponseJson(false, "微信支付未开启")
		}
		result, err := pay.WechatUnifiedOrder(pay.WechatOrder{
			OrderNo:   order.OrderNo,
			Body:      "充值金币",
			TotalFee:  int(amount * 100),
			NotifyURL: pay.NotifyURL("/pay/wechat/notify"),
			ClientIP:  this.Ctx.Input.IP(),
		})
		if err != nil {
			this.ResponseJson(false, "微信下单失败: "+err.Error())
		}
		this.ResponseJson(true, "请扫码支付", map[string]interface{}{
			"code_url":  result.CodeURL,
			"order_no":  order.OrderNo,
			"redirect":  "/pay/wechat/qrcode?order=" + order.OrderNo,
		})
	}
	this.ResponseJson(false, "未知的支付方式")
}

//支付宝同步返回页
func (this *PayController) AlipayReturn() {
	orderNo := this.GetString("out_trade_no")
	if orderNo == "" {
		this.Abort("404")
	}
	this.Redirect("/pay/result?order="+orderNo, 302)
}

//支付宝异步通知
func (this *PayController) AlipayNotify() {
	//收集所有参数
	params := make(map[string]string)
	for k, v := range this.Ctx.Request.Form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	orderNo, tradeNo, amount, tradeStatus, err := pay.AlipayVerifyNotify(params)
	if err != nil {
		helper.Logger.Error("支付宝验签失败: %v", err)
		this.Ctx.WriteString("fail")
		return
	}
	if tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		this.Ctx.WriteString("success")
		return
	}
	amt, _ := strconv.ParseFloat(amount, 64)
	if err := models.NewOrder().PaySuccess(orderNo, tradeNo, amt); err != nil {
		helper.Logger.Error("支付宝回调处理失败: %v", err)
		this.Ctx.WriteString("fail")
		return
	}
	this.Ctx.WriteString("success")
}

//微信扫码支付页
func (this *PayController) WechatQrcode() {
	if this.IsLogin == 0 {
		this.Redirect("/user/login", 302)
		return
	}
	uid := this.IsLogin
	orderNo := this.GetString("order")
	order := models.NewOrder().GetByOrderNo(orderNo)
	if order.Id == 0 || order.Uid != uid {
		this.Abort("404")
	}
	//重新下单获取code_url（页面刷新或首次进入）
	cfg := pay.GetConfig()
	result, err := pay.WechatUnifiedOrder(pay.WechatOrder{
		OrderNo:   order.OrderNo,
		Body:      "充值金币",
		TotalFee:  int(order.Amount * 100),
		NotifyURL: pay.NotifyURL("/pay/wechat/notify"),
		ClientIP:  this.Ctx.Input.IP(),
	})
	codeURL := ""
	if err == nil {
		codeURL = result.CodeURL
	}
	this.Data["Order"] = order
	this.Data["CodeURL"] = codeURL
	this.Data["WechatEnable"] = cfg.WechatEnable
	this.Data["Seo"] = models.NewSeo().GetByPage("PC-Pay", "微信扫码支付", "微信支付", "微信扫码支付", this.Sys.Site)
	this.Data["PageId"] = "wenku-wechat-pay"
	this.TplName = "wechat_qrcode.html"
}

//微信异步通知
func (this *PayController) WechatNotify() {
	body := this.Ctx.Input.RequestBody
	data, err := pay.WechatVerifyNotify(body)
	if err != nil {
		helper.Logger.Error("微信验签失败: %v", err)
		this.Ctx.WriteString(pay.WechatNotifyFail("验签失败"))
		return
	}
	if data.ResultCode != "SUCCESS" {
		this.Ctx.WriteString(pay.WechatNotifyFail("业务失败"))
		return
	}
	amount := float64(data.TotalFee) / 100
	if err := models.NewOrder().PaySuccess(data.OutTradeNo, data.TradeNo, amount); err != nil {
		helper.Logger.Error("微信回调处理失败: %v", err)
		this.Ctx.WriteString(pay.WechatNotifyFail(err.Error()))
		return
	}
	this.Ctx.WriteString(pay.WechatNotifyOK())
}

//查询订单状态（前端轮询）
func (this *PayController) OrderStatus() {
	orderNo := this.GetString("order")
	order := models.NewOrder().GetByOrderNo(orderNo)
	if order.Id == 0 {
		this.ResponseJson(false, "订单不存在")
	}
	this.ResponseJson(true, "ok", map[string]interface{}{
		"status": order.Status,
		"coin":   order.Coin,
	})
}

//支付结果页
func (this *PayController) Result() {
	if this.IsLogin == 0 {
		this.Redirect("/user/login", 302)
		return
	}
	uid := this.IsLogin
	orderNo := this.GetString("order")
	order := models.NewOrder().GetByOrderNo(orderNo)
	if order.Id == 0 {
		this.Abort("404")
	}
	this.Data["Order"] = order
	this.Data["UserInfo"] = models.NewUser().UserInfo(uid)
	statusText := "待支付"
	if order.Status == models.OrderStatusPaid {
		statusText = "支付成功"
	} else if order.Status == models.OrderStatusClosed {
		statusText = "已关闭"
	}
	this.Data["StatusText"] = statusText
	this.Data["Seo"] = models.NewSeo().GetByPage("PC-PayResult", "支付结果", "支付结果", "支付结果", this.Sys.Site)
	this.Data["PageId"] = "wenku-pay-result"
	this.TplName = "result.html"
}
