package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"localcat/internal/group"
)

const (
	multicastAddress = "224.0.0.250:9999"
	discoveryEvery   = 3 * time.Second
)

// Peer describes a LocalCat instance discovered on the LAN.
type Peer struct {
	ID       string
	Name     string
	Address  string
	Port     int
	LastSeen time.Time
	Groups   []group.GroupAdvertisement
}

type announcement struct {
	App    string                     `json:"app"`
	ID     string                     `json:"id"`
	Name   string                     `json:"name"`
	Port   int                        `json:"port"`
	Groups []group.GroupAdvertisement `json:"groups,omitempty"`
}

// Discovery sends and receives UDP multicast announcements.
type Discovery struct {
	mu        sync.RWMutex
	selfID    string
	name      string
	port      int
	groups    []group.GroupAdvertisement
	peers     chan Peer
	refreshCh chan struct{}
	iface     *net.Interface
	localAddr *net.UDPAddr
}

func NewDiscovery(selfID, name string, port int) *Discovery {
	return &Discovery{
		selfID:    selfID,
		name:      name,
		port:      port,
		peers:     make(chan Peer, 64),
		refreshCh: make(chan struct{}, 1),
	}
}

func (d *Discovery) Peers() <-chan Peer { return d.peers }

func (d *Discovery) UpdateName(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.name = name
}

func (d *Discovery) SetGroups(groups []group.GroupAdvertisement) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups = append([]group.GroupAdvertisement(nil), groups...)
}

func (d *Discovery) Refresh() {
	select {
	case d.refreshCh <- struct{}{}:
	default:
	}
}

// selectInterface finds an active IPv4 interface with a private LAN address.
func selectInterface() (*net.Interface, *net.UDPAddr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, fmt.Errorf("list interfaces: %w", err)
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			if !isPrivateIPv4(ip4) {
				continue
			}

			localAddr := &net.UDPAddr{IP: ip4, Port: 0}
			fmt.Printf("[DISCOVERY] interface: %s\n", iface.Name)
			fmt.Printf("[DISCOVERY] IPv4: %s\n", ip4.String())

			ifaceCopy := iface
			return &ifaceCopy, localAddr, nil
		}
	}

	return nil, nil, fmt.Errorf("no suitable IPv4 LAN interface found")
}

func isPrivateIPv4(ip net.IP) bool {
	if ip[0] == 10 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	return false
}

func (d *Discovery) Run(ctx context.Context) error {
	iface, localAddr, err := selectInterface()
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.iface = iface
	d.localAddr = localAddr
	d.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp4", multicastAddress)
	if err != nil {
		return fmt.Errorf("resolve multicast address: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", iface, addr)
	if err != nil {
		return fmt.Errorf("listen multicast: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(8192)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		d.readLoop(ctx, conn)
	}()
	go func() {
		defer wg.Done()
		d.refreshLoop(ctx, addr)
	}()
	go func() {
		defer wg.Done()
		d.announceLoop(ctx, addr)
	}()
	<-ctx.Done()
	_ = conn.SetReadDeadline(time.Now())
	wg.Wait()
	return nil
}

func (d *Discovery) refreshLoop(ctx context.Context, addr *net.UDPAddr) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.refreshCh:
			d.sendAnnouncement(addr)
		}
	}
}

func (d *Discovery) readLoop(ctx context.Context, conn *net.UDPConn) {
	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			continue
		}

		var ann announcement
		if err := json.Unmarshal(buf[:n], &ann); err != nil || ann.App != "LocalCat" || ann.ID == d.selfID || ann.Port == 0 {
			continue
		}

		peer := Peer{
			ID:       ann.ID,
			Name:     ann.Name,
			Address:  remote.IP.String(),
			Port:     ann.Port,
			LastSeen: time.Now(),
			Groups:   ann.Groups,
		}
		if peer.Groups == nil {
			peer.Groups = []group.GroupAdvertisement{}
		}

		select {
		case d.peers <- peer:
		default:
		}
	}
}

func (d *Discovery) announceLoop(ctx context.Context, addr *net.UDPAddr) {
	ticker := time.NewTicker(discoveryEvery)
	defer ticker.Stop()
	d.sendAnnouncement(addr)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sendAnnouncement(addr)
		}
	}
}

func (d *Discovery) sendAnnouncement(addr *net.UDPAddr) {
	d.mu.RLock()
	localAddr := d.localAddr
	d.mu.RUnlock()

	if localAddr == nil {
		return
	}

	conn, err := net.DialUDP("udp4", localAddr, addr)
	if err != nil {
		return
	}
	defer conn.Close()

	d.mu.RLock()
	ann := announcement{
		App:    "LocalCat",
		ID:     d.selfID,
		Name:   d.name,
		Port:   d.port,
		Groups: d.groups,
	}
	d.mu.RUnlock()

	payload, err := json.Marshal(ann)
	if err != nil {
		return
	}
	_, _ = conn.Write(payload)
}
