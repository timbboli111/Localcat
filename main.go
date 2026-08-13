package main

import (
	"context"
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"localcat/internal/history"
	"localcat/internal/identity"
	"localcat/internal/network"
)

const appVersion = "v1.2.0"

var (
	colorPrimary      = color.NRGBA{R: 0x1E, G: 0x88, B: 0xE5, A: 0xFF}
	colorPrimaryLight = color.NRGBA{R: 0xE3, G: 0xF2, B: 0xFD, A: 0xFF}
	colorIncomingBg   = color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
	colorOutgoingBg   = color.NRGBA{R: 0x1E, G: 0x88, B: 0xE5, A: 0xFF}
	colorGreen        = color.NRGBA{R: 0x4C, G: 0xAF, B: 0x50, A: 0xFF}
	colorGray         = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF}
	colorTextDark     = color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
	colorTextSub      = color.NRGBA{R: 0x75, G: 0x75, B: 0x75, A: 0xFF}
	colorTextWhite    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorSelectedBg   = color.NRGBA{R: 0xE3, G: 0xF2, B: 0xFD, A: 0xFF}
)

type peerStore struct {
	mu    sync.Mutex
	peers map[string]network.Peer
}

type peerListItem struct {
	peer        network.Peer
	unread      int
	lastMsg     string
	lastMsgTime time.Time
	selected    bool
}

func main() {
	application := app.NewWithID("dev.localcat.app")
	window := application.NewWindow("LocalCat")
	window.Resize(fyne.NewSize(920, 680))

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

	peerListItems := make([]peerListItem, 0)
	var peerListWidget *widget.List

	chatMessagesContainer := container.NewVBox()
	chatScroll := container.NewVScroll(chatMessagesContainer)

	roomTitle := widget.NewLabel("Room chat")
	roomTitle.TextStyle = fyne.TextStyle{Bold: true}

	roomStatusDot := canvas.NewCircle(colorGray)
	roomStatusDot.Resize(fyne.NewSize(8, 8))

	roomSubtitle := widget.NewLabel("")
	roomSubtitle.TextStyle = fyne.TextStyle{Italic: true}

	roomHeader := container.NewVBox(
		roomTitle,
		container.NewHBox(
			roomStatusDot,
			roomSubtitle,
		),
	)

	updatePeerListUI := func() {}

	buildChatBubble := func(msg history.Message) fyne.CanvasObject {
		timeStr := msg.SentAt.Format("15:04")
		displayTime := timeStr
		if msg.Outgoing {
			displayTime = timeStr + "  ✓✓"
		}

		textCanvas := canvas.NewText(msg.Text, colorTextDark)
		textCanvas.TextSize = 14

		timeCanvas := canvas.NewText(displayTime, colorTextSub)
		timeCanvas.TextSize = 11
		timeCanvas.TextStyle = fyne.TextStyle{Italic: true}

		if msg.Outgoing {
			textCanvas.Color = colorTextWhite
			timeCanvas.Color = colorTextWhite
		}

		bubbleBg := canvas.NewRectangle(colorIncomingBg)
		if msg.Outgoing {
			bubbleBg = canvas.NewRectangle(colorOutgoingBg)
		}
		bubbleBg.CornerRadius = 14

		bubbleContent := container.NewVBox(
			textCanvas,
			timeCanvas,
		)

		bubble := container.NewStack(
			bubbleBg,
			container.NewPadded(bubbleContent),
		)

		if msg.Outgoing {
			// OUTGOING: spacer fleksibel di kiri, bubble di kanan
			return container.NewHBox(
				layout.NewSpacer(),
				bubble,
			)
		}
		// INCOMING: bubble di kiri, spacer fleksibel di kanan
		return container.NewHBox(
			bubble,
			layout.NewSpacer(),
		)
	}

	renderConversation := func(peer network.Peer) {
		_ = historyStore.MarkAsRead(history.DirectID(peer.ID))
		roomTitle.SetText(peer.Name)

		if isPeerOnline(peer) {
			roomStatusDot.FillColor = colorGreen
			roomSubtitle.SetText(fmt.Sprintf("● Online • %s:%d", peer.Address, peer.Port))
		} else {
			roomStatusDot.FillColor = colorGray
			roomSubtitle.SetText(fmt.Sprintf("○ Offline • %s:%d", peer.Address, peer.Port))
		}
		roomStatusDot.Refresh()

		msgs := historyStore.Messages(history.DirectID(peer.ID))
		chatMessagesContainer.Objects = nil
		if len(msgs) == 0 {
			emptyLabel := widget.NewLabel("Belum ada pesan dengan " + peer.Name + ".")
			emptyLabel.Alignment = fyne.TextAlignCenter
			emptyLabel.Importance = widget.LowImportance
			chatMessagesContainer.Add(emptyLabel)
		} else {
			for _, msg := range msgs {
				chatMessagesContainer.Add(buildChatBubble(msg))
			}
		}
		chatMessagesContainer.Refresh()
		chatScroll.ScrollToBottom()
		updatePeerListUI()
	}

	appendStoredMessage := func(peer network.Peer, msg history.Message) {
		conv := history.Conversation{ID: history.DirectID(peer.ID), Type: history.DirectConversation, Title: peer.Name, Participant: peer.ID}
		if err := historyStore.AddMessage(conv, msg); err != nil {
			dialog.ShowError(err, window)
			return
		}
		if selectedPeer.ID == peer.ID {
			renderConversation(peer)
		} else {
			updatePeerListUI()
		}
	}

	buildPeerListItems := func(store *peerStore, hs *history.Store, selected network.Peer) []peerListItem {
		store.mu.Lock()
		defer store.mu.Unlock()

		items := make([]peerListItem, 0, len(store.peers))
		for id, peer := range store.peers {
			convID := history.DirectID(id)
			msgs := hs.Messages(convID)
			unread := 0
			var lastMsg string
			var lastTime time.Time
			if len(msgs) > 0 {
				lastMsg = msgs[len(msgs)-1].Text
				lastTime = msgs[len(msgs)-1].SentAt
			}
			for _, conv := range hs.Conversations() {
				if conv.ID == convID {
					unread = conv.UnreadCount
					break
				}
			}
			items = append(items, peerListItem{
				peer:        peer,
				unread:      unread,
				lastMsg:     lastMsg,
				lastMsgTime: lastTime,
				selected:    selected.ID == peer.ID,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].peer.Name < items[j].peer.Name
		})
		return items
	}

	updatePeerListUI = func() {
		peerListItems = buildPeerListItems(peers, historyStore, selectedPeer)
		if peerListWidget != nil {
			peerListWidget.Refresh()
		}
	}

	peerListWidget = widget.NewList(
		func() int { return len(peerListItems) },
		func() fyne.CanvasObject {
			statusDot := canvas.NewCircle(colorGray)
			statusDot.Resize(fyne.NewSize(10, 10))

			nameLabel := widget.NewLabel("")
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}

			ipLabel := widget.NewLabel("")
			ipLabel.TextStyle = fyne.TextStyle{Italic: true}
			ipLabel.Importance = widget.LowImportance

			lastMsgLabel := widget.NewLabel("")
			lastMsgLabel.Wrapping = fyne.TextTruncate
			lastMsgLabel.Importance = widget.LowImportance

			unreadBadge := widget.NewLabel("")
			unreadBadge.TextStyle = fyne.TextStyle{Bold: true}

			bgRect := canvas.NewRectangle(color.Transparent)
			bgRect.CornerRadius = 8

			return container.NewStack(
				bgRect,
				container.NewPadded(container.NewVBox(
					container.NewHBox(statusDot, nameLabel),
					container.NewHBox(widget.NewLabel("   "), ipLabel),
					container.NewHBox(widget.NewLabel("   "), lastMsgLabel, unreadBadge),
				)),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(peerListItems) {
				return
			}
			item := peerListItems[id]
			stack := obj.(*fyne.Container)
			bgRect := stack.Objects[0].(*canvas.Rectangle)
			padded := stack.Objects[1].(*fyne.Container)
			vbox := padded.Objects[0].(*fyne.Container)

			statusRow := vbox.Objects[0].(*fyne.Container)
			statusDot := statusRow.Objects[0].(*canvas.Circle)
			nameLabel := statusRow.Objects[1].(*widget.Label)

			ipRow := vbox.Objects[1].(*fyne.Container)
			ipLabel := ipRow.Objects[1].(*widget.Label)

			lastRow := vbox.Objects[2].(*fyne.Container)
			lastMsgLabel := lastRow.Objects[1].(*widget.Label)
			unreadBadge := lastRow.Objects[2].(*widget.Label)

			if isPeerOnline(item.peer) {
				statusDot.FillColor = colorGreen
			} else {
				statusDot.FillColor = colorGray
			}
			statusDot.Refresh()

			nameLabel.SetText(item.peer.Name)
			ipLabel.SetText(item.peer.Address)

			if item.lastMsgTime.IsZero() {
				lastMsgLabel.SetText("")
			} else {
				lastMsgLabel.SetText(item.lastMsg)
			}

			if item.unread > 0 {
				unreadBadge.SetText(fmt.Sprintf("%d", item.unread))
			} else {
				unreadBadge.SetText("")
			}

			if item.selected {
				bgRect.FillColor = colorSelectedBg
			} else {
				bgRect.FillColor = color.Transparent
			}
			bgRect.Refresh()
		},
	)
	peerListWidget.OnSelected = func(id widget.ListItemID) {
		if id >= len(peerListItems) {
			return
		}
		item := peerListItems[id]
		selectedPeer = item.peer
		renderConversation(selectedPeer)
	}

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
	sendButton.Importance = widget.HighImportance

	status := widget.NewLabel("")
	refreshStatus := func() {
		status.SetText(fmt.Sprintf("● Discovery UDP multicast aktif • TCP:%d", server.Port()))
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
					chatMessagesContainer.Objects = nil
					emptyLabel := widget.NewLabel("Pesan LAN akan muncul di sini...")
					emptyLabel.Alignment = fyne.TextAlignCenter
					chatMessagesContainer.Add(emptyLabel)
					chatMessagesContainer.Refresh()
				}
				updatePeerListUI()
			}
		}, window)
	}
	window.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("Settings", fyne.NewMenuItem("Change display name", func() { showNameDialog(false) }), fyne.NewMenuItem("Delete conversation", deleteConversation), fyne.NewMenuItem("Delete all history", deleteAll))))

	peersTitle := widget.NewLabel("Peers")
	peersTitle.TextStyle = fyne.TextStyle{Bold: true}
	leftPanel := container.NewBorder(
		peersTitle,
		nil, nil, nil,
		peerListWidget,
	)

	chatInputRow := container.NewBorder(nil, nil, nil, sendButton, input)
	rightPanel := container.NewBorder(
		roomHeader,
		chatInputRow,
		nil, nil,
		chatScroll,
	)

	splitView := container.NewHSplit(leftPanel, rightPanel)
	splitView.Offset = 0.28

	appTitle := widget.NewLabel("LocalCat")
	appTitle.TextStyle = fyne.TextStyle{Bold: true}

	topBar := container.NewVBox(
		container.NewHBox(appTitle, refreshButton),
		status,
	)

	footer := widget.NewLabel(fmt.Sprintf("LocalCat %s • ID Lokal: %s • TCP: %d", appVersion, ident.ID[:8], server.Port()))
	footer.TextStyle = fyne.TextStyle{Italic: true}
	footer.Importance = widget.LowImportance

	content := container.NewBorder(
		topBar,
		footer,
		nil, nil,
		splitView,
	)
	window.SetContent(content)

	updatePeerListUI()

	go func() {
		if err := server.Run(ctx); err != nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
		}
	}()
	go func() {
		for msg := range server.Incoming() {
			message := msg
			fyne.Do(func() {
				peers.mu.Lock()
				knownPeer, exists := peers.peers[message.FromID]
				peers.mu.Unlock()

				var peer network.Peer
				if exists {
					peer = knownPeer
				} else {
					peer = network.Peer{ID: message.FromID, Name: message.From}
					if peer.ID == "" {
						peer.ID = message.From
					}
				}

				appendStoredMessage(peer, history.Message{SenderID: peer.ID, SenderName: message.From, Text: message.Text, SentAt: message.Time})
				sendNotification(application, peer.Name, message.Text)
			})
		}
	}()
	go runDiscovery(ctx, discovery, peers, updatePeerListUI, &selectedPeer)
	go checkPeerTimeout(ctx, peers, updatePeerListUI)

	window.SetCloseIntercept(func() { cancel(); window.Close() })
	window.ShowAndRun()
}

func sendNotification(app fyne.App, title, content string) {
	app.SendNotification(&fyne.Notification{
		Title:   title,
		Content: content,
	})
}

func runDiscovery(ctx context.Context, discovery *network.Discovery, store *peerStore, onUpdate func(), selectedPeer *network.Peer) {
	go func() { _ = discovery.Run(ctx) }()
	for {
		select {
		case <-ctx.Done():
			return
		case peer := <-discovery.Peers():
			store.mu.Lock()
			store.peers[peer.ID] = peer
			if selectedPeer.ID == "" {
				*selectedPeer = peer
			} else if selectedPeer.ID == peer.ID {
				*selectedPeer = peer
			}
			store.mu.Unlock()
			fyne.Do(func() {
				onUpdate()
			})
		}
	}
}

func checkPeerTimeout(ctx context.Context, store *peerStore, onUpdate func()) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fyne.Do(func() {
				onUpdate()
			})
		}
	}
}

func isPeerOnline(peer network.Peer) bool {
	return time.Since(peer.LastSeen) <= 10*time.Second
}
