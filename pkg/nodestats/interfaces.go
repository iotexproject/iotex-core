package nodestats

import "github.com/iotexproject/iotex-core/pkg/lifecycle"

type IRPCLocalStats interface {
	ReportCall(report RpcReport, size int64)
	BuildReport() string
	GetMethodStats(methodName string) *MethodStats
}

type NodeStats interface {
	lifecycle.StartStopper
}
