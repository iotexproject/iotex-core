package ssdp

import "net"

// Interfaces specify target interfaces to multicast.  If no interfaces are
// specified, all interfaces will be used.
var Interfaces []net.Interface

func interfacesIPv4() []net.Interface {
	iflist, err := net.Interfaces()
	if err != nil {
		return nil
	}
	list := make([]net.Interface, 0, len(iflist))
	for _, ifi := range iflist {
		if !hasIPv4Address(&ifi) {
			continue
		}
		list = append(list, ifi)
	}
	return list
}

func hasIPv4Address(ifi *net.Interface) bool {
	addrs, err := ifi.Addrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			continue
		}
		if len(ip.To4()) == net.IPv4len && !ip.IsUnspecified() {
			return true
		}
	}
	return false
}
