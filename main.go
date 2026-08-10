package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"localcat/internal/network"
)

type peerStore struct {
	mu    sync.Mutex
	peers map[string]network.Peer
}

func main() {
	application := app.NewWithID("dev.localcat.app")
	window := application.NewWindow("LocalCat")
	window.Resize(fyne.NewSize(520, 640))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := network.NewChatServer()
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}

	selfName := network.DefaultName()
	selfID := network.NewID()
	store := &peerStore{peers: make(map[string]network.Peer)}

	messages := widget.NewMultiLineEntry()
	messages.SetPlaceHolder("Pesan LAN akan muncul di sini...")
	messages.Wrapping = fyne.TextWrapWord
	onMessage := func(line string) {
		messages.SetText(messages.Text + line + "\n")
	}

	peerNames := []string{}
	var selectedPeer network.Peer
	peerSelect := widget.NewSelect(peerNames, func(name string) {
		store.mu.Lock()
		defer store.mu.Unlock()
		for _, peer := range store.peers {
			if displayName(peer) == name {
				selectedPeer = peer
				return
			}
		}
	})
	peerSelect.PlaceHolder = "Menunggu perangkat LocalCat..."

	input := widget.NewEntry()
	input.SetPlaceHolder("Ketik pesan...")
	sendButton := widget.NewButton("Kirim", func() {
		text := input.Text
		if text == "" || selectedPeer.ID == "" {
			return
		}
		msg := network.Message{From: selfName, Text: text, Time: time.Now()}
		go func(peer network.Peer) {
			if err := network.SendMessage(peer, msg); err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
				return
			}
			fyne.Do(func() { onMessage(fmt.Sprintf("Saya → %s: %s", peer.Name, text)) })
		}(selectedPeer)
		input.SetText("")
	})

	status := widget.NewLabel(fmt.Sprintf("Nama: %s • TCP:%d • Discovery UDP multicast aktif", selfName, server.Port()))
	content := container.NewBorder(
		container.NewVBox(widget.NewLabel("Pilih peer LocalCat di LAN:"), peerSelect, status),
		container.NewBorder(nil, nil, nil, sendButton, input),
		nil,
		nil,
		messages,
	)
	window.SetContent(content)

	go func() {
		if err := server.Run(ctx); err != nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
		}
	}()
	go func() {
		for msg := range server.Incoming() {
			message := msg
			fyne.Do(func() { onMessage(fmt.Sprintf("%s: %s", message.From, message.Text)) })
		}
	}()
	go runDiscovery(ctx, network.NewDiscovery(selfID, selfName, server.Port()), store, peerSelect, &selectedPeer)

	window.SetCloseIntercept(func() {
		cancel()
		window.Close()
	})
	window.ShowAndRun()
}

func runDiscovery(ctx context.Context, discovery *network.Discovery, store *peerStore, peerSelect *widget.Select, selectedPeer *network.Peer) {
	go func() { _ = discovery.Run(ctx) }()
	for {
		select {
		case <-ctx.Done():
			return
		case peer := <-discovery.Peers():
			store.mu.Lock()
			store.peers[peer.ID] = peer
			names := make([]string, 0, len(store.peers))
			for _, known := range store.peers {
				names = append(names, displayName(known))
			}
			sort.Strings(names)
			if selectedPeer.ID == "" {
				*selectedPeer = peer
			}
			store.mu.Unlock()
			fyne.Do(func() {
				peerSelect.Options = names
				if peerSelect.Selected == "" && len(names) > 0 {
					peerSelect.SetSelected(names[0])
				}
				peerSelect.Refresh()
			})
		}
	}
}

func displayName(peer network.Peer) string {
	return fmt.Sprintf("%s (%s:%d)", peer.Name, peer.Address, peer.Port)
}
