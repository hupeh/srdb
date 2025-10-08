package commands

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
)

// CheckSeq 检查特定序列号的数据
func CheckSeq(dbPath string) {
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	table, err := db.GetTable("logs")
	if err != nil {
		log.Fatal(err)
	}

	// Check seq 1
	row1, err := table.Get(1)
	if err != nil {
		fmt.Printf("Error getting seq=1: %v\n", err)
	} else if row1 == nil {
		fmt.Println("Seq=1: NOT FOUND")
	} else {
		fmt.Printf("Seq=1: FOUND (time=%d)\n", row1.Time)
	}

	// Check seq 100
	row100, err := table.Get(100)
	if err != nil {
		fmt.Printf("Error getting seq=100: %v\n", err)
	} else if row100 == nil {
		fmt.Println("Seq=100: NOT FOUND")
	} else {
		fmt.Printf("Seq=100: FOUND (time=%d)\n", row100.Time)
	}

	// Check seq 729
	row729, err := table.Get(729)
	if err != nil {
		fmt.Printf("Error getting seq=729: %v\n", err)
	} else if row729 == nil {
		fmt.Println("Seq=729: NOT FOUND")
	} else {
		fmt.Printf("Seq=729: FOUND (time=%d)\n", row729.Time)
	}

	// Query all records
	result, err := table.Query().Rows()
	if err != nil {
		log.Fatal(err)
	}

	count := result.Count()
	fmt.Printf("\nTotal rows from Query: %d\n", count)

	if count > 0 {
		first, _ := result.First()
		if first != nil {
			data := first.Data()
			fmt.Printf("First row _seq: %v\n", data["_seq"])
		}
	}
}
