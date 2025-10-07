.PHONY: help test test-verbose test-coverage test-race test-bench test-engine test-compaction test-query fmt fmt-check vet tidy verify clean

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

test-engine: ## 只运行 engine 包的测试
	@echo "$(GREEN)运行 engine 测试...$(RESET)"
	@go test -v ./engine

test-compaction: ## 只运行 compaction 包的测试
	@echo "$(GREEN)运行 compaction 测试...$(RESET)"
	@go test -v ./compaction

test-query: ## 只运行 query 包的测试
	@echo "$(GREEN)运行 query 测试...$(RESET)"
	@go test -v ./query

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

clean: ## 清理测试文件
	@echo "$(GREEN)清理测试文件...$(RESET)"
	@rm -f coverage.out coverage.html
	@find . -type d -name "mydb*" -exec rm -rf {} + 2>/dev/null || true
	@find . -type d -name "testdb*" -exec rm -rf {} + 2>/dev/null || true
	@echo "$(GREEN)✓ 清理完成$(RESET)"
