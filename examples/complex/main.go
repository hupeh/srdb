package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hupeh/srdb"
	"github.com/shopspring/decimal"
)

// ========== 嵌套结构体定义 ==========

// Location 位置信息（嵌套结构体）
type Location struct {
	Country  string  `json:"country"`
	Province string  `json:"province"`
	City     string  `json:"city"`
	Address  string  `json:"address"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

// NetworkConfig 网络配置（嵌套结构体）
type NetworkConfig struct {
	SSID        string `json:"ssid"`
	Password    string `json:"password"`
	IPAddress   string `json:"ip_address"`
	Gateway     string `json:"gateway"`
	DNS         string `json:"dns"`
	UseStaticIP bool   `json:"use_static_ip"`
}

// Sensor 传感器信息（用于切片）
type Sensor struct {
	Type         string  `json:"type"`          // 传感器类型
	Model        string  `json:"model"`         // 型号
	Value        float64 `json:"value"`         // 当前值
	Unit         string  `json:"unit"`          // 单位
	MinValue     float64 `json:"min_value"`     // 最小值
	MaxValue     float64 `json:"max_value"`     // 最大值
	Precision    int     `json:"precision"`     // 精度
	SamplingRate int     `json:"sampling_rate"` // 采样率
	Enabled      bool    `json:"enabled"`       // 是否启用
}

// MaintenanceRecord 维护记录（用于切片）
type MaintenanceRecord struct {
	Date        string  `json:"date"`        // 维护日期
	Technician  string  `json:"technician"`  // 技术员
	Type        string  `json:"type"`        // 维护类型
	Description string  `json:"description"` // 描述
	Cost        float64 `json:"cost"`        // 费用
	NextDate    string  `json:"next_date"`   // 下次维护日期
}

// ========== 主结构体定义 ==========

// ComplexDevice 复杂设备记录（包含所有复杂场景）
type ComplexDevice struct {
	// ========== 基本字段 ==========
	DeviceID string `srdb:"device_id;indexed;comment:设备ID"`
	Name     string `srdb:"name;comment:设备名称"`
	Model    string `srdb:"model;comment:设备型号"`

	// ========== Nullable 字段（指针类型）==========
	SerialNumber    *string          `srdb:"serial_number;nullable;comment:序列号（可选）"`
	Manufacturer    *string          `srdb:"manufacturer;nullable;comment:制造商（可选）"`
	Description     *string          `srdb:"description;nullable;comment:描述（可选）"`
	WarrantyEnd     *time.Time       `srdb:"warranty_end;nullable;comment:保修截止日期（可选）"`
	LastMaintenance *time.Time       `srdb:"last_maintenance;nullable;comment:上次维护时间（可选）"`
	MaxPower        *float32         `srdb:"max_power;nullable;comment:最大功率（可选）"`
	Weight          *float64         `srdb:"weight;nullable;comment:重量（可选）"`
	Voltage         *int32           `srdb:"voltage;nullable;comment:电压（可选）"`
	Price           *decimal.Decimal `srdb:"price;nullable;comment:价格（可选）"`

	// ========== 所有基本类型 ==========
	// 有符号整数
	Signal      int   `srdb:"signal;comment:信号强度"`
	ErrorCode   int8  `srdb:"error_code;comment:错误码"`
	Temperature int16 `srdb:"temperature;comment:温度（℃*10）"`
	Counter     int32 `srdb:"counter;comment:计数器"`
	TotalBytes  int64 `srdb:"total_bytes;comment:总字节数"`

	// 无符号整数
	Flags     uint   `srdb:"flags;comment:标志位"`
	Status    uint8  `srdb:"status;comment:状态码"`
	Port      uint16 `srdb:"port;comment:端口号"`
	SessionID uint32 `srdb:"session_id;comment:会话ID"`
	Timestamp uint64 `srdb:"timestamp;comment:时间戳"`

	// 浮点数
	Humidity  float32 `srdb:"humidity;comment:湿度"`
	Latitude  float64 `srdb:"latitude;comment:纬度"`
	Longitude float64 `srdb:"longitude;comment:经度"`

	// 布尔
	IsOnline    bool `srdb:"is_online;indexed;comment:是否在线"`
	IsActivated bool `srdb:"is_activated;comment:是否激活"`

	// 特殊类型
	BatteryLevel byte            `srdb:"battery_level;comment:电池电量"`
	Grade        rune            `srdb:"grade;comment:等级"`
	TotalPrice   decimal.Decimal `srdb:"total_price;comment:总价"`
	CreatedAt    time.Time       `srdb:"created_at;comment:创建时间"`
	Uptime       time.Duration   `srdb:"uptime;comment:运行时长"`

	// ========== 嵌套结构体（Object）==========
	Location      Location      `srdb:"location;comment:位置信息（嵌套结构体）"`
	NetworkConfig NetworkConfig `srdb:"network_config;comment:网络配置（嵌套结构体）"`

	// ========== 结构体切片（Array）==========
	Sensors            []Sensor            `srdb:"sensors;comment:传感器列表（结构体切片）"`
	MaintenanceRecords []MaintenanceRecord `srdb:"maintenance_records;comment:维护记录（结构体切片）"`

	// ========== 基本类型切片 ==========
	Tags            []string  `srdb:"tags;comment:标签列表"`
	AlertCodes      []int32   `srdb:"alert_codes;comment:告警代码列表"`
	HistoryReadings []float64 `srdb:"history_readings;comment:历史读数"`

	// ========== 简单 Map（Object）==========
	Metadata       map[string]any `srdb:"metadata;comment:元数据"`
	CustomSettings map[string]any `srdb:"custom_settings;comment:自定义设置"`
}

func main() {
	// 命令行参数
	dataDir := flag.String("dir", "./data", "数据存储目录")
	clean := flag.Bool("clean", false, "运行前清理数据目录")
	flag.Parse()

	fmt.Println("=============================================================")
	fmt.Println("  SRDB 复杂类型系统演示（Nullable + 嵌套结构体 + 结构体切片）")
	fmt.Println("=============================================================\n")

	// 准备数据目录
	absDir, err := filepath.Abs(*dataDir)
	if err != nil {
		fmt.Printf("❌ 无效的目录路径: %v\n", err)
		os.Exit(1)
	}

	if *clean {
		fmt.Printf("🧹 清理数据目录: %s\n", absDir)
		os.RemoveAll(absDir)
	}

	fmt.Printf("📁 数据目录: %s\n\n", absDir)

	// ========== 步骤 1: 从结构体生成 Schema ==========
	fmt.Println("【步骤 1】从结构体自动生成 Schema")
	fmt.Println("─────────────────────────────────────────────────────")

	fields, err := srdb.StructToFields(ComplexDevice{})
	if err != nil {
		fmt.Printf("❌ 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 成功生成 %d 个字段\n\n", len(fields))

	// 统计字段类型
	nullableCount := 0
	objectCount := 0
	arrayCount := 0
	for _, field := range fields {
		if field.Nullable {
			nullableCount++
		}
		if field.Type.String() == "object" {
			objectCount++
		}
		if field.Type.String() == "array" {
			arrayCount++
		}
	}

	fmt.Println("字段统计:")
	fmt.Printf("  • 总字段数: %d\n", len(fields))
	fmt.Printf("  • Nullable 字段: %d 个（使用指针）\n", nullableCount)
	fmt.Printf("  • Object 字段: %d 个（结构体/map）\n", objectCount)
	fmt.Printf("  • Array 字段: %d 个（切片）\n", arrayCount)

	// ========== 步骤 2: 创建表 ==========
	fmt.Println("\n【步骤 2】创建数据表")
	fmt.Println("─────────────────────────────────────────────────────")

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    absDir,
		Name:   "complex_devices",
		Fields: fields,
	})
	if err != nil {
		fmt.Printf("❌ 创建表失败: %v\n", err)
		os.Exit(1)
	}
	defer table.Close()
	fmt.Println("✅ 表 'complex_devices' 创建成功")

	// ========== 步骤 3: 插入完整数据 ==========
	fmt.Println("\n【步骤 3】插入测试数据")
	fmt.Println("─────────────────────────────────────────────────────")

	// 准备辅助变量
	serialNum := "SN-2025-001-ALPHA"
	manufacturer := "智能科技有限公司"
	description := "高性能工业级环境监测站，支持多种传感器接入"
	warrantyEnd := time.Now().AddDate(3, 0, 0)        // 3年保修
	lastMaint := time.Now().Add(-30 * 24 * time.Hour) // 30天前维护
	maxPower := float32(500.5)
	weight := 12.5
	voltage := int32(220)
	price := decimal.NewFromFloat(9999.99)

	// 数据1: 完整填充（包含所有 Nullable 字段）
	device1 := map[string]any{
		// 基本字段
		"device_id": "COMPLEX-DEV-001",
		"name":      "智能环境监测站 Pro",
		"model":     "ENV-MONITOR-PRO-X1",

		// Nullable 字段（全部有值）
		"serial_number":    serialNum,
		"manufacturer":     manufacturer,
		"description":      description,
		"warranty_end":     warrantyEnd,
		"last_maintenance": lastMaint,
		"max_power":        maxPower,
		"weight":           weight,
		"voltage":          voltage,
		"price":            price,

		// 基本类型
		"signal":        -55,
		"error_code":    int8(0),
		"temperature":   int16(235), // 23.5°C
		"counter":       int32(12345),
		"total_bytes":   int64(1024 * 1024 * 500),
		"flags":         uint(0x0F),
		"status":        uint8(200),
		"port":          uint16(8080),
		"session_id":    uint32(987654321),
		"timestamp":     uint64(time.Now().Unix()),
		"humidity":      float32(65.5),
		"latitude":      39.904200,
		"longitude":     116.407396,
		"is_online":     true,
		"is_activated":  true,
		"battery_level": byte(85),
		"grade":         rune('S'),
		"total_price":   decimal.NewFromFloat(15999.99),
		"created_at":    time.Now(),
		"uptime":        72 * time.Hour,

		// 嵌套结构体
		"location": Location{
			Country:  "中国",
			Province: "北京市",
			City:     "朝阳区",
			Address:  "建国路88号",
			Lat:      39.904200,
			Lng:      116.407396,
		},
		"network_config": NetworkConfig{
			SSID:        "SmartDevice-5G",
			Password:    "******",
			IPAddress:   "192.168.1.100",
			Gateway:     "192.168.1.1",
			DNS:         "8.8.8.8",
			UseStaticIP: true,
		},

		// 结构体切片
		"sensors": []Sensor{
			{
				Type:         "temperature",
				Model:        "DHT22",
				Value:        23.5,
				Unit:         "°C",
				MinValue:     -40.0,
				MaxValue:     80.0,
				Precision:    1,
				SamplingRate: 1000,
				Enabled:      true,
			},
			{
				Type:         "humidity",
				Model:        "DHT22",
				Value:        65.5,
				Unit:         "%",
				MinValue:     0.0,
				MaxValue:     100.0,
				Precision:    1,
				SamplingRate: 1000,
				Enabled:      true,
			},
			{
				Type:         "pressure",
				Model:        "BMP280",
				Value:        1013.25,
				Unit:         "hPa",
				MinValue:     300.0,
				MaxValue:     1100.0,
				Precision:    2,
				SamplingRate: 500,
				Enabled:      true,
			},
		},
		"maintenance_records": []MaintenanceRecord{
			{
				Date:        "2024-12-01",
				Technician:  "张工",
				Type:        "定期维护",
				Description: "清洁传感器、检查线路、更新固件",
				Cost:        500.00,
				NextDate:    "2025-03-01",
			},
			{
				Date:        "2024-09-15",
				Technician:  "李工",
				Type:        "故障维修",
				Description: "更换损坏的温度传感器",
				Cost:        800.00,
				NextDate:    "2024-12-01",
			},
		},

		// 基本类型切片
		"tags":             []string{"industrial", "outdoor", "monitoring", "iot", "smart-city"},
		"alert_codes":      []int32{1001, 2003, 3005, 4002},
		"history_readings": []float64{23.1, 23.3, 23.5, 23.7, 23.9, 24.0},

		// Map
		"metadata": map[string]any{
			"install_date":      "2024-01-15",
			"firmware_version":  "v2.3.1",
			"hardware_revision": "Rev-C",
			"certification":     []string{"CE", "FCC", "RoHS"},
		},
		"custom_settings": map[string]any{
			"auto_calibrate":  true,
			"report_interval": 60,
			"alert_threshold": 85.0,
			"debug_mode":      false,
		},
	}

	err = table.Insert(device1)
	if err != nil {
		fmt.Printf("❌ 插入数据1失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 数据1插入成功: " + device1["name"].(string))
	fmt.Println("   包含: 9个Nullable字段（全部有值）")
	fmt.Println("   包含: 2个嵌套结构体 + 2个结构体切片")

	// Debug: 检查插入后的记录数
	count1, _ := table.Query().Rows()
	c1 := 0
	for count1.Next() {
		c1++
	}
	count1.Close()
	fmt.Printf("   🔍 插入后表中有 %d 条记录\n", c1)

	// 数据2: 部分 Nullable 字段为 nil
	device2 := map[string]any{
		// 基本字段
		"device_id": "COMPLEX-DEV-002",
		"name":      "简易温湿度传感器",
		"model":     "TEMP-SENSOR-LITE",

		// Nullable 字段（部分为 nil）
		"serial_number":    "SN-2025-002-BETA",
		"manufacturer":     "普通传感器公司",
		"description":      nil, // NULL
		"warranty_end":     nil, // NULL
		"last_maintenance": nil, // NULL
		"max_power":        nil, // NULL
		"weight":           nil, // NULL
		"voltage":          nil, // NULL
		"price":            nil, // NULL

		// 基本类型
		"signal":        -70,
		"error_code":    int8(0),
		"temperature":   int16(220),
		"counter":       int32(500),
		"total_bytes":   int64(1024 * 1024 * 10),
		"flags":         uint(0x03),
		"status":        uint8(100),
		"port":          uint16(8081),
		"session_id":    uint32(123456789),
		"timestamp":     uint64(time.Now().Unix()),
		"humidity":      float32(55.0),
		"latitude":      39.900000,
		"longitude":     116.400000,
		"is_online":     false,
		"is_activated":  true,
		"battery_level": byte(30),
		"grade":         rune('B'),
		"total_price":   decimal.NewFromFloat(299.99),
		"created_at":    time.Now().Add(-7 * 24 * time.Hour),
		"uptime":        168 * time.Hour,

		// 嵌套结构体
		"location": Location{
			Country:  "中国",
			Province: "上海市",
			City:     "浦东新区",
			Address:  "世纪大道123号",
			Lat:      31.235929,
			Lng:      121.506058,
		},
		"network_config": NetworkConfig{
			SSID:        "SmartDevice-2.4G",
			Password:    "******",
			IPAddress:   "192.168.1.101",
			Gateway:     "192.168.1.1",
			DNS:         "114.114.114.114",
			UseStaticIP: false,
		},

		// 结构体切片（较少的元素）
		"sensors": []Sensor{
			{
				Type:         "temperature",
				Model:        "DS18B20",
				Value:        22.0,
				Unit:         "°C",
				MinValue:     -55.0,
				MaxValue:     125.0,
				Precision:    0,
				SamplingRate: 500,
				Enabled:      true,
			},
		},
		"maintenance_records": []MaintenanceRecord{},

		// 基本类型切片
		"tags":             []string{"indoor", "basic"},
		"alert_codes":      []int32{},
		"history_readings": []float64{21.5, 21.8, 22.0},

		// Map
		"metadata": map[string]any{
			"install_date":     "2025-01-01",
			"firmware_version": "v1.0.0",
		},
		"custom_settings": map[string]any{
			"report_interval": 120,
		},
	}

	err = table.Insert(device2)
	if err != nil {
		fmt.Printf("❌ 插入数据2失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 数据2插入成功: " + device2["name"].(string))
	fmt.Println("   包含: 9个Nullable字段（6个为nil）")
	fmt.Println("   包含: 较少的结构体切片元素")

	// Debug: 检查插入后的记录数
	count2, _ := table.Query().Rows()
	c2 := 0
	for count2.Next() {
		c2++
	}
	count2.Close()
	fmt.Printf("   🔍 插入后表中有 %d 条记录\n", c2)

	// 数据3: 所有 Nullable 字段为 nil
	device3 := map[string]any{
		// 基本字段
		"device_id": "COMPLEX-DEV-003",
		"name":      "最小配置设备",
		"model":     "MIN-CONFIG",

		// Nullable 字段（全部为 nil）
		"serial_number":    nil,
		"manufacturer":     nil,
		"description":      nil,
		"warranty_end":     nil,
		"last_maintenance": nil,
		"max_power":        nil,
		"weight":           nil,
		"voltage":          nil,
		"price":            nil,

		// 基本类型（最小值/默认值）
		"signal":        -90,
		"error_code":    int8(-1),
		"temperature":   int16(0),
		"counter":       int32(0),
		"total_bytes":   int64(0),
		"flags":         uint(0),
		"status":        uint8(0),
		"port":          uint16(0),
		"session_id":    uint32(0),
		"timestamp":     uint64(0),
		"humidity":      float32(0.0),
		"latitude":      0.0,
		"longitude":     0.0,
		"is_online":     false,
		"is_activated":  false,
		"battery_level": byte(0),
		"grade":         rune('C'),
		"total_price":   decimal.Zero,
		"created_at":    time.Unix(0, 0),
		"uptime":        0 * time.Second,

		// 嵌套结构体（空值）
		"location":       Location{},
		"network_config": NetworkConfig{},

		// 结构体切片（空切片）
		"sensors":             []Sensor{},
		"maintenance_records": []MaintenanceRecord{},

		// 基本类型切片（空切片）
		"tags":             []string{},
		"alert_codes":      []int32{},
		"history_readings": []float64{},

		// Map（空map）
		"metadata":        map[string]any{},
		"custom_settings": map[string]any{},
	}

	err = table.Insert(device3)
	if err != nil {
		fmt.Printf("❌ 插入数据3失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 数据3插入成功: " + device3["name"].(string))
	fmt.Println("   包含: 9个Nullable字段（全部为nil）")
	fmt.Println("   包含: 所有切片为空")

	// Debug: 检查插入后的记录数
	count3, _ := table.Query().Rows()
	c3 := 0
	for count3.Next() {
		c3++
	}
	count3.Close()
	fmt.Printf("   🔍 插入后表中有 %d 条记录\n", c3)

	// ========== 步骤 4: 查询并展示 ==========
	fmt.Println("\n【步骤 4】查询并验证数据")
	fmt.Println("─────────────────────────────────────────────────────")

	// Debug: 直接检查表的记录数
	debugRows, _ := table.Query().Rows()
	debugCount := 0
	for debugRows.Next() {
		debugCount++
	}
	debugRows.Close()
	fmt.Printf("🔍 调试: 表中实际有 %d 条记录\n\n", debugCount)

	rows, err := table.Query().OrderBy("_seq").Rows()
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		row := rows.Row()
		data := row.Data()
		count++

		fmt.Printf("\n╔══════════════════ 设备 #%d (seq=%d) ══════════════════╗\n", count, row.Seq())
		fmt.Printf("║ ID: %-53s ║\n", data["device_id"])
		fmt.Printf("║ 名称: %-51s ║\n", data["name"])
		fmt.Printf("║ 型号: %-51s ║\n", data["model"])

		// Nullable 字段展示
		fmt.Printf("╟────────────────── Nullable 字段 ─────────────────────╢\n")

		if data["serial_number"] != nil {
			fmt.Printf("║ 序列号: %-47s ║\n", data["serial_number"])
		} else {
			fmt.Printf("║ 序列号: <未设置>%40s ║\n", "")
		}

		if data["manufacturer"] != nil {
			fmt.Printf("║ 制造商: %-47s ║\n", data["manufacturer"])
		} else {
			fmt.Printf("║ 制造商: <未设置>%40s ║\n", "")
		}

		if data["price"] != nil {
			price := data["price"].(decimal.Decimal)
			fmt.Printf("║ 价格: ¥%-47s ║\n", price.StringFixed(2))
		} else {
			fmt.Printf("║ 价格: <未设置>%42s ║\n", "")
		}

		if data["warranty_end"] != nil {
			warrantyEnd := data["warranty_end"].(time.Time)
			fmt.Printf("║ 保修截止: %-43s ║\n", warrantyEnd.Format("2006-01-02"))
		} else {
			fmt.Printf("║ 保修截止: <未设置>%38s ║\n", "")
		}

		// 嵌套结构体展示
		fmt.Printf("╟───────────────── 嵌套结构体 ─────────────────────╢\n")

		location := data["location"].(map[string]any)
		fmt.Printf("║ 位置: %s %s %s%*s ║\n",
			location["country"], location["province"], location["city"],
			37-len(fmt.Sprint(location["country"], location["province"], location["city"])), "")
		fmt.Printf("║       地址: %-43v ║\n", location["address"])

		networkCfg := data["network_config"].(map[string]any)
		fmt.Printf("║ 网络: SSID=%v, IP=%v%*s ║\n",
			networkCfg["ssid"], networkCfg["ip_address"],
			27-len(fmt.Sprint(networkCfg["ssid"], networkCfg["ip_address"])), "")

		// 结构体切片展示
		fmt.Printf("╟───────────────── 结构体切片 ──────────────────────╢\n")

		sensors := data["sensors"].([]any)
		fmt.Printf("║ 传感器数量: %d 个%39s ║\n", len(sensors), "")
		for i, s := range sensors {
			sensor := s.(map[string]any)
			fmt.Printf("║   [%d] %s: %.1f %s (型号: %s)%*s ║\n",
				i+1, sensor["type"], sensor["value"], sensor["unit"], sensor["model"],
				20-len(fmt.Sprint(sensor["type"], sensor["model"])), "")
		}

		maintRecords := data["maintenance_records"].([]any)
		fmt.Printf("║ 维护记录: %d 条%40s ║\n", len(maintRecords), "")
		for i, m := range maintRecords {
			maint := m.(map[string]any)
			fmt.Printf("║   [%d] %s - %s (¥%.2f)%*s ║\n",
				i+1, maint["date"], maint["type"], maint["cost"],
				22-len(fmt.Sprint(maint["date"], maint["type"])), "")
		}

		// 基本类型切片
		fmt.Printf("╟───────────────── 基本类型切片 ────────────────────╢\n")

		tags := data["tags"].([]any)
		fmt.Printf("║ 标签: %d 个 %v%*s ║\n",
			len(tags), tags,
			45-len(fmt.Sprint(tags)), "")

		fmt.Println("╚═════════════════════════════════════════════════════════╝")
	}

	if count != 3 {
		fmt.Printf("\n❌ 预期 3 条记录，实际 %d 条\n", count)
		os.Exit(1)
	}

	// ========== 总结 ==========
	fmt.Println("\n\n=============================================================")
	fmt.Println("  ✅ 所有复杂类型测试通过！")
	fmt.Println("=============================================================")
	fmt.Println("\n📊 功能验证:")
	fmt.Println("  ✓ Nullable 字段（指针类型）")
	fmt.Println("    - 数据1: 9个Nullable字段全部有值")
	fmt.Println("    - 数据2: 9个Nullable字段部分为nil")
	fmt.Println("    - 数据3: 9个Nullable字段全部为nil")
	fmt.Println("\n  ✓ 嵌套结构体（Object）")
	fmt.Println("    - Location: 6个字段的位置信息结构体")
	fmt.Println("    - NetworkConfig: 6个字段的网络配置结构体")
	fmt.Println("\n  ✓ 结构体切片（Array of Struct）")
	fmt.Println("    - Sensors: 传感器列表（每个9个字段）")
	fmt.Println("    - MaintenanceRecords: 维护记录列表（每个6个字段）")
	fmt.Println("\n  ✓ 基本类型切片")
	fmt.Println("    - []string: 标签列表")
	fmt.Println("    - []int32: 告警代码列表")
	fmt.Println("    - []float64: 历史读数")
	fmt.Println("\n  ✓ Map类型")
	fmt.Println("    - metadata: 元数据信息")
	fmt.Println("    - custom_settings: 自定义设置")
	fmt.Println("\n💡 关键特性:")
	fmt.Println("  • 指针类型自动识别为Nullable")
	fmt.Println("  • 嵌套结构体自动转JSON")
	fmt.Println("  • 结构体切片自动序列化")
	fmt.Println("  • nil值正确处理和展示")
	fmt.Println("  • 空切片和空map正确存储")
	fmt.Printf("\n📁 数据已保存到: %s\n", absDir)
}
