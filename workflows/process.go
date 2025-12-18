package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: process <number>")
		os.Exit(1)
	}

	num, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Error: invalid number %s\n", os.Args[1])
		os.Exit(1)
	}

	result := num * 2
	fmt.Printf("Processing %d: result is %d\n", num, result)
	fmt.Printf("OUTPUT_RESULT:%d\n", result)

	// Simulate some work
	time.Sleep(1 * time.Second)

	fmt.Println("Processing complete")
}
