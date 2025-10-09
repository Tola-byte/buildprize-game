package main

import (
	"buildprize-game/internal/testing"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Get the directory of this file
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}
	
	// Change to the project root
	os.Chdir(dir)
	
	// Run the tests
	cmd := exec.Command("go", "test", "./internal/testing", "-v")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
