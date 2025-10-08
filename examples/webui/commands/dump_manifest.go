package commands

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
)

// DumpManifest 导出 manifest 信息
func DumpManifest(dbPath string) {
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	table, err := db.GetTable("logs")
	if err != nil {
		log.Fatal(err)
	}

	versionSet := table.GetVersionSet()
	version := versionSet.GetCurrent()

	// Check for duplicates in each level
	for level := range 7 {
		files := version.GetLevel(level)
		if len(files) == 0 {
			continue
		}

		// Track file numbers
		fileMap := make(map[int64][]struct {
			minKey int64
			maxKey int64
		})

		for _, f := range files {
			fileMap[f.FileNumber] = append(fileMap[f.FileNumber], struct {
				minKey int64
				maxKey int64
			}{f.MinKey, f.MaxKey})
		}

		// Report duplicates
		fmt.Printf("Level %d: %d files\n", level, len(files))
		for fileNum, entries := range fileMap {
			if len(entries) > 1 {
				fmt.Printf("  [DUPLICATE] File #%d appears %d times:\n", fileNum, len(entries))
				for i, e := range entries {
					fmt.Printf("    Entry %d: min=%d max=%d\n", i+1, e.minKey, e.maxKey)
				}
			}
		}
	}
}
