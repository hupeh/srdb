package commands

import (
	"fmt"
	"log"

	"github.com/hupeh/srdb"
)

// TestFix 测试修复
func TestFix(dbPath string) {
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	table, err := db.GetTable("logs")
	if err != nil {
		log.Fatal(err)
	}

	// Get total count
	result, err := table.Query().Rows()
	if err != nil {
		log.Fatal(err)
	}
	totalCount := result.Count()
	fmt.Printf("Total rows in Query(): %d\n", totalCount)

	// Test Get() for first 10, middle 10, and last 10
	testRanges := []struct {
		name  string
		start int64
		end   int64
	}{
		{"First 10", 1, 10},
		{"Middle 10", 50, 59},
		{"Last 10", int64(totalCount) - 9, int64(totalCount)},
	}

	for _, tr := range testRanges {
		fmt.Printf("\n%s (keys %d-%d):\n", tr.name, tr.start, tr.end)
		foundCount := 0
		for seq := tr.start; seq <= tr.end; seq++ {
			row, err := table.Get(seq)
			if err != nil {
				fmt.Printf("  Seq %d: ERROR - %v\n", seq, err)
			} else if row == nil {
				fmt.Printf("  Seq %d: NULL\n", seq)
			} else {
				foundCount++
			}
		}
		fmt.Printf("  Found: %d/%d\n", foundCount, tr.end-tr.start+1)
	}

	fmt.Printf("\n✅ If all keys found, the bug is FIXED!\n")
}
