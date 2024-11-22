# Oracle SQL Runner

[![Build Status](https://github.com/iyuangang/oracle-sql-runner/workflows/Build/badge.svg)](https://github.com/iyuangang/oracle-sql-runner/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/iyuangang/oracle-sql-runner)](https://goreportcard.com/report/github.com/iyuangang/oracle-sql-runner)
[![License](https://img.shields.io/github/license/iyuangang/oracle-sql-runner)](LICENSE)

一个强大的Oracle SQL脚本执行工具，支持并行执行、多数据库环境管理、事务控制等特性。

## 特性

- 🚀 支持并行执行SQL语句
- 🔒 安全的密码加密存储
- 📝 支持所有Oracle SQL类型
  - DDL (CREATE, ALTER, DROP等)
  - DML (INSERT, UPDATE, DELETE等)
  - DCL (GRANT, REVOKE等)
  - PL/SQL块和匿名块
- 🔄 自动重试机制
- 📊 详细的执行统计
- 🎯 进度显示
- 🌐 多数据库环境配置
- 💾 连接池管理

## 安装

### 从源码编译

```bash
# 克隆仓库
git clone https://github.com/iyuangang/oracle-sql-runner.git
cd oracle-sql-runner

# 安装依赖
go mod download

# 编译
go build -o sql-runner .\cmd\sql-runner\
```

### 下载预编译版本

访问 [Releases](https://github.com/iyuangang/oracle-sql-runner/releases) 页面下载适合您系统的版本。

## 快速开始

1. 创建配置文件 `config.json`:

```json
{
  "databases": {
    "dev": {
      "name": "开发环境",
      "user": "dev_user",
      "password": "encrypted_password",
      "host": "dev-oracle.example.com",
      "port": 1521,
      "service": "DEV"
    }
  }
}
```

2. 执行SQL文件:

```bash
# 基本用法
./sql-runner -d dev -f script.sql

# 指定配置文件
./sql-runner -c custom-config.json -d prod -f script.sql

# 并行执行
./sql-runner -d dev -f script.sql -p 4

# 显示详细输出
./sql-runner -d dev -f script.sql -v
```

## 配置说明

### 数据库配置

```json
{
  "name": "数据库名称",
  "user": "用户名",
  "password": "密码",
  "host": "主机地址",
  "port": 1521,
  "service": "服务名",
  "auto_commit": true,
  "max_retries": 3,
  "timeout_seconds": 30,
  "enable_dbms_output": true,
  "max_connections": 10
}
```

### 执行配置

```json
{
  "parallel_degree": 4,
  "batch_size": 1000,
  "max_file_size": 104857600,
  "retry_interval_seconds": 5
}
```

## 命令行参数

```
Usage:
  sql-runner [flags]
  sql-runner [command]

Available Commands:
  help            帮助信息
  version         显示版本信息
  test-connection 测试数据库连接

Flags:
  -c, --config string    配置文件路径 (默认 "config.json")
  -d, --database string  数据库名称
  -f, --file string     SQL文件路径
  -p, --parallel int    并行度 (默认 1)
  -v, --verbose        显示详细输出
      --no-progress    不显示进度条
      --validate       仅验证SQL语法
  -o, --output string  输出格式 (text/json)
  -h, --help          帮助信息
```

## 开发

### 运行测试

```bash
go test ./... -v
```

### 构建发布版本

```bash
make release
```

## 贡献

欢迎提交 Pull Request 和 Issue！

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件
