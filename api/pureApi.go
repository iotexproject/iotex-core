package api

import "strconv"

var (
	apiMap = map[string]func(*Server, interface{}) (interface{}, error){
		"eth_gasPrice": gasPrice,
	}
)

func gasPrice(svr *Server, in interface{}) (interface{}, error) {
	val, err := svr.suggestGasPrice()
	if err != nil {
		return nil, err
	}
	ret := strconv.FormatUint(val, 16)
	return "0x" + ret, nil
}
