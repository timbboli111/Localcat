package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"localcat/internal/history"
	"localcat/internal/identity"
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

	ident, err := identity.Load(application.Preferences())
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}
	storePath := filepath.Join(application.Storage().RootURI().Path(), "history.json")
	historyStore, err := history.Open(storePath)
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := network.NewChatServer()
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}

	peers := &peerStore{peers: make(map[string]network.Peer)}
	var selectedPeer network.Peer

	messageLabel := widget.NewLabel("Pesan LAN akan muncul di sini...")
	messageLabel.Wrapping = fyne.TextWrapWord
	messageScroll := container.NewVScroll(messageLabel)

	renderConversation := func(peer network.Peer) {
		_ = historyStore.MarkAsRead(history.DirectID(peer.ID))
		msgs := historyStore.Messages(history.DirectID(peer.ID))
		if len(msgs) == 0 {
			messageLabel.SetText("Belum ada pesan dengan " + peer.Name + ".")
			return
		}
		var b strings.Builder
		for _, msg := range msgs {
			name := msg.SenderName
			if msg.Outgoing {
				name = "Saya → " + peer.Name
			}
			b.WriteString(fmt.Sprintf("%s: %s\n", name, msg.Text))
		}
		messageLabel.SetText(b.String())
		messageScroll.ScrollToBottom()
	}
	appendStoredMessage := func(peer network.Peer, msg history.Message) {
		conv := history.Conversation{ID: history.DirectID(peer.ID), Type: history.DirectConversation, Title: peer.Name, Participant: peer.ID}
		if err := historyStore.AddMessage(conv, msg); err != nil {
			dialog.ShowError(err, window)
			return
		}
		if selectedPeer.ID == peer.ID {
			renderConversation(peer)
		}
	}

	peerSelect := widget.NewSelect(nil, func(name string) {
		peers.mu.Lock()
		defer peers.mu.Unlock()
		for _, peer := range peers.peers {
			if displayName(peer) == name {
				selectedPeer = peer
				renderConversation(peer)
				return
			}
		}
	})
	peerSelect.PlaceHolder = "Menunggu perangkat LocalCat..."

	input := widget.NewEntry()
	input.SetPlaceHolder("Ketik pesan...")

	sendMessage := func() {
		text := strings.TrimSpace(input.Text)
		if text == "" || selectedPeer.ID == "" {
			return
		}
		msg := network.Message{From: ident.DisplayName, FromID: ident.ID, Text: text, Time: time.Now()}
		peer := selectedPeer
		go func() {
			if err := network.SendMessage(peer, msg); err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
				return
			}
			fyne.Do(func() {
				appendStoredMessage(peer, history.Message{SenderID: ident.ID, SenderName: ident.DisplayName, Text: text, SentAt: msg.Time, Outgoing: true})
			})
		}()
		input.SetText("")
		window.Canvas().Focus(input)
	}

	input.OnSubmitted = func(_ string) {
		sendMessage()
	}

	sendButton := widget.NewButton("Kirim", func() {
		sendMessage()
	})

	status := widget.NewLabel("")
	refreshStatus := func() {
		status.SetText(fmt.Sprintf("Nama: %s • ID:%s • TCP:%d • Discovery UDP multicast aktif", ident.DisplayName, ident.ID[:8], server.Port()))
	}
	refreshStatus()

	discovery := network.NewDiscovery(ident.ID, ident.DisplayName, server.Port())

	refreshButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		discovery.Refresh()
	})

	var showNameDialog func(first bool)
	showNameDialog = func(first bool) {
		entry := widget.NewEntry()
		entry.SetText(ident.DisplayName)
		entry.SetPlaceHolder("Nama tampilan")
		d := dialog.NewCustomConfirm("Nama tampilan", "Simpan", "Batal", entry, func(ok bool) {
			if !ok && !first {
				return
			}
			name := strings.TrimSpace(entry.Text)
			if err := identity.SaveDisplayName(application.Preferences(), name); err != nil {
				dialog.ShowError(err, window)
				if first {
					showNameDialog(true)
				}
				return
			}
			ident.DisplayName = name
			discovery.UpdateName(name)
			refreshStatus()
		}, window)
		d.Show()
	}
	if ident.DisplayName == "" {
		showNameDialog(true)
	}

	deleteConversation := func() {
		if selectedPeer.ID != "" {
			dialog.ShowConfirm("Hapus percakapan", "Hapus riwayat lokal dengan "+selectedPeer.Name+"? Riwayat di perangkat lain tidak terpengaruh.", func(ok bool) {
				if ok {
					_ = historyStore.DeleteConversation(history.DirectID(selectedPeer.ID))
					renderConversation(selectedPeer)
				}
			}, window)
		}
	}
	deleteAll := func() {
		dialog.ShowConfirm("Hapus semua riwayat", "Hapus semua riwayat chat lokal di perangkat ini?", func(ok bool) {
			if ok {
				_ = historyStore.DeleteAll()
				if selectedPeer.ID != "" {
					renderConversation(selectedPeer)
				} else {
					messageLabel.SetText("Pesan LAN akan muncul di sini...")
				}
			}
		}, window)
	}
	window.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("Settings", fyne.NewMenuItem("Change display name", func() { showNameDialog(false) }), fyne.NewMenuItem("Delete conversation", deleteConversation), fyne.NewMenuItem("Delete all history", deleteAll))))

	header := container.NewBorder(nil, nil, nil, refreshButton,
		container.NewVBox(
			widget.NewLabel("Pilih peer LocalCat di LAN:"),
			peerSelect,
			status,
		))

	content := container.NewBorder(header, container.NewBorder(nil, nil, nil, sendButton, input), nil, nil, messageScroll)
	window.SetContent(content)

	go func() {
		if err := server.Run(ctx); err != nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
		}
	}()
	go func() {
		for msg := range server.Incoming() {
			message := msg
			fyne.Do(func() {
				peer := network.Peer{ID: message.FromID, Name: message.From}
				if peer.ID == "" {
					peer.ID = message.From
				}
				appendStoredMessage(peer, history.Message{SenderID: peer.ID, SenderName: message.From, Text: message.Text, SentAt: message.Time})
				sendNotification(application, peer.Name, message.Text)
			})
		}
	}()
	go runDiscovery(ctx, discovery, peers, peerSelect, &selectedPeer)
	go checkPeerTimeout(ctx, peers)

	window.SetCloseIntercept(func() { cancel(); window.Close() })
	window.ShowAndRun()
}

func sendNotification(app fyne.App, title, content string) {
	app.SendNotification(&fyne.Notification{
		Title:   title,
		Content: content,
	})
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
			} else if selectedPeer.ID == peer.ID {
				*selectedPeer = peer
			}
			store.mu.Unlock()
			fyne.Do(func() {
				peerSelect.Options = names
				if peerSelect.Selected == "" && len(names) > 0 {
					peerSelect.SetSelected(names[0])
				} else if selectedPeer.ID != "" {
					valid := false
					for _, name := range names {
						if name == peerSelect.Selected {
							valid = true
							break
						}
					}
					if !valid {
						peerSelect.SetSelected(displayName(*selectedPeer))
					}
				}
				peerSelect.Refresh()
			})
		}
	}
}

func checkPeerTimeout(ctx context.Context, store *peerStore) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			store.mu.Lock()
			for id, peer := range store.peers {
				if !isPeerOnline(peer) {
					// Peer offline/stale - evaluasi dilakukan, status dapat digunakan
					// oleh komponen lain yang membutuhkan
					_ = id
				}
			}
			store.mu.Unlock()
		}
	}
}

func isPeerOnline(peer network.Peer) bool {
	return time.Since(peer.LastSeen) <= 10*time.Second
}

func displayName(peer network.Peer) string {
	return fmt.Sprintf("%s (%s:%d)", peer.Name, peer.Address, peer.Port)
}
