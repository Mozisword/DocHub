package AdminControllers

import (
	"strings"

	"github.com/TruthHun/DocHub/helper"
	"github.com/TruthHun/DocHub/helper/conv"
	"github.com/TruthHun/DocHub/models"
)

//订单管理控制器
type OrderController struct {
	BaseController
}

func (this *OrderController) Prepare() {
	this.BaseController.Prepare()
	this.Data["IsOrder"] = true
}

//订单列表
func (this *OrderController) List() {
	var (
		condition []string
		args      []interface{}
		listRows  = 15
		p         = 1
		status    string
		payType   string
		keyword   string
	)
	params := conv.Path2Map(this.GetString(":splat"))

	//页码
	if v, ok := params["p"]; ok {
		p = helper.Interface2Int(v)
	} else {
		p, _ = this.GetInt("p")
	}
	p = helper.NumberRange(p, 1, 1000000)

	if v, ok := params["status"]; ok {
		status = v
	} else {
		status = this.GetString("status")
	}
	if v, ok := params["paytype"]; ok {
		payType = v
	} else {
		payType = this.GetString("paytype")
	}
	if v, ok := params["keyword"]; ok {
		keyword = v
	} else {
		keyword = this.GetString("keyword")
	}

	if status != "" {
		condition = append(condition, "o.Status=?")
		args = append(args, status)
		this.Data["Status"] = status
	}
	if payType != "" {
		condition = append(condition, "o.PayType=?")
		args = append(args, payType)
		this.Data["PayType"] = payType
	}
	if keyword != "" {
		condition = append(condition, "(o.OrderNo like ? or u.Username like ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
		this.Data["Keyword"] = keyword
	}

	cond := strings.Join(condition, " and ")
	orders, total, _ := models.NewOrder().List(p, listRows, cond, args...)

	this.Data["Orders"] = orders
	this.Data["Total"] = total
	this.Data["ListRows"] = listRows
	this.Data["TotalRows"] = total
	this.Data["Page"] = helper.Paginations(6, int(total), listRows, p, "/admin/order/list/", "status", status, "paytype", payType, "keyword", keyword)
	this.Data["Title"] = "订单管理"
	this.TplName = "Order/list.html"
}
