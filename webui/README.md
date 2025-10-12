# SRDB WebUI

基于 Preact.js 的 SRDB WebUI，使用无构建工具工作流（No Build Workflow）。

## 技术栈

- **Preact 10.19.3** - 轻量级 React 替代品（3KB）
- **HTM 3.1.1** - JSX 替代方案，直接在浏览器中使用
- **dom-align** - 智能元素定位（用于 Popover 和 Tooltip）
- **ESM.sh** - ES Modules CDN

## 特性

✅ **零构建工具** - 无需 Webpack、Vite 等构建工具
✅ **原生 ES Modules** - 直接使用 `import` 语法
✅ **HTM 语法** - 使用 `html` 标签模板字符串代替 JSX
✅ **主题切换** - 支持深色/浅色主题，持久化保存
✅ **智能 Popover** - 鼠标悬停单元格显示完整数据
✅ **字段 Tooltip** - 悬停表头显示字段注释，悬停表名显示表注释
✅ **响应式设计** - 适配不同屏幕尺寸
✅ **数据分页** - 支持自定义每页显示行数（20/50/100/200）+ 跳页功能
✅ **列选择器** - 下拉菜单选择要显示的列，支持持久化保存
✅ **详情模态框** - 点击眼睛图标查看单条记录的完整数据
✅ **Manifest 弹窗** - 点击按钮打开弹窗，实时显示 LSM-Tree 结构和 Compaction 统计
✅ **索引标识** - 有索引的字段显示 🔍 图标
✅ **悬浮加载提示** - 加载时顶部显示悬浮提示，不影响布局

## 目录结构

```
webui/
├── static/
│   ├── index.html          # 主 HTML 文件
│   ├── css/
│   │   └── styles.css      # 全局样式（支持主题）
│   │
│   └── js/
│       ├── main.js         # 应用入口
│       ├── hooks/          # 自定义 Hooks
│       │   ├── useCellPopover.js    # 单元格 Popover Hook
│       │   └── useTooltip.js        # Tooltip Hook
│       │
│       └── components/     # Preact 组件
│           ├── App.js                  # 主应用组件
│           ├── Sidebar.js              # 侧边栏（表列表）
│           ├── TableItem.js            # 表列表项（可展开字段）
│           ├── FieldList.js            # 字段列表
│           ├── TableView.js            # 表视图（主容器）
│           ├── ColumnSelector.js       # 列选择器（下拉菜单）
│           ├── DataTable.js            # 数据表格
│           ├── TableRow.js             # 表格行
│           ├── TableCell.js            # 表格单元格
│           ├── Pagination.js           # 分页器（sticky 底部）
│           ├── PageJumper.js           # 跳页输入框
│           ├── RowDetailModal.js       # 记录详情模态框
│           ├── ManifestModal.js        # Manifest 弹窗
│           ├── ManifestView.js         # Manifest 视图
│           ├── LevelCard.js            # 层级卡片
│           ├── FileCard.js             # 文件卡片
│           ├── StatCard.js             # 统计卡片
│           └── CompactionStats.js      # Compaction 统计
│
├── webui.go                    # Go 后端处理器
└── README.md                   # 本文件
```

## 快速开始

### 1. 启动后端服务

```bash
cd /path/to/srdb/examples/webui
go run main.go serve --db ./data --addr :8080
```

### 2. 访问 WebUI

浏览器打开：`http://localhost:8080/`

## 使用技巧

1. **查看字段说明**：鼠标悬停在表头字段名上，会显示该字段的 comment
2. **查看表说明**：鼠标悬停在表名上，会显示该表的 comment
3. **选择显示列**：点击右上角的 "Columns" 按钮，勾选要显示的列，选择会自动保存
4. **查看完整数据**：鼠标悬停在表格单元格上，会显示该单元格的完整数据
5. **查看记录详情**：鼠标悬停在行上，点击出现的眼睛图标查看完整记录
6. **跳转到指定页**：在分页器输入页码，点击"跳转"按钮
7. **查看 LSM-Tree 结构**：点击右上角的 "📊 Manifest" 按钮打开弹窗
8. **展开表字段**：在侧边栏点击表名前的 ▶ 图标展开查看所有字段
9. **识别索引字段**：带有 🔍 图标的字段表示该字段已建立索引
10. **主题切换**：点击左上角的 ☀️/🌙 图标切换深色/浅色主题

## 核心组件说明

### App.js - 主应用
负责：
- 主题管理（深色/浅色切换，LocalStorage 持久化）
- 表列表加载和选择
- 侧边栏 + 主内容区布局

### Sidebar.js - 侧边栏
特性：
- 可展开的表列表（点击 ▶ 展开字段）
- 表统计信息（字段数）
- 字段列表显示（字段名、类型、索引图标）
- 选中状态高亮

### TableView.js - 表视图容器
包含：
- 表名标题（悬停显示表注释）
- 行数统计（格式化显示 K/M）
- Manifest 按钮（打开弹窗）
- 列选择器按钮
- DataTable 组件
- ManifestModal 组件

### ColumnSelector.js - 列选择器
特性：
- 下拉菜单式选择器
- 多选复选框
- 显示字段类型和索引图标
- LocalStorage 持久化保存
- 选中数量徽章显示

### DataTable.js - 数据表格
功能：
- 响应式表格渲染
- 表头 Tooltip（显示字段注释）
- 单元格 Popover（悬停显示完整数据）
- 时间格式化（`_time` 字段）
- 悬浮加载提示（顶部居中）
- Sticky 分页器（粘在底部）

### TableRow.js - 表格行
特性：
- 悬停高亮效果
- 操作列眼睛图标（仅悬停时显示，使用 opacity 动画）
- 点击查看详情

### Pagination.js - 分页组件
功能：
- 上一页/下一页导航
- 每页行数选择（20/50/100/200）
- 跳页功能（PageJumper）
- Sticky 定位在底部
- 显示当前页范围和总数

### RowDetailModal.js - 记录详情
特性：
- 全屏模态框
- 显示单条记录的所有字段
- 系统字段标记（`_seq`, `_time`）
- JSON 格式化显示
- ESC 键和点击遮罩层关闭

### ManifestModal.js - Manifest 弹窗
特性：
- 90vw 宽度弹窗（最大 1200px）
- 包装 ManifestView 组件
- ESC 键和点击外部关闭
- 阻止背景滚动

### ManifestView.js - Manifest 视图
展示：
- 统计卡片（总文件数、总大小、下一个文件号、最后序列号）
- 各层级详情（L0-L6+）
  - 层级标识（彩色徽章）
  - 文件数和总大小
  - Compaction Score（带进度条）
  - 文件列表（文件号、大小、行数、Key 范围）
- Compaction 统计（合并次数、读写字节等）
- 只显示有文件的层级

## 自定义 Hooks

### useCellPopover.js
用于单元格数据预览的 Popover：
- 使用 dom-align 智能定位
- 自动处理溢出和边界
- 延迟隐藏避免闪烁
- 支持 JSON 格式化显示

### useTooltip.js
用于表头和表名的 Tooltip：
- 显示字段 comment 或表 comment
- 使用 dom-align 定位在元素下方
- 无延迟即时显示
- 最大宽度 300px，自动换行

## Preact + HTM 语法示例

### 基本组件

```javascript
import { html } from 'htm/preact';

export function MyComponent({ name }) {
    return html`
        <div class="hello">
            <h1>Hello, ${name}!</h1>
        </div>
    `;
}
```

### 使用 Hooks

```javascript
import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';

export function Counter() {
    const [count, setCount] = useState(0);

    useEffect(() => {
        console.log('Count changed:', count);
    }, [count]);

    return html`
        <div>
            <p>Count: ${count}</p>
            <button onClick=${() => setCount(count + 1)}>
                Increment
            </button>
        </div>
    `;
}
```

### 条件渲染

```javascript
${loading
    ? html`<div class="loading">Loading...</div>`
    : html`<div class="content">${data}</div>`
}
```

### 列表渲染

```javascript
${items.map(item => html`
    <div key=${item.id}>${item.name}</div>
`)}
```

## 主题切换

主题通过修改 `<html>` 元素的 `data-theme` 属性实现：

```javascript
// 切换到浅色主题
document.documentElement.setAttribute('data-theme', 'light');

// 切换回深色主题
document.documentElement.removeAttribute('data-theme');
```

CSS 变量会根据主题自动切换：

```css
:root {
    --bg-main: #0f0f1a;  /* 深色主题 */
}

:root[data-theme="light"] {
    --bg-main: #f5f5f7;  /* 浅色主题 */
}
```

## API 接口

WebUI v2 使用的 API 端点：

- `GET /api/tables` - 获取表列表（含行数统计）
- `GET /api/tables/:name/schema` - 获取表 Schema
- `GET /api/tables/:name/data?limit=100&offset=0` - 获取表数据（支持分页）
- `GET /api/tables/:name/data/:seq` - 获取单条记录详情（完整数据）
- `GET /api/tables/:name/manifest` - 获取 Manifest 信息（LSM-Tree 结构）

## 开发建议

### 添加新组件

1. 在 `static/js/components/` 创建新文件
2. 使用 `html` 标签模板语法
3. 导出函数组件

```javascript
import { html } from 'htm/preact';

export function NewComponent({ prop1, prop2 }) {
    return html`
        <div>Your content here</div>
    `;
}
```

### 内联样式 vs CSS

推荐使用内联样式（通过 `styles` 对象）以获得更好的组件隔离性和主题支持：

```javascript
const styles = {
    container: {
        padding: '20px',
        background: 'var(--bg-elevated)',
        color: 'var(--text-primary)'
    }
};

return html`<div style=${styles.container}>Content</div>`;
```

### 状态管理

使用 `useState` 和 `useEffect` 管理组件状态：

```javascript
import { useState, useEffect } from 'preact/hooks';

// 本地状态
const [count, setCount] = useState(0);

// 副作用处理
useEffect(() => {
    // 组件挂载时执行
    console.log('Component mounted');

    // 清理函数
    return () => {
        console.log('Component unmounted');
    };
}, []); // 空依赖数组表示只在挂载/卸载时执行
```

## 参考资料

- [Preact 官方文档](https://preactjs.com/)
- [HTM 文档](https://github.com/developit/htm)
- [dom-align 文档](https://github.com/yiminghe/dom-align)
- [ESM.sh CDN](https://esm.sh/)

## License

MIT
