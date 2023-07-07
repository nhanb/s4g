package main

import (
	"fmt"
	"time"
)

func timer(format string) func() {
	start := time.Now()
	return func() {
		fmt.Printf(format+"\n", time.Since(start))
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
