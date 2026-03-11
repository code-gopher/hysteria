# 脚本使用说明

## 简介
这是一个 Bash 脚本，用于安装 Hysteria 2，支持 CentOS、Debian、Ubuntu（systemd）和 Alpine Linux（OpenRC）。脚本接收并处理 `apiHost`、`apiKey` 和 `nodeID` 三个参数，从 GitHub 下载安装文件，配置服务并启动。

**所有参数必须通过命令行传入，且不能为空。**

## 前置要求
- 系统：CentOS、Debian、Ubuntu 或 Alpine Linux。
- 网络：确保系统可以访问 GitHub（`https://github.com`）。
- 权限：需要 root 权限运行脚本。
- **Debian 特定要求**：确保 `apt` 可用，防火墙（如 `ufw`）需开放 Hysteria 的端口。

## 使用方法

**执行脚本**：
使用以下命令运行脚本，并传入参数：
```bash
bash <(curl -fsSL https://raw.githubusercontent.com/code-gopher/hysteria/master/install.sh) --apiHost=https://api.example.com --apiKey=abc123 --nodeID=123
```

## 输出示例
```
使用配置:
apiHost = https://api.example.com
apiKey = abc123
nodeID = 1
检测到系统类型: systemd（或 openrc）
Hysteria2 安装完成并已启动
```

## 注意事项
- **CentOS**：确保 `yum` 可用，防火墙（如 `firewalld`）需开放 Hysteria 的端口。
- **Debian**：确保 `apt` 已更新（`apt update`），防火墙（如 `ufw`）需开放 Hysteria 的端口。
- **Ubuntu**：与 Debian 类似，确保 `apt` 可用，防火墙（如 `ufw`）需配置。
- **Alpine**：确保社区仓库已启用，用于安装 `wget`、`tar` 和 `curl`。
- **参数校验**：`apiHost`、`apiKey` 和 `nodeID` 不能为空，否则脚本会报错退出。
- **服务管理**：
  - CentOS/Debian/Ubuntu：使用 `systemctl status/restart/stop hysteria` 管理服务。
  - Alpine：使用 `rc-service hysteria status/restart/stop` 管理服务。
- **清理**：脚本会自动清理临时文件（`hysteria.tar.gz` 和 `hysteria` 目录）。

## Debian 追加说明
- 在 Debian 系统上，脚本会使用 `apt` 安装必要的依赖（如 `wget`、`tar`、`curl`）。
- 建议运行前执行 `apt update && apt upgrade` 确保系统软件包最新。
- 若使用 `ufw`，需手动开放 Hysteria 使用的端口，例如：
  ```bash
  ufw allow <Hysteria端口>
  ```
