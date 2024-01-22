package state

import "fmt"

// SomeFunc following code for testing codecov, delete later
func SomeFunc(flag bool) bool {
	if flag {
		fmt.Print(flag)
		return flag
	}
	return !flag
}
