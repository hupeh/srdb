package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"code.tczkiot.com/srdb"
	"code.tczkiot.com/srdb/sst"
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

	// API endpoints - JSON
	mux.HandleFunc("/api/tables", ui.handleListTables)
	mux.HandleFunc("/api/tables/", ui.handleTableAPI)

	// API endpoints - HTML (for htmx)
	mux.HandleFunc("/api/tables-html", ui.handleTablesHTML)
	mux.HandleFunc("/api/tables-view/", ui.handleTableViewHTML)

	// Debug endpoint - list embedded files
	mux.HandleFunc("/debug/files", ui.handleDebugFiles)

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
			CreatedAt: table.GetCreatedAt(),
			Fields:    fields,
		})
	}

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

	engine := table.GetEngine()
	versionSet := engine.GetVersionSet()
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

	// 获取 Compaction Manager 和 Picker
	compactionMgr := engine.GetCompactionManager()
	picker := compactionMgr.GetPicker()

	levels := make([]LevelInfo, 0)
	for level := 0; level < 7; level++ {
		files := version.GetLevel(level)
		if len(files) == 0 {
			continue
		}

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

		score := picker.GetLevelScore(version, level)

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
	rowData := make(map[string]interface{})
	rowData["_seq"] = row.Seq
	rowData["_time"] = row.Time
	for k, v := range row.Data {
		rowData[k] = v
	}

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
	}

	// 获取 schema 用于字段类型判断
	tableSchema := table.GetSchema()

	// 使用 Query API 获取所有数据（高效）
	queryRows, err := table.Query().Rows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}
	defer queryRows.Close()

	// 收集所有 rows 到内存中用于分页（对于大数据集，后续可以优化为流式处理）
	allRows := make([]*sst.Row, 0)
	for queryRows.Next() {
		row := queryRows.Row()
		// Row 是 query.Row 类型，需要获取其内部的 sst.Row
		// 直接构造 sst.Row
		sstRow := &sst.Row{
			Seq:  row.Data()["_seq"].(int64),
			Time: row.Data()["_time"].(int64),
			Data: make(map[string]any),
		}
		// 复制其他字段
		for k, v := range row.Data() {
			if k != "_seq" && k != "_time" {
				sstRow.Data[k] = v
			}
		}
		allRows = append(allRows, sstRow)
	}

	// 计算分页
	totalRows := int64(len(allRows))
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > int(totalRows) {
		end = int(totalRows)
	}

	// 获取当前页数据
	rows := make([]*sst.Row, 0, pageSize)
	if offset < int(totalRows) {
		rows = allRows[offset:end]
	}

	// 构造响应，对 string 字段进行剪裁
	const maxStringLength = 100 // 最大字符串长度（按字符计数，非字节）
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		rowData := make(map[string]any)

		// 如果指定了字段，只返回选定的字段
		if len(selectedFields) > 0 {
			for _, field := range selectedFields {
				field = strings.TrimSpace(field)
				if field == "_seq" {
					rowData["_seq"] = row.Seq
				} else if field == "_time" {
					rowData["_time"] = row.Time
				} else if v, ok := row.Data[field]; ok {
					// 检查字段类型
					fieldDef, err := tableSchema.GetField(field)
					if err == nil && fieldDef.Type == srdb.FieldTypeString {
						// 对字符串字段进行剪裁
						if str, ok := v.(string); ok {
							runes := []rune(str)
							if len(runes) > maxStringLength {
								rowData[field] = string(runes[:maxStringLength]) + "..."
								rowData[field+"_truncated"] = true
								continue
							}
						}
					}
					rowData[field] = v
				}
			}
		} else {
			// 返回所有字段
			rowData["_seq"] = row.Seq
			rowData["_time"] = row.Time
			for k, v := range row.Data {
				// 检查字段类型
				field, err := tableSchema.GetField(k)
				if err == nil && field.Type == srdb.FieldTypeString {
					// 对字符串字段进行剪裁（按 rune 截取，避免 CJK 等多字节字符乱码）
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
		}
		data = append(data, rowData)
	}

	response := map[string]interface{}{
		"data":       data,
		"page":       page,
		"pageSize":   pageSize,
		"totalRows":  totalRows,
		"totalPages": (totalRows + int64(pageSize) - 1) / int64(pageSize),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDebugFiles 列出所有嵌入的文件（调试用）
func (ui *WebUI) handleDebugFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "Embedded files in staticFS:")
	fs.WalkDir(staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(w, "ERROR walking %s: %v\n", path, err)
			return err
		}
		if d.IsDir() {
			fmt.Fprintf(w, "[DIR]  %s/\n", path)
		} else {
			info, _ := d.Info()
			fmt.Fprintf(w, "[FILE] %s (%d bytes)\n", path, info.Size())
		}
		return nil
	})
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

// handleTablesHTML 处理获取表列表 HTML 请求（for htmx）
func (ui *WebUI) handleTablesHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
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
			CreatedAt: table.GetCreatedAt(),
			Fields:    fields,
		})
	}

	html := renderTablesHTML(tables)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleTableViewHTML 处理获取表视图 HTML 请求（for htmx）
func (ui *WebUI) handleTableViewHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析路径: /api/tables-view/{name} 或 /api/tables-view/{name}/manifest
	path := strings.TrimPrefix(r.URL.Path, "/api/tables-view/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	tableName := parts[0]
	isManifest := len(parts) >= 2 && parts[1] == "manifest"

	table, err := ui.db.GetTable(tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if isManifest {
		// 返回 Manifest 视图 HTML
		ui.renderManifestHTML(w, r, tableName, table)
	} else {
		// 返回 Data 视图 HTML
		ui.renderDataHTML(w, r, tableName, table)
	}
}

// renderDataHTML 渲染数据视图 HTML
func (ui *WebUI) renderDataHTML(w http.ResponseWriter, r *http.Request, tableName string, table *srdb.Table) {
	// 解析分页参数
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

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

	// 获取 schema
	tableSchema := table.GetSchema()
	schemaInfo := SchemaInfo{
		Name:   tableSchema.Name,
		Fields: make([]FieldInfo, 0, len(tableSchema.Fields)),
	}
	for _, field := range tableSchema.Fields {
		schemaInfo.Fields = append(schemaInfo.Fields, FieldInfo{
			Name:    field.Name,
			Type:    field.Type.String(),
			Indexed: field.Indexed,
			Comment: field.Comment,
		})
	}

	// 使用 Query API 获取所有数据
	queryRows, err := table.Query().Rows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}
	defer queryRows.Close()

	// 收集所有 rows
	allRows := make([]*sst.Row, 0)
	for queryRows.Next() {
		row := queryRows.Row()
		sstRow := &sst.Row{
			Seq:  row.Data()["_seq"].(int64),
			Time: row.Data()["_time"].(int64),
			Data: make(map[string]any),
		}
		for k, v := range row.Data() {
			if k != "_seq" && k != "_time" {
				sstRow.Data[k] = v
			}
		}
		allRows = append(allRows, sstRow)
	}

	// 计算分页
	totalRows := int64(len(allRows))
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > int(totalRows) {
		end = int(totalRows)
	}

	// 获取当前页数据
	rows := make([]*sst.Row, 0, pageSize)
	if offset < int(totalRows) {
		rows = allRows[offset:end]
	}

	// 构造 TableDataResponse
	const maxStringLength = 100
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		rowData := make(map[string]any)
		rowData["_seq"] = row.Seq
		rowData["_time"] = row.Time
		for k, v := range row.Data {
			field, err := tableSchema.GetField(k)
			if err == nil && field.Type == srdb.FieldTypeString {
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
	}

	tableData := TableDataResponse{
		Data:       data,
		Page:       int64(page),
		PageSize:   int64(pageSize),
		TotalRows:  totalRows,
		TotalPages: (totalRows + int64(pageSize) - 1) / int64(pageSize),
	}

	html := renderDataViewHTML(tableName, schemaInfo, tableData)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// renderManifestHTML 渲染 Manifest 视图 HTML
func (ui *WebUI) renderManifestHTML(w http.ResponseWriter, r *http.Request, tableName string, table *srdb.Table) {
	engine := table.GetEngine()
	versionSet := engine.GetVersionSet()
	version := versionSet.GetCurrent()

	// 获取 Compaction Manager 和 Picker
	compactionMgr := engine.GetCompactionManager()
	picker := compactionMgr.GetPicker()

	levels := make([]LevelInfo, 0)
	for level := 0; level < 7; level++ {
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

		score := 0.0
		if len(files) > 0 {
			score = picker.GetLevelScore(version, level)
		}

		levels = append(levels, LevelInfo{
			Level:     level,
			FileCount: len(files),
			TotalSize: totalSize,
			Score:     score,
			Files:     fileInfos,
		})
	}

	stats := compactionMgr.GetStats()

	manifest := ManifestResponse{
		Levels:          levels,
		NextFileNumber:  versionSet.GetNextFileNumber(),
		LastSequence:    versionSet.GetLastSequence(),
		CompactionStats: stats,
	}

	html := renderManifestViewHTML(tableName, manifest)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
