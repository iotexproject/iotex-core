package mailutil

import (
	"fmt"
)

// Email define email struct
type Email struct {
}

// Send send msg to email address
func (e *Email) Send(msg string) error {
	fmt.Println("mail:", msg)
	return nil
}
