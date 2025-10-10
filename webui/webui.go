package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"code.tczkiot.com/wlw/srdb"
)

//go:embed static
var staticFS embed.FS

// WebUI Web 界面处理器
type WebUI struct {
	db      *srdb.Database
	handler http.Handler
}

// NewWebUI 创建 WebUI 实例
func NewWebUI(db *srdb.Database) *WebUI {
	ui := &WebUI{db: db}
	ui.handler = ui.setupHandler()
	return ui
}

// setupHandler 设置 HTTP Handler
func (ui *WebUI) setupHandler() http.Handler {
	mux := http.NewServeMux()

	// API endpoints - 纯 JSON API
	mux.HandleFunc("/api/tables", ui.handleListTables)
	mux.HandleFunc("/api/tables/", ui.handleTableAPI)

	// 静态文件服务
	staticFiles, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))

	// 首页
	mux.HandleFunc("/", ui.handleIndex)

	return mux
}

// Handler 返回 HTTP Handler
func (ui *WebUI) Handler() http.Handler {
	return ui.handler
}

// ServeHTTP 实现 http.Handler 接口
func (ui *WebUI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ui.handler.ServeHTTP(w, r)
}

// handleListTables 处理获取表列表请求
func (ui *WebUI) handleListTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type FieldInfo struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Indexed bool   `json:"indexed"`
		Comment string `json:"comment"`
	}

	type TableListItem struct {
		Name      string      `json:"name"`
		CreatedAt int64       `json:"created_at"`
		Fields    []FieldInfo `json:"fields"`
	}

	allTables := ui.db.GetAllTablesInfo()
	tables := make([]TableListItem, 0, len(allTables))
	for name, table := range allTables {
		schema := table.GetSchema()
		fields := make([]FieldInfo, 0, len(schema.Fields))
		for _, field := range schema.Fields {
			fields = append(fields, FieldInfo{
				Name:    field.Name,
				Type:    field.Type.String(),
				Indexed: field.Indexed,
				Comment: field.Comment,
			})
		}

		tables = append(tables, TableListItem{
			Name:      name,
			CreatedAt: 0, // TODO: Table 不再有 createdAt 字段
			Fields:    fields,
		})
	}

	// 按表名排序
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

// handleTableAPI 处理表相关的 API 请求
func (ui *WebUI) handleTableAPI(w http.ResponseWriter, r *http.Request) {
	// 解析路径: /api/tables/{name}/schema 或 /api/tables/{name}/data
	path := strings.TrimPrefix(r.URL.Path, "/api/tables/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	tableName := parts[0]
	action := parts[1]

	switch action {
	case "schema":
		ui.handleTableSchema(w, r, tableName)
	case "data":
		// 检查是否是单条数据查询: /api/tables/{name}/data/{seq}
		if len(parts) >= 3 {
			ui.handleTableDataBySeq(w, r, tableName, parts[2])
		} else {
			ui.handleTableData(w, r, tableName)
		}
	case "manifest":
		ui.handleTableManifest(w, r, tableName)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleTableSchema 处理获取表 schema 请求
func (ui *WebUI) handleTableSchema(w http.ResponseWriter, r *http.Request, tableName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	table, err := ui.db.GetTable(tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	schema := table.GetSchema()

	type FieldInfo struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Indexed bool   `json:"indexed"`
		Comment string `json:"comment"`
	}

	fields := make([]FieldInfo, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		fields = append(fields, FieldInfo{
			Name:    field.Name,
			Type:    field.Type.String(),
			Indexed: field.Indexed,
			Comment: field.Comment,
		})
	}

	response := map[string]any{
		"name":   schema.Name,
		"fields": fields,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTableManifest 处理获取表 manifest 信息请求
func (ui *WebUI) handleTableManifest(w http.ResponseWriter, r *http.Request, tableName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	table, err := ui.db.GetTable(tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	versionSet := table.GetVersionSet()
	version := versionSet.GetCurrent()

	// 构建每层的信息
	type FileInfo struct {
		FileNumber int64 `json:"file_number"`
		Level      int   `json:"level"`
		FileSize   int64 `json:"file_size"`
		MinKey     int64 `json:"min_key"`
		MaxKey     int64 `json:"max_key"`
		RowCount   int64 `json:"row_count"`
	}

	type LevelInfo struct {
		Level     int        `json:"level"`
		FileCount int        `json:"file_count"`
		TotalSize int64      `json:"total_size"`
		Score     float64    `json:"score"`
		Files     []FileInfo `json:"files"`
	}

	// 获取 Compaction Manager
	compactionMgr := table.GetCompactionManager()

	levels := make([]LevelInfo, 0, 7)
	for level := range 7 {
		// 只调用一次 GetLevel，避免重复复制文件列表
		files := version.GetLevel(level)

		totalSize := int64(0)
		fileInfos := make([]FileInfo, 0, len(files))
		for _, f := range files {
			totalSize += f.FileSize
			fileInfos = append(fileInfos, FileInfo{
				FileNumber: f.FileNumber,
				Level:      f.Level,
				FileSize:   f.FileSize,
				MinKey:     f.MinKey,
				MaxKey:     f.MaxKey,
				RowCount:   f.RowCount,
			})
		}

		// 使用已计算的 totalSize 和 fileCount 计算 score，避免再次调用 GetLevel
		score := 0.0
		if len(files) > 0 && level < 3 { // L3 是最后一层，不需要 compaction
			// 直接计算 score，避免调用 picker.GetLevelScore（它会再次获取 files）
			// 使用下一级的大小限制来计算得分（从 Options 配置读取）
			nextLevelLimit := compactionMgr.GetLevelSizeLimit(level + 1)
			if nextLevelLimit > 0 {
				score = float64(totalSize) / float64(nextLevelLimit)
			}
		}

		levels = append(levels, LevelInfo{
			Level:     level,
			FileCount: len(files),
			TotalSize: totalSize,
			Score:     score,
			Files:     fileInfos,
		})
	}

	// 获取 Compaction 统计
	stats := compactionMgr.GetStats()

	response := map[string]any{
		"levels":           levels,
		"next_file_number": versionSet.GetNextFileNumber(),
		"last_sequence":    versionSet.GetLastSequence(),
		"compaction_stats": stats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTableDataBySeq 处理获取单条数据请求
func (ui *WebUI) handleTableDataBySeq(w http.ResponseWriter, r *http.Request, tableName string, seqStr string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	table, err := ui.db.GetTable(tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 解析 seq
	seq, err := strconv.ParseInt(seqStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid seq parameter", http.StatusBadRequest)
		return
	}

	// 获取数据
	row, err := table.Get(seq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Row not found: %v", err), http.StatusNotFound)
		return
	}

	// 构造响应（不进行剪裁，返回完整数据）
	rowData := make(map[string]any)
	rowData["_seq"] = row.Seq
	rowData["_time"] = row.Time
	maps.Copy(rowData, row.Data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rowData)
}

// handleTableData 处理获取表数据请求（分页）
func (ui *WebUI) handleTableData(w http.ResponseWriter, r *http.Request, tableName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	table, err := ui.db.GetTable(tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 解析分页参数
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")
	selectParam := r.URL.Query().Get("select") // 要选择的字段，逗号分隔

	page := 1
	pageSize := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 1000 {
			pageSize = ps
		}
	}

	// 解析要选择的字段
	var selectedFields []string
	if selectParam != "" {
		selectedFields = strings.Split(selectParam, ",")
		// 清理字段名（去除空格）
		for i := range selectedFields {
			selectedFields[i] = strings.TrimSpace(selectedFields[i])
		}
	}

	// 获取 schema 用于字段类型判断
	tableSchema := table.GetSchema()

	// 使用 Query API 获取数据，如果指定了字段则只查询指定字段（按字段压缩优化）
	queryBuilder := table.Query()
	if len(selectedFields) > 0 {
		// 确保 _seq 和 _time 总是被查询（用于构造响应）
		fieldsWithMeta := make([]string, 0, len(selectedFields)+2)
		hasSeq := false
		hasTime := false
		for _, field := range selectedFields {
			switch field {
			case "_seq":
				hasSeq = true
			case "_time":
				hasTime = true
			}
		}
		if !hasSeq {
			fieldsWithMeta = append(fieldsWithMeta, "_seq")
		}
		if !hasTime {
			fieldsWithMeta = append(fieldsWithMeta, "_time")
		}
		fieldsWithMeta = append(fieldsWithMeta, selectedFields...)

		queryBuilder = queryBuilder.Select(fieldsWithMeta...)
	}
	queryRows, err := queryBuilder.Rows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}
	defer queryRows.Close()

	// 计算分页范围
	offset := (page - 1) * pageSize
	currentIndex := 0

	// 直接在遍历时进行分页和字段处理
	const maxStringLength = 100 // 最大字符串长度
	data := make([]map[string]any, 0, pageSize)
	totalRows := int64(0)

	for queryRows.Next() {
		totalRows++

		// 跳过不在当前页的数据
		if currentIndex < offset {
			currentIndex++
			continue
		}

		// 已经收集够当前页的数据
		if len(data) >= pageSize {
			continue
		}

		row := queryRows.Row()
		rowData := make(map[string]any)
		rowData["_seq"] = row.Data()["_seq"]
		rowData["_time"] = row.Data()["_time"]

		// 遍历所有字段并进行字符串截断
		for k, v := range row.Data() {
			if k == "_seq" || k == "_time" {
				continue
			}

			// 检查字段类型
			field, err := tableSchema.GetField(k)
			if err == nil && field.Type == srdb.String {
				// 对字符串字段进行剪裁
				if str, ok := v.(string); ok {
					runes := []rune(str)
					if len(runes) > maxStringLength {
						rowData[k] = string(runes[:maxStringLength]) + "..."
						rowData[k+"_truncated"] = true
						continue
					}
				}
			}
			rowData[k] = v
		}

		data = append(data, rowData)
		currentIndex++
	}

	response := map[string]any{
		"data":       data,
		"page":       page,
		"pageSize":   pageSize,
		"totalRows":  totalRows,
		"totalPages": (totalRows + int64(pageSize) - 1) / int64(pageSize),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleIndex 处理首页请求
func (ui *WebUI) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// 读取 index.html
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to load page", http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}
