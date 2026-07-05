# DocHub 文库系统部署指南

## 项目简介

DocHub 是一个基于 Go + Beego 框架开发的文库系统（类似百度文库），支持文档上传、在线预览、下载、金币体系等。

本次改造在原项目基础上新增了 **支付宝 + 微信支付** 充值系统，支持用户通过充值金币来下载付费文档。

---

## 一、Docker Compose 部署（推荐）

### 1. 前置要求

- Docker 20.10+
- Docker Compose 2.0+

### 2. 快速启动

```bash
# 1. 复制环境变量配置文件
cp .env.example .env

# 2. 按需修改 .env 中的配置（数据库密码、支付参数等）
vi .env

# 3. 构建并启动服务
docker-compose up -d --build

# 4. 查看启动日志
docker-compose logs -f dochub

# 5. 验证服务是否正常
curl http://localhost:8090/
```

### 3. 服务管理

```bash
# 停止服务
docker-compose down

# 重启服务
docker-compose restart

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f dochub   # 应用日志
docker-compose logs -f mysql    # 数据库日志

# 更新代码后重新构建
docker-compose up -d --build dochub
```

### 4. 数据持久化

Docker 部署使用以下卷持久化数据：

| 卷名 | 容器路径 | 说明 |
|------|----------|------|
| mysql_data | /var/lib/mysql | MySQL 数据文件 |
| dochub_cache | /app/cache | Session 缓存 |
| dochub_logs | /app/logs | 应用日志 |
| dochub_upload | /app/upload | 用户上传文件 |
| dochub_runtime | /app/runtime | 运行时文件 |

> 备份数据库：`docker exec dochub-mysql mysqldump -u root -p<密码> dochub > backup.sql`

### 5. 环境变量说明

| 变量 | 默认值 | 说明 |
|------|--------|------|
| DB_ROOT_PASSWORD | dochub123456 | MySQL root 密码 |
| DB_NAME | dochub | 数据库名 |
| DB_PORT | 3307 | 宿主机映射端口 |
| WEB_PORT | 8090 | Web 服务端口 |
| RUNMODE | dev | 运行模式 (dev/prod) |
| PAY_ENABLE | false | 支付总开关 (false=测试模式) |
| PAY_COINRATE | 10 | 1元兑换金币数 |
| ALIPAY_ENABLE | false | 支付宝开关 |
| WECHAT_ENABLE | false | 微信支付开关 |

---

## 二、本机运行部署

### 1. 前置要求

- Go 1.21+
- MySQL 5.7+ 或 MariaDB 10.3+

### 2. 编译

```bash
cd DocHub

# 设置 Go 代理（国内环境）
export GOPROXY=https://goproxy.cn,direct
export GO111MODULE=on

# 编译
go build -o DocHub .

# 或指定 CGO 禁用
CGO_ENABLED=0 go build -o DocHub .
```

### 3. 配置数据库

```sql
CREATE DATABASE dochub DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

### 4. 配置 app.conf

复制 `conf/app.conf.example` 为 `conf/app.conf`，修改数据库连接信息：

```ini
[db]
host = localhost
port = 3306
user = root
password = 你的数据库密码
database = dochub
prefix = hc_
charset = utf8
```

### 5. 创建必要目录

```bash
mkdir -p cache/session logs upload runtime
```

### 6. 启动

```bash
./DocHub
```

首次启动会自动建表并初始化种子数据。

---

## 三、支付系统配置

### 测试模式（默认）

`app.conf` 或 `.env` 中设置 `PAY_ENABLE = false`，充值流程会直接模拟支付成功，无需真实支付。适合开发和功能演示。

### 真实支付

#### 支付宝配置

1. 访问 https://open.alipay.com 创建应用
2. 获取 AppID、应用私钥、支付宝公钥
3. 配置 `app.conf`：

```ini
[pay]
enable = true
coinrate = 10          # 1元 = 10金币
sitedomain = https://你的域名

[alipay]
enable = true
appid = 你的AppID
privatekey = 应用私钥
publickey = 支付宝公钥
gateway = https://openapi.alipay.com/gateway.do
```

#### 微信支付配置

1. 访问 https://pay.weixin.qq.com 开通商户
2. 获取 AppID、商户号、API密钥
3. 配置 `app.conf`：

```ini
[wechatpay]
enable = true
appid = 你的AppID
mchid = 商户号
apikey = API密钥（32位）
```

> **注意**：异步通知URL为 `https://你的域名/pay/alipay/notify`（支付宝）和 `https://你的域名/pay/wechat/notify`（微信），确保域名外网可访问。

---

## 四、默认账号

首次启动自动创建管理员账号：

- **后台管理**：http://localhost:8090/admin/login
  - 用户名：`admin`
  - 密码：`admin`
  - 验证码：`芝麻开门`

- **前台用户注册**：http://localhost:8090/user/reg

---

## 五、支付功能说明

### 新增文件清单

| 文件 | 说明 |
|------|------|
| models/OrderModel.go | 订单数据模型（建表、下单、支付回调、金币入账） |
| helper/pay/pay.go | 支付配置加载 |
| helper/pay/alipay.go | 支付宝 RSA2 签名/验签 + 电脑网站支付 |
| helper/pay/wechat.go | 微信 Native 扫码支付 |
| controllers/HomeControllers/PayController.go | 前台支付控制器 |
| controllers/AdminControllers/OrderController.go | 后台订单管理 |
| views/Home/default/Pay/recharge.html | 充值页面 |
| views/Home/default/Pay/wechat_qrcode.html | 微信扫码页 |
| views/Home/default/Pay/result.html | 支付结果页 |
| views/Admin/default/Order/list.html | 后台订单列表 |
| static/Common/js/qrcode.min.js | QR码生成库 |

### 支付流程

#### 支付宝流程
1. 用户选择金额 → 点击支付宝支付
2. 跳转到支付宝网站完成支付
3. 支付宝同步返回 → /pay/alipay/return
4. 支付宝异步通知 → /pay/alipay/notify（验签 + 金币入账）

#### 微信支付流程
1. 用户选择金额 → 点击微信支付
2. 后端调用统一下单 → 返回二维码页面
3. 用户扫码支付
4. 微信异步通知 → /pay/wechat/notify（验签 + 金币入账）
5. 前端轮询订单状态 → 显示结果页

### 订单表结构 (hc_order)

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | int | 主键 |
| OrderNo | varchar(32) | 订单号 |
| Uid | int | 用户ID |
| Amount | int | 金额（分） |
| Coin | int | 金币数 |
| PayMethod | varchar(10) | 支付方式 alipay/wechat |
| Status | tinyint | 0=待支付 1=已支付 |
| TradeNo | varchar(64) | 第三方交易号 |
| TimeCreate | int | 创建时间 |
| TimePay | int | 支付时间 |

---

## 六、常见问题

### Q: 启动后访问 8090 端口无响应？

检查防火墙是否放行端口。Docker 部署检查 `docker-compose ps` 确认服务状态。

### Q: 数据库连接失败？

确认 MySQL 已启动且账号密码正确。Docker 部署中应用会等待 MySQL 就绪后自动启动。

### Q: 支付宝/微信支付回调收不到？

确保 `sitedomain` 配置为外网可访问的 HTTPS 域名，且回调路径（/pay/alipay/notify、/pay/wechat/notify）可被外部访问。

### Q: 如何切换测试模式？

修改 `app.conf` 中 `[pay] enable = false`，或 Docker 环境变量 `PAY_ENABLE=false`，重启服务即可。

### Q: 金币体系说明？

- 注册赠送金币（默认 10）
- 签到获得金币
- 上传文档获得金币
- 下载文档消耗金币
- 充值获得金币（1元 = 10金币，可配置）
