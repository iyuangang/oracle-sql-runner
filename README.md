# Oracle SQL Runner

[![Build](https://github.com/iyuangang/oracle-sql-runner/actions/workflows/build.yml/badge.svg)](https://github.com/iyuangang/oracle-sql-runner/actions/workflows/build.yml)
[![codecov](https://codecov.io/github/iyuangang/oracle-sql-runner/branch/dev/graph/badge.svg?token=XZOV0PQA4N)](https://codecov.io/github/iyuangang/oracle-sql-runner)
[![Release](https://img.shields.io/github/v/release/iyuangang/oracle-sql-runner)](https://github.com/iyuangang/oracle-sql-runner/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/iyuangang/oracle-sql-runner)](https://goreportcard.com/report/github.com/iyuangang/oracle-sql-runner)
[![License](https://img.shields.io/github/license/iyuangang/oracle-sql-runner)](LICENSE)

Oracle SQL Runner 是一个高性能的 Oracle SQL 脚本执行工具，支持并行执行、错误重试、PL/SQL 块等特性。

## 特性

- 支持并行执行 SQL 语句
- 自动识别和处理 PL/SQL 块
- 智能错误重试机制
- 详细的执行日志和性能指标
- 支持查询结果显示
- 跨平台支持 (Linux, Windows, macOS)

## 安装

### 从二进制安装

从 [Releases](https://github.com/iyuangang/oracle-sql-runner/releases) 页面下载对应平台的二进制文件。

### 从源码构建

需要 Go 1.22 或更高版本：

```bash
git clone https://github.com/iyuangang/oracle-sql-runner.git
cd oracle-sql-runner
go build ./cmd/sql-runner
```

## 配置

创建 `config.json` 配置文件：

```json
{
  "databases": {
    "prod": {
      "name": "生产环境",
      "user": "your_username",
      "password": "your_password",
      "host": "localhost",
      "port": 1521,
      "service": "ORCLPDB1",
      "max_connections": 5,
      "idle_timeout": 300
    }
  },
  "max_retries": 3,
  "max_concurrent": 5,
  "batch_size": 1000,
  "timeout": 30,
  "log_level": "info",
  "log_file": "logs/sql-runner.log"
}
```

### 配置说明

- `databases`: 数据库配置列表
  - `name`: 数据库描述
  - `user`: 用户名
  - `password`: 密码
  - `host`: 主机地址
  - `port`: 端口号
  - `service`: 服务名
  - `max_connections`: 最大连接数
  - `idle_timeout`: 空闲超时时间(秒)
- `max_retries`: 最大重试次数
- `max_concurrent`: 最大并发执行数
- `batch_size`: 批处理大小
- `timeout`: SQL 执行超时时间(秒)
- `log_level`: 日志级别 (debug/info/warn/error)
- `log_file`: 日志文件路径

## 使用方法

### 基本用法

```bash
sql-runner -f script.sql -d prod
```

### 命令行参数

```bash
Usage:
  sql-runner [flags]

Flags:
  -c, --config string    配置文件路径 (默认 "config.json")
  -d, --database string  数据库名称
  -f, --file string      SQL文件路径
  -h, --help            帮助信息
  -v, --verbose         显示详细信息
      --version         版本信息
```

### SQL 文件格式

支持三种类型的 SQL 语句：

1. 普通查询
```sql
SELECT * FROM employees;
```

2. DML/DDL 语句
```sql
CREATE TABLE test_table (
    id NUMBER PRIMARY KEY,
    name VARCHAR2(100)
);
```

3. PL/SQL 块
```sql
CREATE OR REPLACE PROCEDURE test_proc AS
BEGIN
    DBMS_OUTPUT.PUT_LINE('Hello World');
END;
/
```

## 日志输出

日志以 JSON 格式输出，包含详细的执行信息：

```json
{
    "time": "2024-11-26T09:46:38.7720375+08:00",
    "level": "INFO",
    "source": {
        "function": "core.(*Executor).ExecuteFile",
        "file": "internal/core/executor.go",
        "line": 123
    },
    "msg": "SQL文件执行完成",
    "success": 33,
    "failed": 1,
    "duration": 3827597300
}
```

## 开发

### 运行测试

```bash
# 运行单元测试
go test ./...

# 运行集成测试
go test -tags=integration ./...
```

### 构建发布版本

```bash
make release
```

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE) 文件。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 鸣谢

- [godror](https://github.com/godror/godror) 提供了 Oracle 数据库的 Go 驱动
- [cobra](https://github.com/spf13/cobra) 提供了命令行解析库
- [slog](https://github.com/slog/slog) 提供了高性能的日志库
