package lang

import (
	"os"
)

func TranslateInLang(cmdString map[string]string) string {
	lang := os.Getenv("LANG")
	switch lang {
	case "zh_CN.UTF-8":
		return cmdString["chinese"]
	default:
		return cmdString["english"]
	}
}
