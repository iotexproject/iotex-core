package api

var (
	FuncMap = map[string]func(*Server, interface{}) interface{}{
		"eth_gasPrice": gasPrice,
	}
)

func gasPrice(svr *Server, in interface{}) interface{} {
	val, err := svr.suggestGasPrice()
	if err != nil {
		panic(err)
	}
	return val
}
