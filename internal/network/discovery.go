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
}

func NewDiscovery(selfID, name string, port int) *Discovery {
	return &Discovery{
		selfID:    selfID,
		name:      name,
		port:      port,
		peers:     make(chan Peer, 32),
		refreshCh: make(chan struct{}, 1),
	}
}

func (d *Discovery) Peers() <-chan Peer { return d.peers }

func (d *Discovery) UpdateName(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.name = name
}

// SetGroups replaces the advertised group metadata for this peer.
// An empty or nil slice removes group advertisements.
func (d *Discovery) SetGroups(groups []group.GroupAdvertisement) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups = append([]group.GroupAdvertisement(nil), groups...)
}

// Refresh triggers an additional announcement using the same Discovery instance.
// If a refresh is already pending, subsequent calls are ignored.
func (d *Discovery) Refresh() {
	select {
	case d.refreshCh <- struct{}{}:
	default:
	}
}

func (d *Discovery) Run(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp4", multicastAddress)
	if err != nil {
		return fmt.Errorf("resolve multicast address: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
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
	conn, err := net.DialUDP("udp4", nil, addr)
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
