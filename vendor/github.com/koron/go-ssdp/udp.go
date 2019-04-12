package ssdp

import (
	"errors"
	"net"
	"time"

	"golang.org/x/net/ipv4"
)

var (
	ssdpAddrIPv4 *net.UDPAddr
)

func init() {
	var err error
	ssdpAddrIPv4, err = net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		panic(err)
	}
}

type packetHandler func(net.Addr, []byte) error

func readPackets(conn *net.UDPConn, timeout time.Duration, h packetHandler) error {
	buf := make([]byte, 65535)
	conn.SetReadBuffer(len(buf))
	conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				return nil
			}
			return err
		}
		if err := h(addr, buf[:n]); err != nil {
			return err
		}
	}
}

func sendTo(to *net.UDPAddr, data []byte) (int, error) {
	conn, err := net.DialUDP("udp4", nil, to)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	n, err := conn.Write(data)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func multicastListen(localAddr string) (*net.UDPConn, error) {
	// prepare parameters.
	laddr, err := net.ResolveUDPAddr("udp4", localAddr)
	if err != nil {
		return nil, err
	}
	// connect.
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, err
	}
	// configure socket to use with multicast.
	if err := joinGroupIPv4(conn, Interfaces, ssdpAddrIPv4); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}

// joinGroupIPv4 makes the connection join to a group on interfaces.
func joinGroupIPv4(conn net.PacketConn, iflist []net.Interface, gaddr net.Addr) error {
	wrap := ipv4.NewPacketConn(conn)
	wrap.SetMulticastLoopback(true)
	if len(iflist) == 0 {
		iflist = interfacesIPv4()
	}
	// add interfaces to multicast group.
	joined := 0
	for _, ifi := range iflist {
		if err := wrap.JoinGroup(&ifi, gaddr); err != nil {
			logf("failed to join group %s on %s: %s", gaddr.String(), ifi.Name, err)
			continue
		}
		joined++
		logf("joined gropup %s on %s", gaddr.String(), ifi.Name)
	}
	if joined == 0 {
		return errors.New("no interfaces had joined to group")
	}
	return nil
}
