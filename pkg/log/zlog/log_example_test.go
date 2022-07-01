package zlog_test

import (
	"os"

	"github.com/rs/zerolog"
)

func ExampleLogger_Debug() {
	log := zerolog.New(os.Stdout)

	log.Debug().
		Str("foo", "bar").
		Int("n", 123).
		Msg("hello world")

	// Output: {"level":"debug","foo":"bar","n":123,"message":"hello world"}
}

// func ExamplePrint() {
// 	log := zerolog.New(os.Stdout)

// 	log.Debug().
// 		Str("foo", "bar").
// 		Int("n", 123).
// 		Msg("hello world")

// 	// Output: {"level":"debug","foo":"bar","n":123,"message":"hello world"}
// }

// err := InitLoggers(zaplog.GlobalConfig{}, map[string]zaplog.GlobalConfig{}, map[string]interface{}{"ioAddr": "accountaddr"})
// if err != nil {
// 	return
// }
// writers := []io.Writer{zerolog.ConsoleWriter{
// 	Out:        os.Stdout,
// 	TimeFormat: time.RFC3339,
// }}
// log.Logger = log.Output(zerolog.MultiLevelWriter(writers...)).
// 	With().
// 	// Timestamp().
// 	// Caller().
// 	// Fields(fields).
// 	Logger()
