package wiresocks

import (
	"context"
	"net"
	"net/netip"
	"sync"
)

func NewVtunUDPForwarder(ctx context.Context, localBind netip.AddrPort, dest string, vtun *VirtualTun, mtu int) (netip.AddrPort, error) {
	destAddr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		return netip.AddrPort{}, err
	}

	listener, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(localBind))
	if err != nil {
		return netip.AddrPort{}, err
	}

	rconn, err := vtun.Tnet.DialUDP(nil, destAddr)
	if err != nil {
		return netip.AddrPort{}, err
	}

	var clientAddr *net.UDPAddr
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		buffer := make([]byte, mtu)
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			default:
				n, cAddr, err := listener.ReadFrom(buffer)
				if err != nil {
					continue
				}

				clientAddr = cAddr.(*net.UDPAddr)

				rconn.WriteTo(buffer[:n], destAddr)
			}
		}
	}()
	go func() {
		buffer := make([]byte, mtu)
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			default:
				n, _, err := rconn.ReadFrom(buffer)
				if err != nil {
					continue
				}
				if clientAddr != nil {
					listener.WriteTo(buffer[:n], clientAddr)
				}
			}
		}
	}()
	go func() {
		wg.Wait()
		_ = listener.Close()
		_ = rconn.Close()
	}()

	return listener.LocalAddr().(*net.UDPAddr).AddrPort(), nil
}
