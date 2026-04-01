package main

import (
	"fmt"
	"os"

	"github.com/andys/dbtop"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: dbtop <database-uri>\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  dbtop postgres://user:pass@host:5432/dbname\n")
		fmt.Fprintf(os.Stderr, "  dbtop mysql://user:pass@host:3306/dbname\n")
		os.Exit(1)
	}

	if err := dbtop.Run(os.Args[1], "", nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
