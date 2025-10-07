package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"code.tczkiot.com/srdb/sst"
)

// InspectAllSST 检查所有 SST 文件
func InspectAllSST(sstDir string) {
	// List all SST files
	files, err := os.ReadDir(sstDir)
	if err != nil {
		log.Fatal(err)
	}

	var sstFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sst") {
			sstFiles = append(sstFiles, file.Name())
		}
	}

	sort.Strings(sstFiles)

	fmt.Printf("Found %d SST files\n\n", len(sstFiles))

	// Inspect each file
	for _, filename := range sstFiles {
		sstPath := filepath.Join(sstDir, filename)

		reader, err := sst.NewReader(sstPath)
		if err != nil {
			fmt.Printf("%s: ERROR - %v\n", filename, err)
			continue
		}

		header := reader.GetHeader()
		allKeys := reader.GetAllKeys()

		// Extract file number
		numStr := strings.TrimPrefix(filename, "000")
		numStr = strings.TrimPrefix(numStr, "00")
		numStr = strings.TrimPrefix(numStr, "0")
		numStr = strings.TrimSuffix(numStr, ".sst")
		fileNum, _ := strconv.Atoi(numStr)

		fmt.Printf("File #%d (%s):\n", fileNum, filename)
		fmt.Printf("  Header: MinKey=%d MaxKey=%d RowCount=%d\n", header.MinKey, header.MaxKey, header.RowCount)
		fmt.Printf("  Actual: %d keys", len(allKeys))
		if len(allKeys) > 0 {
			fmt.Printf(" [%d ... %d]", allKeys[0], allKeys[len(allKeys)-1])
		}
		fmt.Printf("\n")

		// Check if header matches actual keys
		if len(allKeys) > 0 {
			if header.MinKey != allKeys[0] || header.MaxKey != allKeys[len(allKeys)-1] {
				fmt.Printf("  *** MISMATCH: Header says %d-%d but file has %d-%d ***\n",
					header.MinKey, header.MaxKey, allKeys[0], allKeys[len(allKeys)-1])
			}
		}

		reader.Close()
	}
}
