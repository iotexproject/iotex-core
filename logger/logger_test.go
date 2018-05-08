package logger

import "testing"

func TestLogger(t *testing.T) {
	Print("test")
	Debug().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
}
