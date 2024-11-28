# 检测操作系统
ifeq ($(OS),Windows_NT)
    # Windows 命令
    RM = if exist "$(BUILD_DIR)" rmdir /s /q "$(BUILD_DIR)"
    MKDIR = if not exist "$(BUILD_DIR)" mkdir "$(BUILD_DIR)"
    RMFILE = del /f /q
    SEP = \\
    EXE = .exe
    # Windows 下获取时间
    BUILD_TIME = $(Get-Date -Format "yyyy-MM-ddTHH:mm:ss")
else
    # Unix-like 命令
    RM = rm -rf $(BUILD_DIR)
    MKDIR = mkdir -p $(BUILD_DIR)
    RMFILE = rm -f
    SEP = /
    EXE =
    # Unix 下获取时间
    BUILD_TIME = $(date -u '+%Y-%m-%dT%H:%M:%S')
endif

# 基本变量
BINARY_NAME = sql-runner
MAIN_PACKAGE = ./cmd/sql-runner
BUILD_DIR = build

# Git 信息获取（Windows 和 Unix 通用）
VERSION = $(git describe --tags --always --dirty 2>nul || git describe --tags --always --dirty 2>/dev/null || echo unknown)
COMMIT = $(git rev-parse --short HEAD 2>nul || git rev-parse --short HEAD 2>/dev/null || echo unknown)

# Go 编译标志
LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"
GOFLAGS = -trimpath

# 操作系统和架构
PLATFORMS = linux windows darwin
ARCHITECTURES = amd64 arm64

# 清理
.PHONY: clean
clean:
	$(RM)

# 创建构建目录
.PHONY: init
init:
	$(MKDIR)

# 构建当前平台的二进制文件
.PHONY: build
build: clean init
	go build $(GOFLAGS) $(LDFLAGS) -o "$(BUILD_DIR)$(SEP)$(BINARY_NAME)$(EXE)" $(MAIN_PACKAGE)

# 构建所有平台的二进制文件
.PHONY: build-all
build-all: clean init
	$(foreach PLATFORM,$(PLATFORMS),\
		$(foreach ARCH,$(ARCHITECTURES),\
			GOOS=$(PLATFORM) GOARCH=$(ARCH) go build $(GOFLAGS) $(LDFLAGS) \
				-o "$(BUILD_DIR)$(SEP)$(BINARY_NAME)_$(PLATFORM)_$(ARCH)$(if $(findstring windows,$(PLATFORM)),.exe,)" \
				$(MAIN_PACKAGE); \
		)\
	)

# 运行测试
.PHONY: test
test:
	go test -v -race -cover ./...

# 运行测试并生成覆盖率报告
.PHONY: coverage
coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	$(RMFILE) coverage.out

# 代码检查
.PHONY: lint
lint:
	golangci-lint run

# 生成文档
.PHONY: doc
doc:
	godoc -http=:6060

# 帮助信息
.PHONY: help
help:
	@echo "可用的 make 命令:"
	@echo "  build        - 构建当前平台的二进制文件"
	@echo "  build-all    - 构建所有平台的二进制文件"
	@echo "  test         - 运行测试"
	@echo "  coverage     - 生成测试覆盖率报告"
	@echo "  clean        - 清理构建目录"
	@echo "  lint         - 运行代码检查"
	@echo "  doc          - 启动文档服务器"
