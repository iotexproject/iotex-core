package p2p

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

// EnsureIPv4 returns an IPv4 address regardless the input is a IPv4 address or host name. If the host name has multiple
// IPv4 address be associated, a random one will be returned.
func EnsureIPv4(ipOrHost string) (string, error) {
	ip := net.ParseIP(ipOrHost)
	if ip != nil {
		if ip.To4() == nil {
			return "", fmt.Errorf("%s is IPv6 address", ipOrHost)
		}
		return ipOrHost, nil
	}
	addrs, err := net.LookupHost(ipOrHost)
	if err != nil {
		return "", err
	}
	ips := make([]string, 0)
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil {
			if ip.To4() == nil {
				continue
			}
			ips = append(ips, addr)
		}
	}
	if len(ips) == 0 {
		return "", errors.New("no IPv4 address found")
	}
	rand.Seed(time.Now().UnixNano())
	return ips[rand.Intn(len(ips))], nil
}
