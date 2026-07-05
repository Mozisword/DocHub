package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/TruthHun/DocHub/helper"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
)

//充值订单表
type Order struct {
	Id         int     `orm:"column(Id)"`
	OrderNo    string  `orm:"size(32);unique;column(OrderNo)"`            //商户订单号
	Uid        int     `orm:"index;column(Uid)"`                          //用户id
	Amount     float64 `orm:"digits(10);decimals(2);default(0);column(Amount)"` //充值金额(元)
	Coin       int     `orm:"default(0);column(Coin)"`                    //获得金币数量
	PayType    string  `orm:"size(20);default();column(PayType)"`         //支付方式：alipay/wechat
	Status     int     `orm:"default(0);index;column(Status)"`            //订单状态：0待支付 1已支付 2已关闭
	TimeCreate int     `orm:"column(TimeCreate)"`                         //创建时间
	TimePay    int     `orm:"column(TimePay);default(0)"`                 //支付时间
	TradeNo    string  `orm:"size(64);default();column(TradeNo)"`         //第三方交易流水号
}

func NewOrder() *Order {
	return &Order{}
}

func GetTableOrder() string {
	return getTable("order")
}

//订单状态常量
const (
	OrderStatusUnpaid = 0 //待支付
	OrderStatusPaid   = 1 //已支付
	OrderStatusClosed = 2 //已关闭
)

//生成商户订单号：年月日时分秒 + 6位随机
func GenOrderNo() string {
	return fmt.Sprintf("DH%s%06d", time.Now().Format("20060102150405"), helper.RangeRand(100000, 999999))
}

//创建充值订单
//@param            uid         用户id
//@param            amount      充值金额(元)
//@param            payType     支付方式 alipay/wechat
//@return           order       订单
//@return           err         错误
func (this *Order) Create(uid int, amount float64, payType string) (order Order, err error) {
	coinRate := beego.AppConfig.DefaultInt("pay::coinrate", 10) //1元=N金币
	order = Order{
		OrderNo:    GenOrderNo(),
		Uid:        uid,
		Amount:     amount,
		Coin:       int(amount * float64(coinRate)),
		PayType:    payType,
		Status:     OrderStatusUnpaid,
		TimeCreate: int(time.Now().Unix()),
	}
	_, err = orm.NewOrm().Insert(&order)
	return
}

//根据订单号查询订单
func (this *Order) GetByOrderNo(orderNo string) (order Order) {
	orm.NewOrm().QueryTable(GetTableOrder()).Filter("OrderNo", orderNo).One(&order)
	return
}

//根据id查询订单
func (this *Order) GetById(id int) (order Order) {
	orm.NewOrm().QueryTable(GetTableOrder()).Filter("Id", id).One(&order)
	return
}

//支付成功：更新订单状态并为用户充值金币（事务）
//@param            orderNo     商户订单号
//@param            tradeNo     第三方交易流水号
//@param            amount      实际支付金额(元)，用于校验
//@return           err         错误
func (this *Order) PaySuccess(orderNo, tradeNo string, amount float64) (err error) {
	order := this.GetByOrderNo(orderNo)
	if order.Id == 0 {
		return fmt.Errorf("订单不存在: %s", orderNo)
	}
	if order.Status == OrderStatusPaid {
		return nil //已处理过，幂等返回
	}
	//金额校验（允许浮点微小误差）
	if absf(order.Amount-amount) > 0.01 {
		return fmt.Errorf("订单金额不匹配: 期望%.2f, 实际%.2f", order.Amount, amount)
	}

	o := orm.NewOrm()
	o.Begin()
	defer func() {
		if err == nil {
			o.Commit()
		} else {
			o.Rollback()
		}
	}()

	//更新订单状态
	order.Status = OrderStatusPaid
	order.TradeNo = tradeNo
	order.TimePay = int(time.Now().Unix())
	if _, err = o.Update(&order, "Status", "TradeNo", "TimePay"); err != nil {
		return
	}

	//用户金币增加
	sql := fmt.Sprintf("update `%v` set `Coin`=`Coin`+? where Id=?", GetTableUserInfo())
	if _, err = o.Raw(sql, order.Coin, order.Uid).Exec(); err != nil {
		return
	}

	//记录金币变更日志
	log := CoinLog{
		Uid:        order.Uid,
		Coin:       order.Coin,
		Log:        fmt.Sprintf("充值金币，订单号 %s，支付方式 %s，金额 %.2f 元", orderNo, order.PayType, order.Amount),
		TimeCreate: int(time.Now().Unix()),
	}
	if _, err = o.Insert(&log); err != nil {
		return
	}
	return
}

//订单列表（后台）
//@param            p           页码
//@param            listRows    每页记录数
//@param            cond        查询条件字符串(不含where)，可为空
//@param            args        查询条件参数
//@return           params      数据列表
//@return           total       总记录数
//@return           err         错误
func (this *Order) List(p, listRows int, cond string, args ...interface{}) (params []orm.Params, total int64, err error) {
	o := orm.NewOrm()
	where := ""
	if strings.TrimSpace(cond) != "" {
		where = " where " + cond
	}
	//统计
	var cnt []orm.Params
	sqlCount := fmt.Sprintf("select count(Id) cnt from `%v`%v limit 1", GetTableOrder(), where)
	if _, e := o.Raw(sqlCount, args...).Values(&cnt); e == nil && len(cnt) > 0 {
		total = int64(helper.Interface2Int(cnt[0]["cnt"]))
	}
	//列表(关联用户表取用户名)
	sql := fmt.Sprintf("select o.*, u.Username from `%v` o left join `%v` u on o.Uid=u.Id %v order by o.Id desc limit %v offset %v",
		GetTableOrder(), GetTableUser(), where, listRows, (p-1)*listRows)
	_, err = o.Raw(sql, args...).Values(&params)
	return
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
