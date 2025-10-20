# SRDB 复杂类型示例

这个示例演示了 SRDB 支持的所有 **21 种数据类型**的使用方法，包括：

- 结构体自动生成 Schema
- 所有基本类型和特殊类型的插入与查询
- 边界值测试
- 索引查询
- 分页查询
- 复杂类型（Object/Array）的序列化

## 📊 支持的 21 种类型

### 基本类型 (14种)

| 分类 | 类型 | Go 类型 | 说明 |
|------|------|---------|------|
| **字符串** | String | `string` | UTF-8 字符串 |
| **有符号整数** | Int | `int` | 平台相关 |
|  | Int8 | `int8` | -128 ~ 127 |
|  | Int16 | `int16` | -32768 ~ 32767 |
|  | Int32 | `int32` | -2^31 ~ 2^31-1 |
|  | Int64 | `int64` | -2^63 ~ 2^63-1 |
| **无符号整数** | Uint | `uint` | 平台相关 |
|  | Uint8 | `uint8` | 0 ~ 255 |
|  | Uint16 | `uint16` | 0 ~ 65535 |
|  | Uint32 | `uint32` | 0 ~ 4294967295 |
|  | Uint64 | `uint64` | 0 ~ 2^64-1 |
| **浮点数** | Float32 | `float32` | 单精度 |
|  | Float64 | `float64` | 双精度 |
| **布尔** | Bool | `bool` | true/false |

### 特殊类型 (5种)

| 类型 | Go 类型 | 说明 | 使用场景 |
|------|---------|------|----------|
| Byte | `byte` | 0-255（独立类型） | 状态码、百分比、标志位 |
| Rune | `rune` | Unicode 字符（独立类型） | 等级、分类字符 |
| Decimal | `decimal.Decimal` | 高精度十进制 | 金融计算、货币金额 |
| Time | `time.Time` | 时间戳 | 日期时间 |
| Duration | `time.Duration` | 时长 | 超时、间隔、运行时长 |

### 复杂类型 (2种)

| 类型 | Go 类型 | 说明 |
|------|---------|------|
| Object | `map[string]any`, `struct{}` | JSON 编码存储 |
| Array | `[]any`, `[]string`, `[]int` 等 | JSON 编码存储 |

## 🚀 快速开始

### 1. 构建并运行

```bash
cd examples/complex
go run main.go
```

### 2. 使用参数

```bash
# 指定数据目录
go run main.go --dir ./mydata

# 清理数据并重新生成
go run main.go --clean

# 指定目录并清理
go run main.go --dir ./mydata --clean
```

### 3. 构建可执行文件

```bash
go build -o complex
./complex --clean
```

## 📝 代码结构

### 结构体定义

```go
type DeviceRecord struct {
    // 字符串
    DeviceID string `srdb:"device_id;indexed;comment:设备ID"`
    Name     string `srdb:"name;comment:设备名称"`

    // 有符号整数 (5种)
    Signal     int   `srdb:"signal;comment:信号强度"`
    ErrorCode  int8  `srdb:"error_code;comment:错误码"`
    DeltaTemp  int16 `srdb:"delta_temp;comment:温差"`
    RecordNum  int32 `srdb:"record_num;comment:记录号"`
    TotalBytes int64 `srdb:"total_bytes;comment:总字节数"`

    // 无符号整数 (5种)
    Flags      uint   `srdb:"flags;comment:标志位"`
    Status     uint8  `srdb:"status;comment:状态"`
    Port       uint16 `srdb:"port;comment:端口"`
    SessionID  uint32 `srdb:"session_id;comment:会话ID"`
    Timestamp  uint64 `srdb:"timestamp;comment:时间戳"`

    // 浮点数 (2种)
    TempValue float32 `srdb:"temp_value;comment:温度值"`
    Latitude  float64 `srdb:"latitude;comment:纬度"`
    Longitude float64 `srdb:"longitude;comment:经度"`

    // 布尔
    IsOnline bool `srdb:"is_online;indexed;comment:在线状态"`

    // 特殊类型
    BatteryPct byte            `srdb:"battery_pct;comment:电量百分比"`
    Level      rune            `srdb:"level;comment:等级字符"`
    Price      decimal.Decimal `srdb:"price;comment:价格"`
    CreatedAt  time.Time       `srdb:"created_at;comment:创建时间"`
    RunTime    time.Duration   `srdb:"run_time;comment:运行时长"`

    // 复杂类型
    Settings map[string]any `srdb:"settings;comment:设置"`
    Tags     []string       `srdb:"tags;comment:标签列表"`
}
```

### 核心步骤

1. **从结构体生成 Schema**
   ```go
   fields, err := srdb.StructToFields(DeviceRecord{})
   ```

2. **创建表**
   ```go
   table, err := srdb.OpenTable(&srdb.TableOptions{
       Dir:    "./data",
       Name:   "devices",
       Fields: fields,
   })
   ```

3. **插入数据（使用 map）**
   ```go
   device := map[string]any{
       "device_id":   "IOT-2025-0001",
       "name":        "智能环境监测站",
       "signal":      -55,
       "error_code":  int8(0),
       "port":        uint16(8080),
       "temp_value":  float32(23.5),
       "is_online":   true,
       "battery_pct": byte(85),
       "level":       rune('S'),
       "price":       decimal.NewFromFloat(999.99),
       "created_at":  time.Now(),
       "run_time":    3*time.Hour + 25*time.Minute,
       "settings":    map[string]any{"interval": 60},
       "tags":        []string{"indoor", "hvac"},
   }
   table.Insert(device)
   ```

4. **查询数据**
   ```go
   rows, err := table.Query().OrderBy("_seq").Rows()
   for rows.Next() {
       row := rows.Row()
       data := row.Data()
       // 处理数据...
   }
   ```

5. **索引查询**
   ```go
   table.BuildIndexes()
   rows, _ := table.Query().Eq("device_id", "IOT-2025-0001").Rows()
   ```

6. **分页查询**
   ```go
   rows, total, err := table.Query().OrderBy("_seq").Paginate(1, 10)
   ```

## 🎯 示例输出

运行程序后，你会看到漂亮的表格化输出：

```
╔═══════════════ 设备记录 #1 (seq=1) ═══════════════╗
║ ID: IOT-2025-0001                                   ║
║ 名称: 智能环境监测站                                 ║
╟─────────────────── 整数类型 ────────────────────────╢
║ Signal(int):    -55                                 ║
║ ErrorCode(i8):  0                                   ║
║ DeltaTemp(i16): 150                                 ║
║ RecordNum(i32): 12345                               ║
║ TotalBytes(i64):1073741824                          ║
║ Flags(uint):    0xF                                 ║
║ Status(u8):     200                                 ║
║ Port(u16):      8080                                ║
║ SessionID(u32): 987654321                           ║
║ Timestamp(u64): 1760210986                          ║
╟───────────────── 浮点/布尔 ──────────────────────╢
║ Temperature(f32): 23.50°C                           ║
║ 坐标(f64): (39.904200, 116.407396)                  ║
║ Online(bool): true                                  ║
╟───────────────── 特殊类型 ──────────────────────╢
║ Battery(byte): 85%                                  ║
║ Level(rune):   S                                    ║
║ Price(decimal): ¥999.99                             ║
║ CreatedAt(time): 2025-10-12 03:29:46               ║
║ RunTime(duration): 3h25m0s                         ║
╟───────────────── 复杂类型 ──────────────────────╢
║ Settings(object): 4 项配置                          ║
║   • report_interval      = 60                      ║
║   • sample_rate          = 100                     ║
║   • auto_calibrate       = true                    ║
║   • threshold            = 25                      ║
║ Tags(array): 4 个标签                              ║
║   [indoor hvac monitoring enterprise]               ║
╚═════════════════════════════════════════════════════╝
```

## 💡 关键特性

### 1. 边界值测试

示例包含各类型的边界值测试：

```go
device := map[string]any{
    "error_code":  int8(127),              // int8 最大值
    "delta_temp":  int16(-32768),          // int16 最小值
    "record_num":  int32(2147483647),      // int32 最大值
    "total_bytes": int64(9223372036854775807), // int64 最大值
    "status":      uint8(255),             // uint8 最大值
    "port":        uint16(65535),          // uint16 最大值
}
```

### 2. 索引查询优化

使用索引加速查询：

```go
// 结构体中标记索引
DeviceID string `srdb:"device_id;indexed"`
IsOnline bool   `srdb:"is_online;indexed"`

// 构建索引
table.BuildIndexes()

// 使用索引查询
rows, _ := table.Query().Eq("device_id", "IOT-2025-0001").Rows()
rows, _ := table.Query().Eq("is_online", true).Rows()
```

### 3. 分页查询

支持返回总数的分页：

```go
rows, total, err := table.Query().OrderBy("_seq").Paginate(1, 2)
fmt.Printf("总记录数: %d\n", total)
```

### 4. 复杂类型序列化

Object 和 Array 自动序列化为 JSON：

```go
// Object: map[string]any
"settings": map[string]any{
    "report_interval": 60,
    "sample_rate":     100,
    "auto_calibrate":  true,
}

// Array: []string
"tags": []string{"indoor", "hvac", "monitoring"}

// 查询时自动反序列化
settings := data["settings"].(map[string]any)
tags := data["tags"].([]any)
```

## 📚 类型选择最佳实践

### 整数类型

```go
// ❌ 不推荐：盲目使用 int64
Port   int64  // 端口号 0-65535，浪费 6 字节
Status int64  // 状态码 0-255，浪费 7 字节

// ✅ 推荐：根据数据范围选择
Port   uint16 // 0-65535，2 字节
Status uint8  // 0-255，1 字节
```

### 浮点数类型

```go
// ❌ 不推荐
Temperature float64 // 温度用单精度足够

// ✅ 推荐
Temperature float32 // -40°C ~ 125°C，单精度足够
Latitude    float64 // 地理坐标需要双精度
```

### 特殊类型使用

```go
// Byte: 百分比、状态码
BatteryLevel byte   // 0-100

// Rune: 单字符等级
Grade        rune   // 'S', 'A', 'B', 'C'

// Decimal: 金融计算
Price        decimal.Decimal  // 避免浮点精度问题

// Time: 时间戳
CreatedAt    time.Time

// Duration: 时长
Timeout      time.Duration
```

## 🔧 依赖

```go
import (
    "github.com/hupeh/srdb"
    "github.com/shopspring/decimal"
)
```

确保已安装 `decimal` 包：

```bash
go get github.com/shopspring/decimal
```

## 📖 相关文档

- [SRDB 主文档](../../README.md)
- [CLAUDE.md - 开发指南](../../CLAUDE.md)
- [WebUI 示例](../webui/)

## 🤝 贡献

如果你有更好的示例或发现问题，欢迎提交 Issue 或 Pull Request。

## 📄 许可证

MIT License - 详见项目根目录 LICENSE 文件
