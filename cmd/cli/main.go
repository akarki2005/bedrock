package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/akarki2005/lsm-engine/pkg/engine"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
	lsm-cli -path <dbdir> put <key> <value>
	lsm-cli -path <dbdir> get <key>
	`)
	os.Exit(1)
}

func main() {
	path := flag.String("path", "./data", "path to database directory")
	flag.Parse()

	if flag.NArg() < 2 {
		usage()
	}

	cmd := flag.Arg(0)

	db, err := engine.Open(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open engine: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "put":
		if flag.NArg() != 3 {
			usage()
		}

		key := []byte(flag.Arg(1))
		value := []byte(flag.Arg(2))

		if err := db.Put(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "put: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("ok")
	case "get":
		if flag.NArg() != 2 {
			usage()
		}

		key := []byte(flag.Arg(1))

		value, ok, err := db.Get(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "get: %v\n", err)
			os.Exit(1)
		}

		if !ok {
			fmt.Println("not found")
			os.Exit(0)
		}

		fmt.Printf("%s\n", value)
	default:
		usage()
	}

	if err := db.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close engine: %v\n", err)
	}
}
