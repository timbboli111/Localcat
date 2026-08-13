package network

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// ChatServer accepts TCP messages from peers.
type ChatServer struct {
	listener net.Listener
	incoming chan Message
}

func NewChatServer() (*ChatServer, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen tcp: %w", err)
	}
	return &ChatServer{listener: listener, incoming: make(chan Message, 64)}, nil
}

func (s *ChatServer) Port() int {
	_, portText, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(portText)
	return port
}

func (s *ChatServer) Incoming() <-chan Message { return s.incoming }

func (s *ChatServer) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
	}()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				close(s.incoming)
				return nil
			}
			return fmt.Errorf("accept tcp: %w", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handleConnection(conn)
		}()
	}
}

func (s *ChatServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		msg, err := ReadMessage(reader)
		if err != nil {
			return
		}
		select {
		case s.incoming <- msg:
		default:
		}
	}
}

// SendMessage sends a message to a specific peer.
func SendMessage(peer Peer, msg Message) error {
	address := net.JoinHostPort(peer.Address, strconv.Itoa(peer.Port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", address, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return WriteMessage(conn, msg)
}

// SendToAddress sends a message to a specific address:port.
func SendToAddress(address string, port int, msg Message) error {
	addr := net.JoinHostPort(address, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return WriteMessage(conn, msg)
}
