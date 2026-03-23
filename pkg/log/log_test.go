package log

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestDynamicFieldsCoreUpdatesIoAddr(t *testing.T) {
	t.Cleanup(func() {
		SetDynamicFields()
	})

	observedCore, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(dynamicFieldsCore{Core: observedCore})

	SetDynamicFields(zap.String("ioAddr", "io1-old"))
	logger.Info("first")

	SetDynamicFields(zap.String("ioAddr", "io1-new"))
	logger.Info("second")

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}
	if got := entries[0].ContextMap()["ioAddr"]; got != "io1-old" {
		t.Fatalf("expected first ioAddr to be io1-old, got %v", got)
	}
	if got := entries[1].ContextMap()["ioAddr"]; got != "io1-new" {
		t.Fatalf("expected second ioAddr to be io1-new, got %v", got)
	}
}
