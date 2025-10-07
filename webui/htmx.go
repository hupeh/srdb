package webui

import (
	"bytes"
	"fmt"
	"html"
	"strings"
)

// HTML 渲染辅助函数

// renderTablesHTML 渲染表列表 HTML
func renderTablesHTML(tables []TableListItem) string {
	var buf bytes.Buffer

	for _, table := range tables {
		buf.WriteString(`<div class="table-item" data-table="`)
		buf.WriteString(html.EscapeString(table.Name))
		buf.WriteString(`">`)
		buf.WriteString(`<div class="table-header" onclick="selectTable('`)
		buf.WriteString(html.EscapeString(table.Name))
		buf.WriteString(`')">`)

		// 左侧：展开图标和表名
		buf.WriteString(`<div class="table-header-left">`)
		buf.WriteString(`<span class="expand-icon" onclick="event.stopPropagation(); toggleExpand('`)
		buf.WriteString(html.EscapeString(table.Name))
		buf.WriteString(`')">▶</span>`)
		buf.WriteString(`<span class="table-name">`)
		buf.WriteString(html.EscapeString(table.Name))
		buf.WriteString(`</span></div>`)

		// 右侧：字段数量
		buf.WriteString(`<span class="table-count">`)
		buf.WriteString(formatCount(int64(len(table.Fields))))
		buf.WriteString(` fields</span>`)
		buf.WriteString(`</div>`)

		// Schema 字段列表（默认隐藏）
		if len(table.Fields) > 0 {
			buf.WriteString(`<div class="schema-fields">`)
			for _, field := range table.Fields {
				buf.WriteString(`<div class="field-item">`)
				buf.WriteString(`<span class="field-name">`)
				buf.WriteString(html.EscapeString(field.Name))
				buf.WriteString(`</span>`)
				buf.WriteString(`<span class="field-type">`)
				buf.WriteString(html.EscapeString(field.Type))
				buf.WriteString(`</span>`)
				if field.Indexed {
					buf.WriteString(`<span class="field-indexed">●indexed</span>`)
				}
				buf.WriteString(`</div>`)
			}
			buf.WriteString(`</div>`)
		}

		buf.WriteString(`</div>`)
	}

	return buf.String()
}

// renderDataViewHTML 渲染数据视图 HTML
func renderDataViewHTML(tableName string, schema SchemaInfo, tableData TableDataResponse) string {
	var buf bytes.Buffer

	// 标题
	buf.WriteString(`<h2>`)
	buf.WriteString(html.EscapeString(tableName))
	buf.WriteString(`</h2>`)

	// 视图切换标签
	buf.WriteString(`<div class="view-tabs">`)
	buf.WriteString(`<button class="view-tab active" onclick="switchView('`)
	buf.WriteString(html.EscapeString(tableName))
	buf.WriteString(`', 'data')">Data</button>`)
	buf.WriteString(`<button class="view-tab" onclick="switchView('`)
	buf.WriteString(html.EscapeString(tableName))
	buf.WriteString(`', 'manifest')">Manifest / LSM-Tree</button>`)
	buf.WriteString(`</div>`)

	// Schema 部分
	if len(schema.Fields) > 0 {
		buf.WriteString(`<div class="schema-section">`)
		buf.WriteString(`<h3>Schema <span style="font-size: 12px; font-weight: 400; color: var(--text-secondary);">(点击字段卡片选择要显示的列)</span></h3>`)
		buf.WriteString(`<div class="schema-grid">`)
		for _, field := range schema.Fields {
			buf.WriteString(`<div class="schema-field-card selected" data-column="`)
			buf.WriteString(html.EscapeString(field.Name))
			buf.WriteString(`" onclick="toggleColumn('`)
			buf.WriteString(html.EscapeString(field.Name))
			buf.WriteString(`')">`)
			buf.WriteString(`<div class="field-item">`)
			buf.WriteString(`<span class="field-name">`)
			buf.WriteString(html.EscapeString(field.Name))
			buf.WriteString(`</span>`)
			buf.WriteString(`<span class="field-type">`)
			buf.WriteString(html.EscapeString(field.Type))
			buf.WriteString(`</span>`)
			if field.Indexed {
				buf.WriteString(`<span class="field-indexed">●indexed</span>`)
			}
			buf.WriteString(`</div>`)
			buf.WriteString(`<div class="field-comment">`)
			if field.Comment != "" {
				buf.WriteString(html.EscapeString(field.Comment))
			}
			buf.WriteString(`</div>`)
			buf.WriteString(`</div>`)
		}
		buf.WriteString(`</div>`)
		buf.WriteString(`</div>`)
	}

	// 数据表格
	buf.WriteString(`<h3>Data (`)
	buf.WriteString(formatCount(tableData.TotalRows))
	buf.WriteString(` rows)</h3>`)

	if len(tableData.Data) == 0 {
		buf.WriteString(`<div class="empty"><p>No data available</p></div>`)
		return buf.String()
	}

	// 获取列并排序：_seq 第1列，_time 倒数第2列
	columns := []string{}
	otherColumns := []string{}
	hasSeq := false
	hasTime := false

	if len(tableData.Data) > 0 {
		for key := range tableData.Data[0] {
			if !strings.HasSuffix(key, "_truncated") {
				if key == "_seq" {
					hasSeq = true
				} else if key == "_time" {
					hasTime = true
				} else {
					otherColumns = append(otherColumns, key)
				}
			}
		}
	}

	// 按顺序组装：_seq, 其他列, _time
	if hasSeq {
		columns = append(columns, "_seq")
	}
	columns = append(columns, otherColumns...)
	if hasTime {
		columns = append(columns, "_time")
	}

	// 表格
	buf.WriteString(`<div class="table-wrapper">`)
	buf.WriteString(`<table class="data-table">`)
	buf.WriteString(`<thead><tr>`)
	for _, col := range columns {
		buf.WriteString(`<th data-column="`)
		buf.WriteString(html.EscapeString(col))
		buf.WriteString(`" title="`)
		buf.WriteString(html.EscapeString(col))
		buf.WriteString(`">`)
		buf.WriteString(html.EscapeString(col))
		buf.WriteString(`</th>`)
	}
	buf.WriteString(`<th style="width: 80px;">Actions</th>`)
	buf.WriteString(`</tr></thead>`)

	buf.WriteString(`<tbody>`)
	for _, row := range tableData.Data {
		buf.WriteString(`<tr>`)
		for _, col := range columns {
			value := row[col]
			buf.WriteString(`<td data-column="`)
			buf.WriteString(html.EscapeString(col))
			buf.WriteString(`" onclick="showCellContent('`)
			buf.WriteString(escapeJSString(fmt.Sprintf("%v", value)))
			buf.WriteString(`')" title="Click to view full content">`)
			buf.WriteString(html.EscapeString(fmt.Sprintf("%v", value)))

			// 检查是否被截断
			if truncated, ok := row[col+"_truncated"]; ok && truncated == true {
				buf.WriteString(`<span class="truncated-icon" title="This field is truncated">✂️</span>`)
			}

			buf.WriteString(`</td>`)
		}

		// Actions 列
		buf.WriteString(`<td style="text-align: center;">`)
		buf.WriteString(`<button class="row-detail-btn" onclick="showRowDetail('`)
		buf.WriteString(html.EscapeString(tableName))
		buf.WriteString(`', `)
		buf.WriteString(fmt.Sprintf("%v", row["_seq"]))
		buf.WriteString(`)" title="View full row data">Detail</button>`)
		buf.WriteString(`</td>`)

		buf.WriteString(`</tr>`)
	}
	buf.WriteString(`</tbody>`)
	buf.WriteString(`</table>`)
	buf.WriteString(`</div>`)

	// 分页
	buf.WriteString(renderPagination(tableData))

	return buf.String()
}

// renderManifestViewHTML 渲染 Manifest 视图 HTML
func renderManifestViewHTML(tableName string, manifest ManifestResponse) string {
	var buf bytes.Buffer

	// 标题
	buf.WriteString(`<h2>`)
	buf.WriteString(html.EscapeString(tableName))
	buf.WriteString(`</h2>`)

	// 视图切换标签
	buf.WriteString(`<div class="view-tabs">`)
	buf.WriteString(`<button class="view-tab" onclick="switchView('`)
	buf.WriteString(html.EscapeString(tableName))
	buf.WriteString(`', 'data')">Data</button>`)
	buf.WriteString(`<button class="view-tab active" onclick="switchView('`)
	buf.WriteString(html.EscapeString(tableName))
	buf.WriteString(`', 'manifest')">Manifest / LSM-Tree</button>`)
	buf.WriteString(`</div>`)

	// 标题和控制按钮
	buf.WriteString(`<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">`)
	buf.WriteString(`<h3>LSM-Tree Structure</h3>`)
	buf.WriteString(`<div class="control-buttons">`)
	buf.WriteString(`<button>📖 Expand All</button>`)
	buf.WriteString(`<button>📕 Collapse All</button>`)
	buf.WriteString(`</div>`)
	buf.WriteString(`</div>`)

	// 统计卡片
	totalLevels := len(manifest.Levels)
	totalFiles := 0
	totalSize := int64(0)
	for _, level := range manifest.Levels {
		totalFiles += level.FileCount
		totalSize += level.TotalSize
	}

	buf.WriteString(`<div class="manifest-stats">`)

	// Active Levels
	buf.WriteString(`<div class="stat-card">`)
	buf.WriteString(`<div class="stat-label">Active Levels</div>`)
	buf.WriteString(`<div class="stat-value">`)
	buf.WriteString(fmt.Sprintf("%d", totalLevels))
	buf.WriteString(`</div></div>`)

	// Total Files
	buf.WriteString(`<div class="stat-card">`)
	buf.WriteString(`<div class="stat-label">Total Files</div>`)
	buf.WriteString(`<div class="stat-value">`)
	buf.WriteString(fmt.Sprintf("%d", totalFiles))
	buf.WriteString(`</div></div>`)

	// Total Size
	buf.WriteString(`<div class="stat-card">`)
	buf.WriteString(`<div class="stat-label">Total Size</div>`)
	buf.WriteString(`<div class="stat-value">`)
	buf.WriteString(formatBytes(totalSize))
	buf.WriteString(`</div></div>`)

	// Next File Number
	buf.WriteString(`<div class="stat-card">`)
	buf.WriteString(`<div class="stat-label">Next File Number</div>`)
	buf.WriteString(`<div class="stat-value">`)
	buf.WriteString(fmt.Sprintf("%d", manifest.NextFileNumber))
	buf.WriteString(`</div></div>`)

	// Last Sequence
	buf.WriteString(`<div class="stat-card">`)
	buf.WriteString(`<div class="stat-label">Last Sequence</div>`)
	buf.WriteString(`<div class="stat-value">`)
	buf.WriteString(fmt.Sprintf("%d", manifest.LastSequence))
	buf.WriteString(`</div></div>`)

	// Total Compactions
	buf.WriteString(`<div class="stat-card">`)
	buf.WriteString(`<div class="stat-label">Total Compactions</div>`)
	buf.WriteString(`<div class="stat-value">`)
	totalCompactions := 0
	if manifest.CompactionStats != nil {
		if tc, ok := manifest.CompactionStats["total_compactions"]; ok {
			if tcInt, ok := tc.(float64); ok {
				totalCompactions = int(tcInt)
			}
		}
	}
	buf.WriteString(fmt.Sprintf("%d", totalCompactions))
	buf.WriteString(`</div></div>`)

	buf.WriteString(`</div>`)

	// 渲染所有层级（L0-L6）
	for i := 0; i <= 6; i++ {
		var level *LevelInfo
		for j := range manifest.Levels {
			if manifest.Levels[j].Level == i {
				level = &manifest.Levels[j]
				break
			}
		}

		if level == nil {
			// 创建空层级
			level = &LevelInfo{
				Level:     i,
				FileCount: 0,
				TotalSize: 0,
				Score:     0,
				Files:     []FileInfo{},
			}
		}

		buf.WriteString(renderLevelCard(*level))
	}

	return buf.String()
}

// renderLevelCard 渲染层级卡片
func renderLevelCard(level LevelInfo) string {
	var buf bytes.Buffer

	scoreClass := "normal"
	if level.Score >= 1.0 {
		scoreClass = "critical"
	} else if level.Score >= 0.8 {
		scoreClass = "warning"
	}

	buf.WriteString(`<div class="level-card" data-level="`)
	buf.WriteString(fmt.Sprintf("%d", level.Level))
	buf.WriteString(`">`)
	buf.WriteString(`<div class="level-header" onclick="toggleLevel(`)
	buf.WriteString(fmt.Sprintf("%d", level.Level))
	buf.WriteString(`)">`)

	// 左侧：展开图标和标题
	buf.WriteString(`<div style="display: flex; align-items: center; gap: 10px;">`)
	buf.WriteString(`<span class="expand-icon">▶</span>`)
	buf.WriteString(`<div class="level-title">Level `)
	buf.WriteString(fmt.Sprintf("%d", level.Level))
	buf.WriteString(`</div></div>`)

	// 右侧：统计信息
	buf.WriteString(`<div class="level-stats">`)
	buf.WriteString(`<span>`)
	buf.WriteString(fmt.Sprintf("%d", level.FileCount))
	buf.WriteString(` files</span>`)
	buf.WriteString(`<span>`)
	buf.WriteString(formatBytes(level.TotalSize))
	buf.WriteString(`</span>`)
	buf.WriteString(`<span class="score-badge `)
	buf.WriteString(scoreClass)
	buf.WriteString(`">Score: `)
	buf.WriteString(fmt.Sprintf("%.2f", level.Score))
	buf.WriteString(`</span>`)
	buf.WriteString(`</div>`)

	buf.WriteString(`</div>`)

	// 文件列表（默认隐藏）
	buf.WriteString(`<div class="file-list">`)
	if len(level.Files) == 0 {
		buf.WriteString(`<div class="empty-files">No files in this level</div>`)
	} else {
		for _, file := range level.Files {
			buf.WriteString(`<div class="file-card">`)
			buf.WriteString(`<div class="file-header">`)
			buf.WriteString(`<span>File #`)
			buf.WriteString(fmt.Sprintf("%d", file.FileNumber))
			buf.WriteString(`</span>`)
			buf.WriteString(`<b>`)
			buf.WriteString(formatBytes(file.FileSize))
			buf.WriteString(`</b>`)
			buf.WriteString(`</div>`)

			buf.WriteString(`<div class="file-detail">`)
			buf.WriteString(`<span>Key Range:</span>`)
			buf.WriteString(`<span>`)
			buf.WriteString(fmt.Sprintf("%d - %d", file.MinKey, file.MaxKey))
			buf.WriteString(`</span></div>`)

			buf.WriteString(`<div class="file-detail">`)
			buf.WriteString(`<span>Rows:</span>`)
			buf.WriteString(`<span>`)
			buf.WriteString(formatCount(file.RowCount))
			buf.WriteString(`</span></div>`)

			buf.WriteString(`</div>`)
		}
	}
	buf.WriteString(`</div>`)

	buf.WriteString(`</div>`)
	return buf.String()
}

// renderPagination 渲染分页 HTML
func renderPagination(data TableDataResponse) string {
	var buf bytes.Buffer

	buf.WriteString(`<div class="pagination">`)

	// 页大小选择器
	buf.WriteString(`<select onchange="changePageSize(this.value)">`)
	for _, size := range []int{10, 20, 50, 100} {
		buf.WriteString(`<option value="`)
		buf.WriteString(fmt.Sprintf("%d", size))
		buf.WriteString(`"`)
		if int64(size) == data.PageSize {
			buf.WriteString(` selected`)
		}
		buf.WriteString(`>`)
		buf.WriteString(fmt.Sprintf("%d", size))
		buf.WriteString(` / page</option>`)
	}
	buf.WriteString(`</select>`)

	// 上一页按钮
	buf.WriteString(`<button onclick="changePage(-1)"`)
	if data.Page <= 1 {
		buf.WriteString(` disabled`)
	}
	buf.WriteString(`>Previous</button>`)

	// 页码信息
	buf.WriteString(`<span>Page `)
	buf.WriteString(fmt.Sprintf("%d", data.Page))
	buf.WriteString(` of `)
	buf.WriteString(fmt.Sprintf("%d", data.TotalPages))
	buf.WriteString(` (`)
	buf.WriteString(formatCount(data.TotalRows))
	buf.WriteString(` rows)</span>`)

	// 跳转输入框
	buf.WriteString(`<input type="number" min="1" max="`)
	buf.WriteString(fmt.Sprintf("%d", data.TotalPages))
	buf.WriteString(`" placeholder="Jump to" onkeydown="if(event.key==='Enter') jumpToPage(this.value)">`)

	// Go 按钮
	buf.WriteString(`<button onclick="jumpToPage(this.previousElementSibling.value)">Go</button>`)

	// 下一页按钮
	buf.WriteString(`<button onclick="changePage(1)"`)
	if data.Page >= data.TotalPages {
		buf.WriteString(` disabled`)
	}
	buf.WriteString(`>Next</button>`)

	buf.WriteString(`</div>`)
	return buf.String()
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	size := float64(bytes)
	for size >= k && i < len(sizes)-1 {
		size /= k
		i++
	}
	return fmt.Sprintf("%.2f %s", size, sizes[i])
}

// formatCount 格式化数量（K/M）
func formatCount(count int64) string {
	if count >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	}
	if count >= 1000 {
		return fmt.Sprintf("%.1fK", float64(count)/1000)
	}
	return fmt.Sprintf("%d", count)
}

// escapeJSString 转义 JavaScript 字符串
func escapeJSString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// 数据结构定义
type TableListItem struct {
	Name      string      `json:"name"`
	CreatedAt int64       `json:"created_at"`
	Fields    []FieldInfo `json:"fields"`
}

type FieldInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Indexed bool   `json:"indexed"`
	Comment string `json:"comment"`
}

type SchemaInfo struct {
	Name   string      `json:"name"`
	Fields []FieldInfo `json:"fields"`
}

type TableDataResponse struct {
	Data       []map[string]any `json:"data"`
	Page       int64            `json:"page"`
	PageSize   int64            `json:"pageSize"`
	TotalRows  int64            `json:"totalRows"`
	TotalPages int64            `json:"totalPages"`
}

type ManifestResponse struct {
	Levels          []LevelInfo    `json:"levels"`
	NextFileNumber  int64          `json:"next_file_number"`
	LastSequence    int64          `json:"last_sequence"`
	CompactionStats map[string]any `json:"compaction_stats"`
}

type LevelInfo struct {
	Level     int        `json:"level"`
	FileCount int        `json:"file_count"`
	TotalSize int64      `json:"total_size"`
	Score     float64    `json:"score"`
	Files     []FileInfo `json:"files"`
}

type FileInfo struct {
	FileNumber int64 `json:"file_number"`
	Level      int   `json:"level"`
	FileSize   int64 `json:"file_size"`
	MinKey     int64 `json:"min_key"`
	MaxKey     int64 `json:"max_key"`
	RowCount   int64 `json:"row_count"`
}
