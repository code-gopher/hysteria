#!/bin/sh
set -e

#==============================================================================
# Hysteria2 自动安装脚本
# 功能: 自动检测系统架构和类型，下载并配置 Hysteria2 服务
#==============================================================================

# 全局变量
APIHOST=""
APIKEY=""
NODEID=""
HYSTERIA_ARCH=""
SYSTEM_TYPE=""
DOWNLOAD_URL=""

#------------------------------------------------------------------------------
# 函数: 显示使用说明
#------------------------------------------------------------------------------
show_usage() {
    echo "用法: $0 --apiHost=xxx --apiKey=yyy --nodeID=zzz"
    exit 1
}

#------------------------------------------------------------------------------
# 函数: 解析命令行参数
#------------------------------------------------------------------------------
parse_arguments() {
    for arg in "$@"; do
        case $arg in
            --apiHost=*)
                APIHOST="${arg#*=}"
                ;;
            --apiKey=*)
                APIKEY="${arg#*=}"
                ;;
            --nodeID=*)
                NODEID="${arg#*=}"
                ;;
            *)
                echo "错误: 未知参数 $arg"
                show_usage
                ;;
        esac
    done

    # 参数校验
    if [ -z "$APIHOST" ] || [ -z "$APIKEY" ] || [ -z "$NODEID" ]; then
        echo "错误: 所有参数都不能为空"
        show_usage
    fi

    echo "    apiHost     = ${APIHOST}"
    echo "    apiKey      = ${APIKEY}"
    echo "    nodeID      = ${NODEID}"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 检测系统架构
#------------------------------------------------------------------------------
detect_architecture() {
    echo "==> 检测系统架构..."
    local arch=$(uname -m)
    
    case $arch in
        x86_64|amd64)
            HYSTERIA_ARCH="amd64"
            echo "    检测到: 64 位 x86 架构"
            ;;
        i386|i686|x86)
            HYSTERIA_ARCH="386"
            echo "    检测到: 32 位 x86 架构"
            ;;
        aarch64|arm64)
            HYSTERIA_ARCH="arm64"
            echo "    检测到: 64 位 ARM 架构"
            ;;
        armv7l|armv6l)
            HYSTERIA_ARCH="arm"
            echo "    检测到: 32 位 ARM 架构"
            ;;
        *)
            echo "    错误: 不支持的架构 $arch"
            exit 1
            ;;
    esac
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 检测系统类型
#------------------------------------------------------------------------------
detect_system_type() {
    echo "==> 检测系统类型..."
    
    if command -v systemctl >/dev/null 2>&1; then
        SYSTEM_TYPE="systemd"
        echo "    检测到: systemd"
    elif command -v rc-service >/dev/null 2>&1; then
        SYSTEM_TYPE="openrc"
        echo "    检测到: OpenRC"
    else
        echo "    错误: 不支持的系统（未检测到 systemd 或 OpenRC）"
        exit 1
    fi
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 安装系统依赖
#------------------------------------------------------------------------------
install_dependencies() {
    echo "==> 安装必要工具..."
    
    if [ "$SYSTEM_TYPE" = "openrc" ]; then
        apk add --no-cache wget openssl
    else
        if command -v apt-get >/dev/null 2>&1; then
            apt-get install -y wget openssl
        elif command -v yum >/dev/null 2>&1; then
            yum install -y wget openssl
        else
            echo "    错误: 未找到合适的包管理器 (apt-get 或 yum)"
            exit 1
        fi
    fi
    echo "    依赖安装完成"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 下载 Hysteria 二进制文件
#------------------------------------------------------------------------------
download_hysteria_binary() {
    DOWNLOAD_URL="https://github.com/code-gopher/hysteria/releases/latest/download/hysteria-linux-${HYSTERIA_ARCH}"
    
    echo "==> 下载 Hysteria 最新版本..."
    echo "    URL: ${DOWNLOAD_URL}"
    
    rm -f /tmp/hysteria-download
    # 使用 -L 选项处理 GitHub 的重定向
    if ! wget -L -O /tmp/hysteria-download "$DOWNLOAD_URL"; then
        echo "    错误: 下载失败 (请检查网络或 GitHub 访问情况)"
        exit 1
    fi
    
    echo "    下载完成"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 安装二进制文件
#------------------------------------------------------------------------------
install_binary() {
    echo "==> 安装 Hysteria 二进制文件..."
    
    cp -f /tmp/hysteria-download /usr/local/bin/hysteria
    chmod +x /usr/local/bin/hysteria
    rm -f /tmp/hysteria-download
    
    echo "    安装完成: /usr/local/bin/hysteria"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 生成 SSL 证书
#------------------------------------------------------------------------------
generate_certificates() {
    echo "==> 准备 SSL 证书..."
    
    mkdir -p /etc/hysteria
    
    if [ ! -f /etc/hysteria/server.crt ] || [ ! -f /etc/hysteria/server.key ]; then
        echo "    生成自签名证书..."
        openssl req -x509 -nodes -newkey rsa:2048 \
            -keyout /etc/hysteria/server.key \
            -out /etc/hysteria/server.crt \
            -days 3650 \
            -subj "/C=US/ST=State/L=City/O=Organization/CN=hysteria.local"
        echo "    证书生成完成（有效期 10 年）"
    else
        echo "    证书已存在，跳过生成"
    fi
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 创建配置文件
#------------------------------------------------------------------------------
create_config_file() {
    echo "==> 创建配置文件..."
    
    cat > /etc/hysteria/config.yaml <<EOF
v2board:
  apiHost: ${APIHOST}
  apiKey: ${APIKEY}
  nodeID: ${NODEID}
tls:
  type: tls
  cert: /etc/hysteria/server.crt
  key: /etc/hysteria/server.key
auth:
  type: v2board
trafficStats:
  listen: 127.0.0.1:7653
acl:
  inline: 
    - reject(10.0.0.0/8)
    - reject(172.16.0.0/12)
    - reject(192.168.0.0/16)
    - reject(127.0.0.0/8)
    - reject(fc00::/7)
masquerade:
  type: proxy
  proxy:
    url: https://mirrors.ustc.edu.cn/
    rewriteHost: true
quic:
  initStreamReceiveWindow: 262144
  maxStreamReceiveWindow: 1048576
  initConnReceiveWindow: 524288
  maxConnReceiveWindow: 4194304
  maxIdleTimeout: 30s
  keepAlivePeriod: 10s
EOF
    
    echo "    配置文件已创建: /etc/hysteria/config.yaml"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 配置 systemd 服务
#------------------------------------------------------------------------------
configure_systemd_service() {
    echo "==> 配置 systemd 服务..."
    
    cat > /etc/systemd/system/hysteria.service <<EOF
[Unit]
Description=Hysteria2 Server
After=network.target

[Service]
Type=simple
Environment=GOGC=20 GOMAXPROCS=1
ExecStart=/usr/local/bin/hysteria -c /etc/hysteria/config.yaml server
Restart=always
RestartSec=5
MemoryMax=100M
MemoryHigh=90M

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable hysteria
    systemctl restart hysteria
    
    echo "    systemd 服务配置完成并已启动"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 配置 OpenRC 服务
#------------------------------------------------------------------------------
configure_openrc_service() {
    echo "==> 配置 OpenRC 服务..."
    
    cat > /etc/init.d/hysteria <<EOF
#!/sbin/openrc-run

name="hysteria"
description="Hysteria2 Server"
export GOGC=20
export GOMAXPROCS=1
rc_ulimit="-m 102400"
command="/usr/local/bin/hysteria"
command_args="-c /etc/hysteria/config.yaml server"
pidfile="/var/run/hysteria.pid"
command_background="yes"
respawn_delay=5
respawn_max=0

depend() {
    need net
}

EOF

    chmod +x /etc/init.d/hysteria
    rc-update add hysteria default
    rc-service hysteria restart
    
    echo "    OpenRC 服务配置完成并已启动"
    echo ""
}

#------------------------------------------------------------------------------
# 函数: 配置系统服务
#------------------------------------------------------------------------------
configure_service() {
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        configure_systemd_service
    else
        configure_openrc_service
    fi
}

#------------------------------------------------------------------------------
# 函数: 显示安装完成信息
#------------------------------------------------------------------------------
show_completion_message() {
    echo "=========================================="
    echo "✓ Hysteria2 安装完成！"
    echo "=========================================="
    echo ""
    echo "服务状态检查命令:"
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        echo "  systemctl status hysteria"
    else
        echo "  rc-service hysteria status"
    fi
    echo ""
}

#------------------------------------------------------------------------------
# 主函数
#------------------------------------------------------------------------------
main() {
    echo ""
    echo "=========================================="
    echo "  Hysteria2 自动安装脚本"
    echo "=========================================="
    echo ""
    
    parse_arguments "$@"
    detect_architecture
    detect_system_type
    install_dependencies
    download_hysteria_binary
    install_binary
    generate_certificates
    create_config_file
    configure_service
    show_completion_message
}

# 脚本入口
main "$@"
