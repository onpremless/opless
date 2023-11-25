package util

import (
	"fmt"
	"os"
	"strconv"
)

func GetStrVar(name string) string {
	v := os.Getenv(name)
	if v == "" {
		panic("failed to get env var: " + name)
	}

	return v
}

func GetIntVar(name string) int {
	s := GetStrVar(name)
	if s == "" {
		panic("failed to get env var: " + name)
	}

	v, err := strconv.Atoi(s)

	if err != nil {
		panic(fmt.Errorf("failed to convert env var %s to int: %w", name, err))
	}

	return v
}
