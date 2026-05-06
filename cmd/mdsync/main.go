package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sollniss/mdsync"
)

func main() {
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mdsync <file> [file...]")
		os.Exit(1)
	}

	exitCode := 0
	for _, f := range files {
		if err := processFile(f); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", f, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func processFile(filePath string) error {
	in, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open: %w", err)
	}
	defer in.Close()

	dir := filepath.Dir(filePath)
	out, err := os.CreateTemp(dir, "mdsync-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := out.Name()
	renamed := false
	defer func() {
		if !renamed {
			os.Remove(tmpPath)
		}
	}()

	if err := mdsync.Process(in, out); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	renamed = true

	return nil
}
