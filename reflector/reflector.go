package reflector

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

var mdnsIPv4Addr = net.ParseIP("224.0.0.251")

const (
	mdnsPort     = 5353
	maxPacketLen = 9000

	shutdownTimeout = 5 * time.Second
)

type Reflector struct {
	interfaces []*net.Interface
	conns      map[string]*net.UDPConn
	mu         sync.RWMutex
	dedup      *dedupCache
	debug      bool

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownWg sync.WaitGroup
}

func NewReflector(ifaceNames []string, debug bool) (*Reflector, error) {
	ctx, cancel := context.WithCancel(context.Background())

	r := &Reflector{
		conns:  make(map[string]*net.UDPConn),
		dedup:  newDedupCache(),
		debug:  debug,
		ctx:    ctx,
		cancel: cancel,
	}

	for _, name := range ifaceNames {
		iface, err := net.InterfaceByName(name)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("interface %s not found: %w", name, err)
		}

		if iface.Flags&net.FlagUp == 0 {
			cancel()
			return nil, fmt.Errorf("interface %s is down", name)
		}
		if iface.Flags&net.FlagMulticast == 0 {
			cancel()
			return nil, fmt.Errorf("interface %s does not support multicast", name)
		}

		r.interfaces = append(r.interfaces, iface)
	}

	if len(r.interfaces) < 2 {
		cancel()
		return nil, fmt.Errorf("at least 2 interfaces are required, got %d", len(r.interfaces))
	}

	return r, nil
}

func (r *Reflector) Start() error {
	for _, iface := range r.interfaces {
		conn, err := r.joinMulticast(iface)
		if err != nil {
			r.Stop()
			return fmt.Errorf("failed to join multicast on %s: %w", iface.Name, err)
		}
		r.conns[iface.Name] = conn
		log.Printf("Joined mDNS multicast group on interface: %s", iface.Name)
	}

	r.wg.Go(func() {
		r.dedup.runCleanup(r.ctx)
	})

	for _, iface := range r.interfaces {
		r.wg.Go(func() {
			r.receiveLoop(iface)
		})
	}

	log.Printf("mDNS Reflector started with %d interfaces", len(r.interfaces))
	return nil
}

func (r *Reflector) joinMulticast(iface *net.Interface) (*net.UDPConn, error) {
	addr := &net.UDPAddr{
		IP:   mdnsIPv4Addr,
		Port: mdnsPort,
	}

	conn, err := net.ListenMulticastUDP("udp4", iface, addr)
	if err != nil {
		return nil, err
	}

	if err := conn.SetReadBuffer(maxPacketLen); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (r *Reflector) receiveLoop(iface *net.Interface) {
	r.mu.RLock()
	conn := r.conns[iface.Name]
	r.mu.RUnlock()

	if conn == nil {
		return
	}

	buf := make([]byte, maxPacketLen)

	for {
		select {
		case <-r.ctx.Done():
			if r.debug {
				log.Printf("Receive loop for %s stopping due to context cancellation", iface.Name)
			}
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, srcAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("Error reading from %s: %v", iface.Name, err)
			}
			return
		}

		if n == 0 {
			continue
		}

		select {
		case <-r.ctx.Done():
			if r.debug {
				log.Printf("Dropping packet on %s due to shutdown", iface.Name)
			}
			return
		default:
		}

		packet := make([]byte, n)
		copy(packet, buf[:n])

		if r.dedup.isDuplicate(iface.Name, packet) {
			if r.debug {
				log.Printf("Suppressed duplicate %d bytes on %s from %s", n, iface.Name, srcAddr.String())
			}
			continue
		}

		if r.debug {
			log.Printf("Received %d bytes on %s from %s", n, iface.Name, srcAddr.String())
		}

		r.shutdownWg.Go(func() {
			r.reflect(iface.Name, packet)
		})
	}
}

func (r *Reflector) reflect(srcIface string, packet []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	select {
	case <-r.ctx.Done():
		return
	default:
	}

	dstAddr := &net.UDPAddr{
		IP:   mdnsIPv4Addr,
		Port: mdnsPort,
	}

	for ifaceName, conn := range r.conns {
		if ifaceName == srcIface {
			continue
		}

		_, err := conn.WriteToUDP(packet, dstAddr)
		if err != nil {
			select {
			case <-r.ctx.Done():
				return
			default:
				log.Printf("Error reflecting to %s: %v", ifaceName, err)
			}
			continue
		}

		if r.debug {
			log.Printf("Reflected %d bytes from %s to %s", len(packet), srcIface, ifaceName)
		}
	}
}

func (r *Reflector) Stop() {
	log.Printf("Initiating graceful shutdown...")

	r.cancel()

	done := make(chan struct{})
	go func() {
		r.shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("All in-flight reflections completed")
	case <-time.After(shutdownTimeout):
		log.Printf("Timeout waiting for in-flight reflections")
	}

	r.mu.Lock()
	for name, conn := range r.conns {
		if conn != nil {
			conn.Close()
			log.Printf("Closed connection on interface: %s", name)
		}
	}
	r.conns = make(map[string]*net.UDPConn)
	r.mu.Unlock()

	done = make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("All receive loops terminated")
	case <-time.After(shutdownTimeout):
		log.Printf("Timeout waiting for receive loops to terminate")
	}

	log.Printf("Graceful shutdown completed")
}
