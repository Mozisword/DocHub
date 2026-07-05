#!/bin/sh
# DocHub Docker 入口脚本
# 等待 MySQL 就绪后启动应用

set -e

echo "=== DocHub Docker 启动 ==="

# 等待 MySQL 就绪
if [ -n "${DB_HOST}" ]; then
    echo "等待 MySQL (${DB_HOST}:${DB_PORT:-3306}) 就绪..."
    MAX_WAIT=60
    WAITED=0
    while ! nc -z "${DB_HOST}" "${DB_PORT:-3306}" 2>/dev/null; do
        WAITED=$((WAITED + 1))
        if [ ${WAITED} -ge ${MAX_WAIT} ]; then
            echo "错误: 等待 MySQL 超时（${MAX_WAIT}秒），请检查数据库服务"
            exit 1
        fi
        echo "  等待中... (${WAITED}/${MAX_WAIT})"
        sleep 2
    done
    echo "MySQL 已就绪"
fi

# 生成 app.conf（如果不存在或需要根据环境变量更新）
if [ ! -f /app/conf/app.conf ] || [ "${OVERWRITE_CONFIG}" = "true" ]; then
    echo "生成 app.conf 配置文件..."
    cat > /app/conf/app.conf <<EOF
appname = DocHub
httpport = 8090
runmode = ${RUNMODE:-dev}
EnableGzip = true
enablexsrf = true
xsrfkey = 8fd70cd11c371951b1f385f452784fbb
xsrfexpire = 3600
CookieSecret = ${COOKIE_SECRET:-jLGy7yFD69dYItVJ}
StaticExt = .txt,.html,.ico,.jpeg,.png,.gif,.xml

sessionon = true
SessionProvider = file
SessionProviderConfig = cache/session
SessionName = dochub

[db]
host = ${DB_HOST:-localhost}
port = ${DB_PORT:-3306}
user = ${DB_USER:-root}
password = ${DB_PASSWORD:-}
database = ${DB_NAME:-dochub}
prefix = hc_
charset = utf8
maxIdle = 50
maxConn = 300

[pay]
enable = ${PAY_ENABLE:-false}
coinrate = ${PAY_COINRATE:-10}
sitedomain = ${PAY_SITEDOMAIN:-}

[alipay]
enable = ${ALIPAY_ENABLE:-false}
appid = ${ALIPAY_APPID:-}
privatekey = ${ALIPAY_PRIVATEKEY:-}
publickey = ${ALIPAY_PUBLICKEY:-}
gateway = ${ALIPAY_GATEWAY:-https://openapi.alipay.com/gateway.do}

[wechatpay]
enable = ${WECHAT_ENABLE:-false}
appid = ${WECHAT_APPID:-}
mchid = ${WECHAT_MCHID:-}
apikey = ${WECHAT_APIKEY:-}
EOF
    echo "app.conf 生成完成"
fi

echo "启动 DocHub 服务..."
exec ./DocHub
