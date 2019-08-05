package mailutil

import (
	"fmt"
)

type SendEmailConf struct {
	SmtpAddr    string   `json:"smtp_addr"`
	SmtpPort    int      `json:"smtp_port"`
	SmtpAccount string   `json:"smtp_account"`
	SmtpPwd     string   `json:"smtp_pwd"`
	From        string   `json:"from"`
	To          []string `json:"to"`
}

type EmailConf struct {
	To []string
}
type Email struct {
}

func (e *Email) Send(msg string) error {
	fmt.Println("mail:", msg)
	return nil
}
