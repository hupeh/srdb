package srdb

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.tczkiot.com/srdb/sst"
)

type Fieldset interface {
	Get(key string) (field Field, value any, err error)
}

// mapFieldset 实现 Fieldset 接口，包装 map[string]any 和 Schema
type mapFieldset struct {
	data   map[string]any
	schema *Schema
}

func newMapFieldset(data map[string]any, schema *Schema) *mapFieldset {
	return &mapFieldset{
		data:   data,
		schema: schema,
	}
}

func (m *mapFieldset) Get(key string) (Field, any, error) {
	value, exists := m.data[key]
	if !exists {
		return Field{}, nil, fmt.Errorf("field %s not found", key)
	}

	// 如果有 schema，返回字段定义
	if m.schema != nil {
		field, err := m.schema.GetField(key)
		if err != nil {
			// 字段在 schema 中不存在，返回默认 Field
			return Field{Name: key}, value, nil
		}
		return *field, value, nil
	}

	// 没有 schema，返回默认 Field
	return Field{Name: key}, value, nil
}

type Expr interface {
	Match(fs Fieldset) bool
}

type Neginative struct {
	expr Expr
}

func (n Neginative) Match(fs Fieldset) bool {
	if n.expr == nil {
		return true
	}
	return !n.expr.Match(fs)
}

func Not(expr Expr) Expr {
	return Neginative{expr}
}

type compare struct {
	field string
	op    string
	right any
}

func (c compare) Match(fs Fieldset) bool {
	_, value, err := fs.Get(c.field)
	if err != nil {
		// 字段不存在
		return c.op == "IS NULL"
	}

	// 处理 NULL 检查
	if c.op == "IS NULL" {
		return value == nil
	}
	if c.op == "IS NOT NULL" {
		return value != nil
	}

	// 如果值为 nil，其他操作都返回 false
	if value == nil {
		return false
	}

	switch c.op {
	case "=":
		return compareEqual(value, c.right)
	case "!=":
		return !compareEqual(value, c.right)
	case "<":
		return compareLess(value, c.right)
	case ">":
		return compareGreater(value, c.right)
	case "<=":
		return compareLess(value, c.right) || compareEqual(value, c.right)
	case ">=":
		return compareGreater(value, c.right) || compareEqual(value, c.right)
	case "IN":
		if list, ok := c.right.([]any); ok {
			for _, item := range list {
				if compareEqual(value, item) {
					return true
				}
			}
		}
		return false
	case "NOT IN":
		if list, ok := c.right.([]any); ok {
			for _, item := range list {
				if compareEqual(value, item) {
					return false
				}
			}
			return true
		}
		return false
	case "BETWEEN":
		if list, ok := c.right.([]any); ok && len(list) == 2 {
			return (compareGreater(value, list[0]) || compareEqual(value, list[0])) &&
				(compareLess(value, list[1]) || compareEqual(value, list[1]))
		}
		return false
	case "NOT BETWEEN":
		if list, ok := c.right.([]any); ok && len(list) == 2 {
			return !((compareGreater(value, list[0]) || compareEqual(value, list[0])) &&
				(compareLess(value, list[1]) || compareEqual(value, list[1])))
		}
		return false
	case "CONTAINS":
		if str, ok := value.(string); ok {
			if pattern, ok := c.right.(string); ok {
				return strings.Contains(str, pattern)
			}
		}
		return false
	case "NOT CONTAINS":
		if str, ok := value.(string); ok {
			if pattern, ok := c.right.(string); ok {
				return !strings.Contains(str, pattern)
			}
		}
		return false
	case "STARTS WITH":
		if str, ok := value.(string); ok {
			if pattern, ok := c.right.(string); ok {
				return strings.HasPrefix(str, pattern)
			}
		}
		return false
	case "NOT STARTS WITH":
		if str, ok := value.(string); ok {
			if pattern, ok := c.right.(string); ok {
				return !strings.HasPrefix(str, pattern)
			}
		}
		return false
	case "ENDS WITH":
		if str, ok := value.(string); ok {
			if pattern, ok := c.right.(string); ok {
				return strings.HasSuffix(str, pattern)
			}
		}
		return false
	case "NOT ENDS WITH":
		if str, ok := value.(string); ok {
			if pattern, ok := c.right.(string); ok {
				return !strings.HasSuffix(str, pattern)
			}
		}
		return false
	}
	return false
}

// compareEqual 比较两个值是否相等
func compareEqual(left, right any) bool {
	// 处理数值类型的比较
	leftNum, leftIsNum := toFloat64(left)
	rightNum, rightIsNum := toFloat64(right)
	if leftIsNum && rightIsNum {
		return leftNum == rightNum
	}

	// 其他类型直接比较
	return left == right
}

// compareLess 比较 left < right
func compareLess(left, right any) bool {
	// 数值比较
	leftNum, leftIsNum := toFloat64(left)
	rightNum, rightIsNum := toFloat64(right)
	if leftIsNum && rightIsNum {
		return leftNum < rightNum
	}

	// 字符串比较
	if leftStr, ok := left.(string); ok {
		if rightStr, ok := right.(string); ok {
			return leftStr < rightStr
		}
	}

	return false
}

// compareGreater 比较 left > right
func compareGreater(left, right any) bool {
	// 数值比较
	leftNum, leftIsNum := toFloat64(left)
	rightNum, rightIsNum := toFloat64(right)
	if leftIsNum && rightIsNum {
		return leftNum > rightNum
	}

	// 字符串比较
	if leftStr, ok := left.(string); ok {
		if rightStr, ok := right.(string); ok {
			return leftStr > rightStr
		}
	}

	return false
}

// toFloat64 尝试将值转换为 float64
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint8:
		return float64(val), true
	default:
		return 0, false
	}
}

func Eq(field string, value any) Expr {
	return compare{field, "=", value}
}

func NotEq(field string, value any) Expr {
	return compare{field, "!=", value}
}

func Lt(field string, value any) Expr {
	return compare{field, "<", value}
}

func Gt(field string, value any) Expr {
	return compare{field, ">", value}
}

func Lte(field string, value any) Expr {
	return compare{field, "<=", value}
}

func Gte(field string, value any) Expr {
	return compare{field, ">=", value}
}

func In(field string, values []any) Expr {
	return compare{field, "IN", values}
}

func NotIn(field string, values []any) Expr {
	return compare{field, "NOT IN", values}
}

func Between(field string, min, max any) Expr {
	return compare{field, "BETWEEN", []any{min, max}}
}

func NotBetween(field string, min, max any) Expr {
	return compare{field, "NOT BETWEEN", []any{min, max}}
}

func Contains(field string, pattern string) Expr {
	return compare{field, "CONTAINS", pattern}
}

func NotContains(field string, pattern string) Expr {
	return compare{field, "NOT CONTAINS", pattern}
}

func StartsWith(field string, prefix string) Expr {
	return compare{field, "STARTS WITH", prefix}
}

func NotStartsWith(field string, prefix string) Expr {
	return compare{field, "NOT STARTS WITH", prefix}
}

func EndsWith(field string, suffix string) Expr {
	return compare{field, "ENDS WITH", suffix}
}

func NotEndsWith(field string, suffix string) Expr {
	return compare{field, "NOT ENDS WITH", suffix}
}

func IsNull(field string) Expr {
	return compare{field, "IS NULL", nil}
}

func NotNull(field string) Expr {
	return compare{field, "IS NOT NULL", nil}
}

type group struct {
	exprs []Expr
	and   bool
}

func (g group) Match(fs Fieldset) bool {
	for _, expr := range g.exprs {
		matched := expr.Match(fs)
		if matched && !g.and {
			return true
		}
		if !matched && g.and {
			return false
		}
	}
	return true
}

func And(exprs ...Expr) Expr {
	return group{exprs, true}
}

func Or(exprs ...Expr) Expr {
	return group{exprs, false}
}

type QueryBuilder struct {
	conds  []Expr
	fields []string // 要选择的字段，nil 表示选择所有字段
	engine *Engine
}

func newQueryBuilder(engine *Engine) *QueryBuilder {
	return &QueryBuilder{
		engine: engine,
	}
}

func (qb *QueryBuilder) where(expr Expr) *QueryBuilder {
	qb.conds = append(qb.conds, expr)
	return qb
}

// Match 检查数据是否匹配所有条件
func (qb *QueryBuilder) Match(data map[string]any) bool {
	if len(qb.conds) == 0 {
		return true
	}

	fs := newMapFieldset(data, qb.engine.schema)
	for _, cond := range qb.conds {
		if !cond.Match(fs) {
			return false
		}
	}
	return true
}

// Select 指定要选择的字段，如果不调用则返回所有字段
func (qb *QueryBuilder) Select(fields ...string) *QueryBuilder {
	qb.fields = fields
	return qb
}

func (qb *QueryBuilder) Where(exprs ...Expr) *QueryBuilder {
	return qb.where(And(exprs...))
}

func (qb *QueryBuilder) Eq(field string, value any) *QueryBuilder {
	return qb.where(Eq(field, value))
}

func (qb *QueryBuilder) NotEq(field string, value any) *QueryBuilder {
	return qb.where(NotEq(field, value))
}

func (qb *QueryBuilder) Lt(field string, value any) *QueryBuilder {
	return qb.where(Lt(field, value))
}

func (qb *QueryBuilder) Gt(field string, value any) *QueryBuilder {
	return qb.where(Gt(field, value))
}

func (qb *QueryBuilder) Lte(field string, value any) *QueryBuilder {
	return qb.where(Lte(field, value))
}

func (qb *QueryBuilder) Gte(field string, value any) *QueryBuilder {
	return qb.where(Gte(field, value))
}

func (qb *QueryBuilder) In(field string, values []any) *QueryBuilder {
	return qb.where(In(field, values))
}

func (qb *QueryBuilder) NotIn(field string, values []any) *QueryBuilder {
	return qb.where(NotIn(field, values))
}

func (qb *QueryBuilder) Between(field string, start, end any) *QueryBuilder {
	return qb.where(Between(field, start, end))
}

func (qb *QueryBuilder) NotBetween(field string, start, end any) *QueryBuilder {
	return qb.where(Not(Between(field, start, end)))
}

func (qb *QueryBuilder) Contains(field string, pattern string) *QueryBuilder {
	return qb.where(Contains(field, pattern))
}

func (qb *QueryBuilder) NotContains(field string, pattern string) *QueryBuilder {
	return qb.where(NotContains(field, pattern))
}

func (qb *QueryBuilder) StartsWith(field string, pattern string) *QueryBuilder {
	return qb.where(StartsWith(field, pattern))
}

func (qb *QueryBuilder) NotStartsWith(field string, pattern string) *QueryBuilder {
	return qb.where(NotStartsWith(field, pattern))
}

func (qb *QueryBuilder) EndsWith(field string, pattern string) *QueryBuilder {
	return qb.where(EndsWith(field, pattern))
}

func (qb *QueryBuilder) NotEndsWith(field string, pattern string) *QueryBuilder {
	return qb.where(NotEndsWith(field, pattern))
}

func (qb *QueryBuilder) IsNull(field string) *QueryBuilder {
	return qb.where(IsNull(field))
}

func (qb *QueryBuilder) NotNull(field string) *QueryBuilder {
	return qb.where(NotNull(field))
}

// Rows 返回所有匹配的数据（游标模式 - 惰性加载）
func (qb *QueryBuilder) Rows() (*Rows, error) {
	if qb.engine == nil {
		return nil, fmt.Errorf("engine is nil")
	}

	rows := &Rows{
		schema:  qb.engine.schema,
		fields:  qb.fields,
		qb:      qb,
		engine:  qb.engine,
		visited: make(map[int64]bool),
	}

	// 初始化 Active MemTable 迭代器
	activeMemTable := qb.engine.memtableManager.GetActive()
	if activeMemTable != nil {
		activeKeys := activeMemTable.Keys()
		if len(activeKeys) > 0 {
			rows.memIterator = newMemtableIterator(activeKeys)
		}
	}

	// 准备 Immutable MemTables（延迟初始化）
	rows.immutableIndex = 0

	// 初始化 SST 文件 readers
	sstReaders := qb.engine.sstManager.GetReaders()
	for _, reader := range sstReaders {
		// 获取文件中实际存在的 key 列表（已排序）
		// 这比 minKey→maxKey 逐个尝试高效 100-1000 倍（对于稀疏 key）
		keys := reader.GetAllKeys()
		rows.sstReaders = append(rows.sstReaders, &sstReader{
			reader: reader,
			keys:   keys,
			index:  0,
		})
	}

	return rows, nil
}

// First 返回第一个匹配的数据
func (qb *QueryBuilder) First() (*Row, error) {
	rows, err := qb.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows.First()
}

// Last 返回最后一个匹配的数据
func (qb *QueryBuilder) Last() (*Row, error) {
	rows, err := qb.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows.Last()
}

// Scan 扫描结果到指定的变量
func (qb *QueryBuilder) Scan(value any) error {
	rows, err := qb.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	return rows.Scan(value)
}

type Row struct {
	schema *Schema
	fields []string // 要选择的字段，nil 表示选择所有字段
	inner  *sst.Row
}

// Data 获取行数据（根据 Select 过滤字段）
func (r *Row) Data() map[string]any {
	if r.inner == nil {
		return nil
	}

	// 如果没有指定字段，返回所有数据（包括 _seq 和 _time）
	if r.fields == nil || len(r.fields) == 0 {
		result := make(map[string]any)
		result["_seq"] = r.inner.Seq
		result["_time"] = r.inner.Time
		for k, v := range r.inner.Data {
			result[k] = v
		}
		return result
	}

	// 根据指定的字段过滤
	result := make(map[string]any)
	for _, field := range r.fields {
		if field == "_seq" {
			result["_seq"] = r.inner.Seq
		} else if field == "_time" {
			result["_time"] = r.inner.Time
		} else if val, ok := r.inner.Data[field]; ok {
			result[field] = val
		}
	}
	return result
}

// Seq 获取行序列号
func (r *Row) Seq() int64 {
	if r.inner == nil {
		return 0
	}
	return r.inner.Seq
}

// Scan 扫描行数据到指定的变量
func (r *Row) Scan(value any) error {
	if r.inner == nil {
		return fmt.Errorf("row is nil")
	}

	data, err := json.Marshal(r.inner.Data)
	if err != nil {
		return fmt.Errorf("marshal row data: %w", err)
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return fmt.Errorf("unmarshal to target: %w", err)
	}

	return nil
}

// Rows 游标模式的结果集（惰性加载）
type Rows struct {
	schema *Schema
	fields []string // 要选择的字段，nil 表示选择所有字段
	qb     *QueryBuilder
	engine *Engine

	// 迭代状态
	currentRow *Row
	err        error
	closed     bool
	visited    map[int64]bool // 已访问的 seq，用于去重

	// 数据源迭代器
	memIterator       *memtableIterator
	immutableIndex    int
	immutableIterator *memtableIterator
	sstIndex          int
	sstReaders        []*sstReader

	// 缓存模式（用于 Collect/Data 等方法）
	cached      bool
	cachedRows  []*sst.Row
	cachedIndex int // 缓存模式下的迭代位置
}

// memtableIterator 包装 MemTable 的迭代器
type memtableIterator struct {
	keys  []int64
	index int
}

func newMemtableIterator(keys []int64) *memtableIterator {
	return &memtableIterator{
		keys:  keys,
		index: -1,
	}
}

func (m *memtableIterator) next() (int64, bool) {
	m.index++
	if m.index >= len(m.keys) {
		return 0, false
	}
	return m.keys[m.index], true
}

// sstReader 包装 SST Reader 的迭代状态
type sstReader struct {
	reader any     // 实际的 SST reader
	keys   []int64 // 文件中实际存在的 key 列表（已排序）
	index  int     // 当前迭代位置
}

// Next 移动到下一行，返回是否还有数据
func (r *Rows) Next() bool {
	if r.closed {
		return false
	}
	if r.err != nil {
		return false
	}

	// 如果是缓存模式，使用缓存的数据
	if r.cached {
		return r.nextFromCache()
	}

	// 惰性模式：从数据源读取
	return r.next()
}

// next 从数据源读取下一条匹配的记录（惰性加载的核心逻辑）
func (r *Rows) next() bool {
	for {
		// 1. 尝试从 Active MemTable 获取
		if r.memIterator != nil {
			if seq, ok := r.memIterator.next(); ok {
				if !r.visited[seq] {
					row, err := r.engine.Get(seq)
					if err == nil && r.qb.Match(row.Data) {
						r.visited[seq] = true
						r.currentRow = &Row{schema: r.schema, fields: r.fields, inner: row}
						return true
					}
					r.visited[seq] = true
				}
				continue
			}
			// Active MemTable 迭代完成
			r.memIterator = nil
		}

		// 2. 尝试从 Immutable MemTables 获取
		if r.immutableIterator != nil {
			if seq, ok := r.immutableIterator.next(); ok {
				if !r.visited[seq] {
					row, err := r.engine.Get(seq)
					if err == nil && r.qb.Match(row.Data) {
						r.visited[seq] = true
						r.currentRow = &Row{schema: r.schema, fields: r.fields, inner: row}
						return true
					}
					r.visited[seq] = true
				}
				continue
			}
			// 当前 Immutable 迭代完成，移到下一个
			r.immutableIterator = nil
			r.immutableIndex++
		}

		// 检查是否有更多 Immutable MemTables
		if r.immutableIterator == nil && r.immutableIndex < len(r.engine.memtableManager.GetImmutables()) {
			immutables := r.engine.memtableManager.GetImmutables()
			if r.immutableIndex < len(immutables) {
				r.immutableIterator = newMemtableIterator(immutables[r.immutableIndex].MemTable.Keys())
				continue
			}
		}

		// 3. 尝试从 SST 文件获取
		if r.sstIndex < len(r.sstReaders) {
			sstReader := r.sstReaders[r.sstIndex]
			// 遍历文件中实际存在的 key（不是 minKey→maxKey 范围）
			for sstReader.index < len(sstReader.keys) {
				seq := sstReader.keys[sstReader.index]
				sstReader.index++

				if !r.visited[seq] {
					row, err := r.engine.Get(seq)
					if err == nil && r.qb.Match(row.Data) {
						r.visited[seq] = true
						r.currentRow = &Row{schema: r.schema, fields: r.fields, inner: row}
						return true
					}
					r.visited[seq] = true
				}
			}
			// 当前 SST 文件迭代完成，移到下一个
			r.sstIndex++
			continue
		}

		// 所有数据源都迭代完成
		return false
	}
}

// nextFromCache 从缓存中获取下一条记录
func (r *Rows) nextFromCache() bool {
	r.cachedIndex++
	if r.cachedIndex >= len(r.cachedRows) {
		return false
	}
	r.currentRow = &Row{schema: r.schema, fields: r.fields, inner: r.cachedRows[r.cachedIndex]}
	return true
}

// Row 获取当前行
func (r *Rows) Row() *Row {
	return r.currentRow
}

// Err 返回错误
func (r *Rows) Err() error {
	return r.err
}

// Close 关闭游标
func (r *Rows) Close() error {
	r.closed = true
	return nil
}

// ensureCached 确保所有数据已被加载到缓存
func (r *Rows) ensureCached() {
	if r.cached {
		return
	}

	// 使用私有的 next() 方法直接从数据源读取所有剩余数据
	// 这样避免了与 Next() 的循环调用问题
	// 注意：如果之前已经调用过 Next()，部分数据已经被消耗，只能缓存剩余数据
	for r.next() {
		if r.currentRow != nil && r.currentRow.inner != nil {
			r.cachedRows = append(r.cachedRows, r.currentRow.inner)
		}
	}

	// 标记为已缓存，重置迭代位置
	r.cached = true
	r.cachedIndex = -1
}

// Len 返回总行数（需要完全扫描）
func (r *Rows) Len() int {
	r.ensureCached()
	return len(r.cachedRows)
}

// Collect 收集所有结果到切片
func (r *Rows) Collect() []map[string]any {
	r.ensureCached()
	var results []map[string]any
	for _, row := range r.cachedRows {
		results = append(results, row.Data)
	}
	return results
}

// Data 获取所有行的数据（向后兼容）
func (r *Rows) Data() []map[string]any {
	return r.Collect()
}

// Scan 扫描所有行数据到指定的变量
func (r *Rows) Scan(value any) error {
	data, err := json.Marshal(r.Collect())
	if err != nil {
		return fmt.Errorf("marshal rows data: %w", err)
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return fmt.Errorf("unmarshal to target: %w", err)
	}

	return nil
}

// First 获取第一行
func (r *Rows) First() (*Row, error) {
	// 尝试获取第一条记录（不使用缓存）
	if r.Next() {
		return r.currentRow, nil
	}
	return nil, fmt.Errorf("no rows")
}

// Last 获取最后一行
func (r *Rows) Last() (*Row, error) {
	r.ensureCached()
	if len(r.cachedRows) == 0 {
		return nil, fmt.Errorf("no rows")
	}
	return &Row{schema: r.schema, fields: r.fields, inner: r.cachedRows[len(r.cachedRows)-1]}, nil
}

// Count 返回总行数（别名）
func (r *Rows) Count() int {
	return r.Len()
}
