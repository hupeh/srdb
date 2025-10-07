package commands

import (
	"fmt"
	"log"
	"os"

	"code.tczkiot.com/srdb/sst"
)

// InspectSST 检查特定 SST 文件
func InspectSST(sstPath string) {
	// Check if file exists
	info, err := os.Stat(sstPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File: %s\n", sstPath)
	fmt.Printf("Size: %d bytes\n", info.Size())

	// Open reader
	reader, err := sst.NewReader(sstPath)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	// Get header
	header := reader.GetHeader()
	fmt.Printf("\nHeader:\n")
	fmt.Printf("  RowCount: %d\n", header.RowCount)
	fmt.Printf("  MinKey: %d\n", header.MinKey)
	fmt.Printf("  MaxKey: %d\n", header.MaxKey)
	fmt.Printf("  DataSize: %d bytes\n", header.DataSize)

	// Get all keys using GetAllKeys()
	allKeys := reader.GetAllKeys()
	fmt.Printf("\nActual keys in file: %d keys\n", len(allKeys))
	if len(allKeys) > 0 {
		fmt.Printf("  First key: %d\n", allKeys[0])
		fmt.Printf("  Last key: %d\n", allKeys[len(allKeys)-1])

		if len(allKeys) <= 30 {
			fmt.Printf("  All keys: %v\n", allKeys)
		} else {
			fmt.Printf("  First 15: %v\n", allKeys[:15])
			fmt.Printf("  Last 15: %v\n", allKeys[len(allKeys)-15:])
		}
	}

	// Try to get a specific key
	fmt.Printf("\nTrying to get key 332:\n")
	row, err := reader.Get(332)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if row == nil {
		fmt.Printf("  NULL\n")
	} else {
		fmt.Printf("  FOUND: seq=%d, time=%d\n", row.Seq, row.Time)
	}

	// Try to get key based on actual first key
	if len(allKeys) > 0 {
		firstKey := allKeys[0]
		fmt.Printf("\nTrying to get actual first key %d:\n", firstKey)
		row, err := reader.Get(firstKey)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if row == nil {
			fmt.Printf("  NULL\n")
		} else {
			fmt.Printf("  FOUND: seq=%d, time=%d\n", row.Seq, row.Time)
		}
	}
}
