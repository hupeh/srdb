package main

import (
	"flag"
	"fmt"
	"os"

	"code.tczkiot.com/wlw/srdb/examples/webui/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "webui", "serve":
		serveCmd := flag.NewFlagSet("webui", flag.ExitOnError)
		dbPath := serveCmd.String("db", "./data", "Database directory path")
		addr := serveCmd.String("addr", ":8080", "Server address")
		serveCmd.Parse(args)
		commands.StartWebUI(*dbPath, *addr)

	case "check-data":
		checkDataCmd := flag.NewFlagSet("check-data", flag.ExitOnError)
		dbPath := checkDataCmd.String("db", "./data", "Database directory path")
		checkDataCmd.Parse(args)
		commands.CheckData(*dbPath)

	case "check-seq":
		checkSeqCmd := flag.NewFlagSet("check-seq", flag.ExitOnError)
		dbPath := checkSeqCmd.String("db", "./data", "Database directory path")
		checkSeqCmd.Parse(args)
		commands.CheckSeq(*dbPath)

	case "dump-manifest":
		dumpCmd := flag.NewFlagSet("dump-manifest", flag.ExitOnError)
		dbPath := dumpCmd.String("db", "./data", "Database directory path")
		dumpCmd.Parse(args)
		commands.DumpManifest(*dbPath)

	case "inspect-all-sst":
		inspectAllCmd := flag.NewFlagSet("inspect-all-sst", flag.ExitOnError)
		sstDir := inspectAllCmd.String("dir", "./data/logs/sst", "SST directory path")
		inspectAllCmd.Parse(args)
		commands.InspectAllSST(*sstDir)

	case "inspect-sst":
		inspectCmd := flag.NewFlagSet("inspect-sst", flag.ExitOnError)
		sstPath := inspectCmd.String("file", "./data/logs/sst/000046.sst", "SST file path")
		inspectCmd.Parse(args)
		commands.InspectSST(*sstPath)

	case "test-fix":
		testFixCmd := flag.NewFlagSet("test-fix", flag.ExitOnError)
		dbPath := testFixCmd.String("db", "./data", "Database directory path")
		testFixCmd.Parse(args)
		commands.TestFix(*dbPath)

	case "test-keys":
		testKeysCmd := flag.NewFlagSet("test-keys", flag.ExitOnError)
		dbPath := testKeysCmd.String("db", "./data", "Database directory path")
		testKeysCmd.Parse(args)
		commands.TestKeys(*dbPath)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("SRDB WebUI - Database management tool")
	fmt.Println("\nUsage:")
	fmt.Println("  webui <command> [flags]")
	fmt.Println("\nCommands:")
	fmt.Println("  webui, serve       Start WebUI server (default: :8080)")
	fmt.Println("  check-data         Check database tables and row counts")
	fmt.Println("  check-seq          Check specific sequence numbers")
	fmt.Println("  dump-manifest      Dump manifest information")
	fmt.Println("  inspect-all-sst    Inspect all SST files")
	fmt.Println("  inspect-sst        Inspect a specific SST file")
	fmt.Println("  test-fix           Test fix for data retrieval")
	fmt.Println("  test-keys          Test key existence")
	fmt.Println("  help               Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  webui serve -db ./mydb -addr :3000")
	fmt.Println("  webui check-data -db ./mydb")
	fmt.Println("  webui inspect-sst -file ./data/logs/sst/000046.sst")
}
