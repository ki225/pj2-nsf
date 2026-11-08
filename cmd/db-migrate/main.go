package main

import (
	"flag"
	"fmt"
	"nsf/internal/db"
	"os"
)

func main() {
	source := flag.String("source", db.NSF_DB_PATH, "source bbolt file (stop NSF before migration)")
	uri := flag.String("uri", "mongodb://127.0.0.1:27017", "MongoDB connection URI")
	database := flag.String("database", "nsf", "target MongoDB database; services collection must be empty")
	dryRun := flag.Bool("dry-run", false, "inspect counts without copying records")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(1)
	}
	result, err := MigrateBbolt(*source, *uri, *database, *dryRun)
	fmt.Printf("Source services: %d\nTarget services before migration: %d\n", result.SourceCount, result.TargetCount)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed:", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Println("DRY RUN: no records written; source file retained.")
		return
	}
	fmt.Printf("Migrated services: %d\nVerification: PASS (keys and values match)\nSource file retained.\n", result.MigratedCount)
}
