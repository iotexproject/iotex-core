package common

import (
	"os"
)

// TranslateInLang Switch language output.
func TranslateInLang(cmdString map[string]string) string {
	langEnv := os.Getenv("LANG")
	switch langEnv {
	case "zh_CN.UTF-8":
		return cmdString["chinese"]
	default:
		return cmdString["english"]
	}
}
