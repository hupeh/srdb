# SRDB WebUI - 数据库管理工具

一个功能强大的 SRDB 数据库管理工具，集成了现代化的 Web 界面和实用的命令行工具。

## 📋 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [Web UI 使用指南](#web-ui-使用指南)
- [命令行工具](#命令行工具)
- [技术架构](#技术架构)
- [开发说明](#开发说明)

---

## 🎯 功能特性

### Web UI

#### 📊 数据管理
- **表列表** - 查看所有表及其 Schema 信息
- **数据浏览** - 分页浏览表数据，支持自定义列选择
- **列持久化** - 自动保存列选择偏好到 localStorage
- **数据详情** - 点击查看完整的行数据（JSON 格式）
- **智能截断** - 长字符串自动截断，点击查看完整内容
- **时间格式化** - 自动格式化 `_time` 字段为可读时间

#### 🌳 LSM-Tree 管理
- **Manifest 视图** - 可视化 LSM-Tree 层级结构
- **文件详情** - 查看每层的 SST 文件信息
- **Compaction 监控** - 实时查看 Compaction Score 和统计
- **层级折叠** - 可展开/收起查看文件详情

#### 🎨 用户体验
- **响应式设计** - 完美适配桌面和移动设备
- **深色/浅色主题** - 支持主题切换
- **实时刷新** - 一键刷新当前视图数据
- **移动端优化** - 侧边栏抽屉式导航

### 命令行工具

提供多个实用的数据库诊断和管理工具：

| 命令 | 功能 | 说明 |
|------|------|------|
| `serve` | Web UI 服务器 | 启动 Web 管理界面 |
| `check-data` | 数据检查 | 检查表数据完整性 |
| `check-seq` | 序列号检查 | 验证特定序列号的数据 |
| `dump-manifest` | Manifest 导出 | 导出 LSM-Tree 结构信息 |
| `inspect-sst` | SST 文件检查 | 检查单个 SST 文件 |
| `inspect-all-sst` | 批量 SST 检查 | 检查所有 SST 文件 |
| `test-fix` | 修复测试 | 测试数据修复功能 |
| `test-keys` | 键测试 | 测试键的存在性 |

---

## 🚀 快速开始

### 1. 启动 Web UI

```bash
cd examples/webui

# 使用默认配置（数据库：./data，端口：8080）
go run main.go serve

# 自定义配置
go run main.go serve --db /path/to/database --port 3000

# 启用自动数据插入（用于演示）
go run main.go serve --auto-insert
```

### 2. 访问 Web UI

打开浏览器访问：http://localhost:8080

### 3. 命令行工具示例

```bash
# 检查表数据
go run main.go check-data --db ./data --table users

# 检查特定序列号
go run main.go check-seq --db ./data --table users --seq 123

# 导出 Manifest
go run main.go dump-manifest --db ./data --table users

# 检查 SST 文件
go run main.go inspect-sst --db ./data --table users --file 000001.sst
```

---

## 📖 Web UI 使用指南

### 界面布局

```
┌─────────────────────────────────────────────────┐
│  SRDB Tables                          [🌙/☀️]  │  ← 侧边栏
│  ├─ users                                       │
│  ├─ orders                                      │
│  └─ logs                                        │
├─────────────────────────────────────────────────┤
│  [☰] users                        [🔄 Refresh] │  ← 页头
│  [Data] [Manifest / LSM-Tree]                  │  ← 视图切换
├─────────────────────────────────────────────────┤
│                                                 │
│  Schema (点击字段卡片选择要显示的列)            │  ← Schema 区域
│  ┌──────────┬──────────┬──────────┐            │
│  │⚡ id     │● name    │⚡ email   │            │
│  │[int64]   │[string]  │[string]  │            │
│  └──────────┴──────────┴──────────┘            │
│                                                 │
│  Data (1,234 rows)                              │  ← 数据表格
│  ┌─────┬──────┬───────────┬─────────┐          │
│  │ _seq│ name │ email     │ Actions │          │
│  ├─────┼──────┼───────────┼─────────┤          │
│  │  1  │ John │ john@...  │ Detail  │          │
│  │  2  │ Jane │ jane@...  │ Detail  │          │
│  └─────┴──────┴───────────┴─────────┘          │
│                                                 │
├─────────────────────────────────────────────────┤
│  [10/page] [Previous] Page 1 of 5 [Go] [Next] │  ← 分页控件
└─────────────────────────────────────────────────┘
```

### 功能说明

#### 1. 表列表（侧边栏）
- 显示所有表及其字段信息
- 点击表名切换到该表
- 展开/收起查看字段详情
- 字段图标：⚡ = 已索引，● = 未索引

#### 2. Data 视图
- **Schema 区域**：点击字段卡片选择要显示的列
- **数据表格**：显示选中的列数据
- **系统字段**：
  - `_seq`：序列号（第一列）
  - `_time`：时间戳（倒数第二列，自动格式化）
- **Detail 按钮**：查看完整的行数据（JSON 格式）
- **分页控件**：
  - 每页大小：10/20/50/100
  - 上一页/下一页
  - 跳转到指定页

#### 3. Manifest 视图
- **统计卡片**：
  - Active Levels：活跃层数
  - Total Files：总文件数
  - Total Size：总大小
  - Compactions：Compaction 次数
- **层级卡片**：
  - 点击展开/收起查看文件列表
  - Score 指示器：
    - 🟢 绿色：健康（< 50%）
    - 🟡 黄色：警告（50-80%）
    - 🔴 红色：需要 Compaction（≥ 80%）
- **文件详情**：
  - 文件编号、大小、行数
  - Seq 范围（min_key - max_key）

#### 4. 刷新按钮
- 点击刷新当前视图的数据
- Data 视图：重新加载表数据
- Manifest 视图：重新加载 LSM-Tree 结构

#### 5. 主题切换
- 点击右上角的 🌙/☀️ 图标
- 切换深色/浅色主题
- 自动保存到 localStorage

---

## 🛠️ 命令行工具

### serve - Web UI 服务器

启动 Web 管理界面。

```bash
go run main.go serve [flags]
```

**参数**：
- `--db` - 数据库目录（默认：`./data`）
- `--port` - 服务端口（默认：`8080`）
- `--auto-insert` - 启用自动数据插入（用于演示）

**示例**：
```bash
# 基本使用
go run main.go serve

# 自定义端口
go run main.go serve --port 3000

# 启用自动插入（每秒插入一条随机数据到 logs 表）
go run main.go serve --auto-insert
```

### check-data - 数据检查

检查表数据的完整性。

```bash
go run main.go check-data --db ./data --table <table_name>
```

### check-seq - 序列号检查

验证特定序列号的数据。

```bash
go run main.go check-seq --db ./data --table <table_name> --seq <sequence_number>
```

### dump-manifest - Manifest 导出

导出 LSM-Tree 层级结构信息。

```bash
go run main.go dump-manifest --db ./data --table <table_name>
```

### inspect-sst - SST 文件检查

检查单个 SST 文件的内容和元数据。

```bash
go run main.go inspect-sst --db ./data --table <table_name> --file <file_name>
```

### inspect-all-sst - 批量 SST 检查

检查表的所有 SST 文件。

```bash
go run main.go inspect-all-sst --db ./data --table <table_name>
```

---

## 🏗️ 技术架构

### 前端技术栈

- **Lit** - 轻量级 Web Components 框架
- **ES Modules** - 原生 JavaScript 模块
- **CSS Variables** - 主题系统
- **Shadow DOM** - 组件封装

### 组件架构

```
srdb-app (主应用)
├── srdb-theme-toggle (主题切换)
├── srdb-table-list (表列表)
├── srdb-page-header (页头)
│   └── [🔄 Refresh] (刷新按钮)
├── srdb-table-view (表视图容器)
│   ├── srdb-data-view (数据视图)
│   │   ├── Schema 区域
│   │   │   ├── srdb-field-icon (字段图标)
│   │   │   └── srdb-badge (类型标签)
│   │   └── 数据表格
│   └── srdb-manifest-view (Manifest 视图)
│       └── srdb-badge (Score 标签)
└── srdb-modal-dialog (模态对话框)
```

### 后端架构

```
webui.go
├── API Endpoints
│   ├── GET /api/tables - 获取表列表
│   ├── GET /api/tables/{name}/schema - 获取表 Schema
│   ├── GET /api/tables/{name}/data - 获取表数据（分页）
│   ├── GET /api/tables/{name}/data/{seq} - 获取单条数据
│   └── GET /api/tables/{name}/manifest - 获取 Manifest 信息
├── Static Files
│   └── /static/* - 静态资源服务
└── Index
    └── / - 首页
```

### 数据流

```
用户操作
  ↓
组件事件 (CustomEvent)
  ↓
app.js (事件总线)
  ↓
API 请求 (fetch)
  ↓
webui.go (HTTP Handler)
  ↓
SRDB Database
  ↓
JSON 响应
  ↓
组件更新 (Lit reactive)
  ↓
UI 渲染
```

---

## 🔧 开发说明

### 项目结构

```
webui/
├── commands/
│   └── webui.go              # Web UI 服务器实现
├── static/
│   ├── index.html            # 主页面
│   ├── css/
│   │   └── styles.css        # 全局样式
│   └── js/
│       ├── app.js            # 应用入口和事件总线
│       ├── components/       # Web Components
│       │   ├── app.js        # 主应用容器
│       │   ├── badge.js      # 标签组件
│       │   ├── data-view.js  # 数据视图
│       │   ├── field-icon.js # 字段图标
│       │   ├── manifest-view.js # Manifest 视图
│       │   ├── modal-dialog.js  # 模态对话框
│       │   ├── page-header.js   # 页头
│       │   ├── table-list.js    # 表列表
│       │   ├── table-view.js    # 表视图容器
│       │   └── theme-toggle.js  # 主题切换
│       └── styles/
│           └── shared-styles.js # 共享样式
└── webui.go                  # Web UI 后端实现
```

### 添加新组件

1. 在 `static/js/components/` 创建组件文件
2. 继承 `LitElement`
3. 定义 `static properties` 和 `static styles`
4. 实现 `render()` 方法
5. 使用 `customElements.define('srdb-xxx', Component)` 注册
6. 在 `app.js` 中导入

### 添加新 API

1. 在 `webui.go` 中添加 handler 方法
2. 在 `setupHandler()` 中注册路由
3. 返回 JSON 格式的响应
4. 在前端组件中调用 API

### 样式规范

- 使用 CSS Variables 定义颜色和尺寸
- 组件样式封装在 Shadow DOM 中
- 共享样式定义在 `shared-styles.js`
- 响应式断点：768px

### 命名规范

- **组件名**：`srdb-xxx`（kebab-case）
- **类名**：`ComponentName`（PascalCase）
- **文件名**：`component-name.js`（kebab-case）
- **CSS 类**：`.class-name`（kebab-case）

---

## 📝 注意事项

### 性能优化

1. **列选择**：只加载选中的列，减少数据传输
2. **字符串截断**：长字符串自动截断，按需加载完整内容
3. **分页加载**：大表数据分页加载，避免一次性加载全部
4. **Shadow DOM**：组件样式隔离，避免全局样式污染

### 浏览器兼容性

- Chrome 90+
- Firefox 88+
- Safari 14+
- Edge 90+

需要支持：
- ES Modules
- Web Components
- Shadow DOM
- CSS Variables

### 已知限制

1. **大数据量**：单页最多显示 1000 条数据
2. **字符串长度**：超过 100 字符自动截断
3. **并发限制**：同时只能查看一个表的数据

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License
