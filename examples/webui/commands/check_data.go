package commands

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
)

// CheckData 检查数据库中的数据
func CheckData(dbPath string) {
	// 打开数据库
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 列出所有表
	tables := db.ListTables()
	fmt.Printf("Found %d tables: %v\n", len(tables), tables)

	// 检查每个表的记录数
	for _, tableName := range tables {
		table, err := db.GetTable(tableName)
		if err != nil {
			fmt.Printf("Error getting table %s: %v\n", tableName, err)
			continue
		}

		result, err := table.Query().Rows()
		if err != nil {
			fmt.Printf("Error querying table %s: %v\n", tableName, err)
			continue
		}

		count := result.Count()
		fmt.Printf("Table '%s': %d rows\n", tableName, count)
	}
}
