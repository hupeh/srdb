package commands

import (
	"fmt"
	"log"

	"code.tczkiot.com/srdb"
)

// TestKeys 测试键
func TestKeys(dbPath string) {
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	table, err := db.GetTable("logs")
	if err != nil {
		log.Fatal(err)
	}

	// Test keys from different ranges
	testKeys := []int64{
		1, 100, 331, 332, 350, 400, 447, 500, 600, 700, 800, 850, 861, 862, 900, 1000, 1500, 1665, 1666, 1723,
	}

	fmt.Println("Testing key existence:")
	foundCount := 0
	for _, key := range testKeys {
		row, err := table.Get(key)
		if err != nil {
			fmt.Printf("Key %4d: NOT FOUND (%v)\n", key, err)
		} else if row == nil {
			fmt.Printf("Key %4d: NULL\n", key)
		} else {
			fmt.Printf("Key %4d: FOUND (time=%d)\n", key, row.Time)
			foundCount++
		}
	}

	fmt.Printf("\nFound %d out of %d test keys\n", foundCount, len(testKeys))

	// Query all
	result, err := table.Query().Rows()
	if err != nil {
		log.Fatal(err)
	}

	count := result.Count()
	fmt.Printf("Total rows from Query: %d\n", count)

	if count > 0 {
		first, _ := result.First()
		if first != nil {
			data := first.Data()
			fmt.Printf("First row _seq: %v\n", data["_seq"])
		}

		last, _ := result.Last()
		if last != nil {
			data := last.Data()
			fmt.Printf("Last row _seq: %v\n", data["_seq"])
		}
	}
}
