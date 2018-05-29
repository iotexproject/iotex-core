package logger

import "testing"

func TestLogger(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
		}
	}()

	Print("test")
	Debug().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Info().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Warn().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Error().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Panic().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
}
