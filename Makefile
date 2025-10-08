.PHONY: help test test-verbose test-coverage test-race test-bench test-table test-compaction test-btree test-memtable test-sstable test-wal test-version test-schema test-index test-database fmt fmt-check vet tidy verify clean build run-webui install-webui

# 默认目标
.DEFAULT_GOAL := help

# 颜色输出
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
BLUE   := $(shell tput -Txterm setaf 4)
RESET  := $(shell tput -Txterm sgr0)

help: ## 显示帮助信息
	@echo '$(BLUE)SRDB Makefile 命令:$(RESET)'
	@echo ''
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-18s$(RESET) %s\n", $$1, $$2}'
	@echo ''

test: ## 运行所有测试
	@echo "$(GREEN)运行测试...$(RESET)"
	@go test $$(go list ./... | grep -v /examples/)
	@echo "$(GREEN)✓ 测试完成$(RESET)"

test-verbose: ## 运行测试（详细输出）
	@echo "$(GREEN)运行测试（详细模式）...$(RESET)"
	@go test -v $$(go list ./... | grep -v /examples/)

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "$(GREEN)运行测试并生成覆盖率报告...$(RESET)"
	@go test -v -coverprofile=coverage.out $$(go list ./... | grep -v /examples/)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ 覆盖率报告已生成: coverage.html$(RESET)"

test-race: ## 运行测试（启用竞态检测）
	@echo "$(GREEN)运行测试（竞态检测）...$(RESET)"
	@go test -race $$(go list ./... | grep -v /examples/)
	@echo "$(GREEN)✓ 竞态检测完成$(RESET)"

test-bench: ## 运行基准测试
	@echo "$(GREEN)运行基准测试...$(RESET)"
	@go test -bench=. -benchmem $$(go list ./... | grep -v /examples/)

test-table: ## 只运行 table 测试
	@echo "$(GREEN)运行 table 测试...$(RESET)"
	@go test -v -run TestTable

test-compaction: ## 只运行 compaction 测试
	@echo "$(GREEN)运行 compaction 测试...$(RESET)"
	@go test -v -run TestCompaction

test-btree: ## 只运行 btree 测试
	@echo "$(GREEN)运行 btree 测试...$(RESET)"
	@go test -v -run TestBTree

test-memtable: ## 只运行 memtable 测试
	@echo "$(GREEN)运行 memtable 测试...$(RESET)"
	@go test -v -run TestMemTable

test-sstable: ## 只运行 sstable 测试
	@echo "$(GREEN)运行 sstable 测试...$(RESET)"
	@go test -v -run TestSST

test-wal: ## 只运行 wal 测试
	@echo "$(GREEN)运行 wal 测试...$(RESET)"
	@go test -v -run TestWAL

test-version: ## 只运行 version 测试
	@echo "$(GREEN)运行 version 测试...$(RESET)"
	@go test -v -run TestVersion

test-schema: ## 只运行 schema 测试
	@echo "$(GREEN)运行 schema 测试...$(RESET)"
	@go test -v -run TestSchema

test-index: ## 只运行 index 测试
	@echo "$(GREEN)运行 index 测试...$(RESET)"
	@go test -v -run TestIndex

test-database: ## 只运行 database 测试
	@echo "$(GREEN)运行 database 测试...$(RESET)"
	@go test -v -run TestDatabase

fmt: ## 格式化代码
	@echo "$(GREEN)格式化代码...$(RESET)"
	@go fmt ./...
	@echo "$(GREEN)✓ 代码格式化完成$(RESET)"

fmt-check: ## 检查代码格式（不修改）
	@echo "$(GREEN)检查代码格式...$(RESET)"
	@test -z "$$(gofmt -l .)" || (echo "$(YELLOW)以下文件需要格式化:$(RESET)" && gofmt -l . && exit 1)
	@echo "$(GREEN)✓ 代码格式正确$(RESET)"

vet: ## 运行 go vet 静态分析
	@echo "$(GREEN)运行 go vet...$(RESET)"
	@go vet $$(go list ./... | grep -v /examples/)
	@echo "$(GREEN)✓ 静态分析完成$(RESET)"

tidy: ## 整理依赖
	@echo "$(GREEN)整理依赖...$(RESET)"
	@go mod tidy
	@echo "$(GREEN)✓ 依赖整理完成$(RESET)"

verify: ## 验证依赖
	@echo "$(GREEN)验证依赖...$(RESET)"
	@go mod verify
	@echo "$(GREEN)✓ 依赖验证完成$(RESET)"

build: ## 构建 webui 示例程序
	@echo "$(GREEN)构建 webui 示例...$(RESET)"
	@cd examples/webui && go build -o srdb-webui main.go
	@echo "$(GREEN)✓ 构建完成: examples/webui/srdb-webui$(RESET)"

install-webui: ## 安装 webui 工具到 $GOPATH/bin
	@echo "$(GREEN)安装 webui 工具...$(RESET)"
	@cd examples/webui && go install
	@echo "$(GREEN)✓ 已安装到 $(shell go env GOPATH)/bin/webui$(RESET)"

run-webui: ## 运行 webui 示例（默认端口 8080）
	@echo "$(GREEN)启动 webui 服务...$(RESET)"
	@cd examples/webui && go run main.go webui -db ./data -addr :8080

clean: ## 清理测试文件和构建产物
	@echo "$(GREEN)清理测试文件...$(RESET)"
	@rm -f coverage.out coverage.html
	@rm -f examples/webui/srdb-webui
	@find . -type d -name "mydb*" -exec rm -rf {} + 2>/dev/null || true
	@find . -type d -name "testdb*" -exec rm -rf {} + 2>/dev/null || true
	@find . -type d -name "data" -exec rm -rf {} + 2>/dev/null || true
	@echo "$(GREEN)✓ 清理完成$(RESET)"
