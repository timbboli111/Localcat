package main

import (
	"context"
	"fmt"
	"image/color"
	"net"
	"path/filepath"
	"sort"
	"strconv"
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

	"localcat/internal/group"
	"localcat/internal/history"
	"localcat/internal/identity"
	"localcat/internal/network"
)

const appVersion = "v1.3.6"

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
	colorGroupBg      = color.NRGBA{R: 0xE8, G: 0xF5, B: 0xE9, A: 0xFF}
)

type peerStore struct {
	mu    sync.Mutex
	peers map[string]network.Peer
}

type conversationItem struct {
	id          string
	title       string
	subtitle    string
	unread      int
	lastMsg     string
	lastMsgTime time.Time
	isGroup     bool
	groupID     string
	selected    bool
}

type pendingRequestItem struct {
	GroupID       string
	GroupName     string
	RequesterID   string
	RequesterName string
}

type pendingPeer struct {
	Address string
	Port    int
}

type bubbleRowLayout struct {
	outgoing bool
}

func (l *bubbleRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}

func (l *bubbleRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	bubble := objects[0]
	bubbleSize := bubble.MinSize()
	bubble.Resize(bubbleSize)

	var x float32
	if l.outgoing {
		x = size.Width - bubbleSize.Width
	} else {
		x = 0
	}
	bubble.Move(fyne.NewPos(x, 0))
}

type fixedWidthLayout struct {
	width float32
}

func (l *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	min := objects[0].MinSize()
	return fyne.NewSize(l.width, min.Height)
}

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	child := objects[0]
	child.Resize(fyne.NewSize(l.width, size.Height))
	child.Move(fyne.NewPos(0, 0))
}

func measureTextWidth(text string) float32 {
	if text == "" {
		return 0
	}
	txt := canvas.NewText(text, color.Black)
	txt.TextSize = theme.TextSize()
	return txt.MinSize().Width
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

	storageDir := application.Storage().RootURI().Path()

	historyStore, err := history.Open(filepath.Join(storageDir, "history.json"))
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}

	groupPersistence := group.NewPersistence(filepath.Join(storageDir, "groups.json"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := network.NewChatServer()
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}

	groupManager := group.NewManager()
	groupDiscovery := group.NewGroupDiscovery()
	historyAdapter := group.NewHistoryAdapter(historyStore)
	notificationService := group.NewNotificationService(
		&fyneNotifier{app: application},
		historyAdapter,
	)
	relay := group.NewRelay()

	persistedGroups, err := groupPersistence.Load()
	if err != nil {
		dialog.ShowError(err, window)
		window.ShowAndRun()
		return
	}
	for _, g := range persistedGroups {
		if err := groupManager.AddGroup(g); err != nil {
			continue
		}
		if err := relay.RegisterGroup(g); err != nil {
			continue
		}
	}

	peers := &peerStore{peers: make(map[string]network.Peer)}
	var selectedConversationID string
	var pendingPeers sync.Map
	var joinRequestsDialog *dialog.CustomDialog

	conversationItems := make([]conversationItem, 0)
	var conversationListWidget *widget.List

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

	updateConversationListUI := func() {}

	getChatWidth := func() float32 {
		w := chatScroll.Size().Width
		if w > 0 {
			return w
		}
		canvasW := window.Canvas().Size().Width
		if canvasW > 0 {
			return canvasW * 0.72 // splitView.Offset = 0.28
		}
		return 400
	}

	buildChatBubble := func(msg history.Message, senderName string) fyne.CanvasObject {
		timeStr := msg.SentAt.Format("15:04")
		displayTime := timeStr
		if msg.Outgoing {
			displayTime = timeStr + "  ✓✓"
		}

		chatWidth := getChatWidth()
		maxWidth := chatWidth * 0.70
		if maxWidth < 200 {
			maxWidth = 200
		}
		if maxWidth > 600 {
			maxWidth = 600
		}
		padding := theme.Padding() * 2
		contentMaxWidth := maxWidth - padding
		if contentMaxWidth < 0 {
			contentMaxWidth = 0
		}

		naturalWidth := measureTextWidth(msg.Text)

		var bodyWrapper fyne.CanvasObject
		if naturalWidth <= contentMaxWidth {
			bodyLabel := widget.NewLabel(msg.Text)
			bodyLabel.Wrapping = fyne.TextWrapOff
			bodyWrapper = bodyLabel
		} else {
			richText := widget.NewRichText(&widget.TextSegment{Text: msg.Text, Style: widget.RichTextStyle{}})
			richText.Wrapping = fyne.TextWrapWord
			richText.Resize(fyne.NewSize(contentMaxWidth, 0))
			richText.Refresh()
			bodyWrapper = container.New(&fixedWidthLayout{width: contentMaxWidth}, richText)
		}

		var textContent fyne.CanvasObject

		if msg.Outgoing {
			timeLabel := widget.NewLabel(displayTime)
			timeLabel.TextStyle = fyne.TextStyle{Italic: true}
			textContent = container.NewVBox(bodyWrapper, timeLabel)
		} else if senderName != "" {
			userLabel := widget.NewLabel(senderName)
			userLabel.TextStyle = fyne.TextStyle{Bold: true}
			timeLabel := widget.NewLabel(displayTime)
			timeLabel.TextStyle = fyne.TextStyle{Italic: true}
			textContent = container.NewVBox(userLabel, bodyWrapper, timeLabel)
		} else {
			timeLabel := widget.NewLabel(displayTime)
			timeLabel.TextStyle = fyne.TextStyle{Italic: true}
			textContent = container.NewVBox(bodyWrapper, timeLabel)
		}

		bubbleBg := canvas.NewRectangle(colorIncomingBg)
		if msg.Outgoing {
			bubbleBg = canvas.NewRectangle(colorOutgoingBg)
		}
		bubbleBg.CornerRadius = 14

		bubble := container.NewStack(
			bubbleBg,
			container.NewPadded(textContent),
		)

		return container.New(&bubbleRowLayout{outgoing: msg.Outgoing}, bubble)
	}

	renderConversation := func(convID string) {
		_ = historyStore.MarkAsRead(convID)
		chatMessagesContainer.Objects = nil

		var title string
		var subtitle string

		if strings.HasPrefix(convID, "group:") {
			groupID := strings.TrimPrefix(convID, "group:")
			g, exists := groupManager.GetGroup(groupID)
			if exists {
				title = g.Name
				if g.Closed {
					subtitle = fmt.Sprintf("● Closed • Code: %s", g.Code)
					roomStatusDot.FillColor = colorGray
				} else {
					subtitle = fmt.Sprintf("● Group • %s • Code: %s", g.JoinPolicy.String(), g.Code)
					roomStatusDot.FillColor = colorGreen
				}
			} else {
				title = "Group"
				subtitle = "Unknown group"
				roomStatusDot.FillColor = colorGray
			}
		} else {
			peerID := strings.TrimPrefix(convID, "direct:")
			peers.mu.Lock()
			peer, exists := peers.peers[peerID]
			peers.mu.Unlock()
			if exists {
				title = peer.Name
				if isPeerOnline(peer) {
					roomStatusDot.FillColor = colorGreen
					subtitle = fmt.Sprintf("● Online • %s:%d", peer.Address, peer.Port)
				} else {
					roomStatusDot.FillColor = colorGray
					subtitle = fmt.Sprintf("○ Offline • %s:%d", peer.Address, peer.Port)
				}
			} else {
				title = peerID
				subtitle = "Offline"
				roomStatusDot.FillColor = colorGray
			}
		}

		roomTitle.SetText(title)
		roomSubtitle.SetText(subtitle)
		roomStatusDot.Refresh()

		msgs := historyStore.Messages(convID)
		if len(msgs) == 0 {
			emptyLabel := widget.NewLabel("Belum ada pesan.")
			emptyLabel.Alignment = fyne.TextAlignCenter
			emptyLabel.Importance = widget.LowImportance
			chatMessagesContainer.Add(emptyLabel)
		} else {
			for _, msg := range msgs {
				senderName := msg.SenderName
				if msg.Outgoing {
					senderName = ""
				}
				chatMessagesContainer.Add(buildChatBubble(msg, senderName))
			}
		}
		chatMessagesContainer.Refresh()
		chatScroll.ScrollToBottom()
		updateConversationListUI()
	}

	appendStoredMessage := func(convID string, convType string, title string, msg history.Message) {
		conv := history.Conversation{
			ID:        convID,
			Type:      convType,
			Title:     title,
			UpdatedAt: msg.SentAt,
		}
		if err := historyStore.AddMessage(conv, msg); err != nil {
			dialog.ShowError(err, window)
			return
		}
		if selectedConversationID == convID {
			renderConversation(convID)
		} else {
			updateConversationListUI()
		}
	}

	buildConversationItems := func(store *peerStore, hs *history.Store, gm *group.Manager, selected string) []conversationItem {
		store.mu.Lock()
		defer store.mu.Unlock()

		items := make([]conversationItem, 0)

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
			items = append(items, conversationItem{
				id:          convID,
				title:       peer.Name,
				subtitle:    peer.Address,
				unread:      unread,
				lastMsg:     lastMsg,
				lastMsgTime: lastTime,
				isGroup:     false,
				selected:    selected == convID,
			})
		}

		for _, g := range gm.AllGroups() {
			convID := history.GroupID(g.ID)
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
			subtitle := fmt.Sprintf("Group • %s", g.JoinPolicy.String())
			if g.Closed {
				subtitle = "Group • Closed"
			}
			items = append(items, conversationItem{
				id:          convID,
				title:       g.Name,
				subtitle:    subtitle,
				unread:      unread,
				lastMsg:     lastMsg,
				lastMsgTime: lastTime,
				isGroup:     true,
				groupID:     g.ID,
				selected:    selected == convID,
			})
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].title < items[j].title
		})
		return items
	}

	updateConversationListUI = func() {
		conversationItems = buildConversationItems(peers, historyStore, groupManager, selectedConversationID)
		if conversationListWidget != nil {
			conversationListWidget.Refresh()
		}
	}

	conversationListWidget = widget.NewList(
		func() int { return len(conversationItems) },
		func() fyne.CanvasObject {
			statusDot := canvas.NewCircle(colorGray)
			statusDot.Resize(fyne.NewSize(10, 10))

			nameLabel := widget.NewLabel("")
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}

			subtitleLabel := widget.NewLabel("")
			subtitleLabel.TextStyle = fyne.TextStyle{Italic: true}
			subtitleLabel.Importance = widget.LowImportance

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
					container.NewHBox(widget.NewLabel("   "), subtitleLabel),
					container.NewHBox(widget.NewLabel("   "), lastMsgLabel, unreadBadge),
				)),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(conversationItems) {
				return
			}
			item := conversationItems[id]
			stack := obj.(*fyne.Container)
			bgRect := stack.Objects[0].(*canvas.Rectangle)
			padded := stack.Objects[1].(*fyne.Container)
			vbox := padded.Objects[0].(*fyne.Container)

			statusRow := vbox.Objects[0].(*fyne.Container)
			statusDot := statusRow.Objects[0].(*canvas.Circle)
			nameLabel := statusRow.Objects[1].(*widget.Label)

			subtitleRow := vbox.Objects[1].(*fyne.Container)
			subtitleLabel := subtitleRow.Objects[1].(*widget.Label)

			lastRow := vbox.Objects[2].(*fyne.Container)
			lastMsgLabel := lastRow.Objects[0].(*widget.Label)
			unreadBadge := lastRow.Objects[1].(*widget.Label)

			if item.isGroup {
				statusDot.FillColor = colorGroupBg
			} else {
				statusDot.FillColor = colorGray
			}
			statusDot.Refresh()

			nameLabel.SetText(item.title)
			subtitleLabel.SetText(item.subtitle)

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
	conversationListWidget.OnSelected = func(id widget.ListItemID) {
		if id >= len(conversationItems) {
			return
		}
		item := conversationItems[id]
		selectedConversationID = item.id
		renderConversation(selectedConversationID)
	}

	input := widget.NewEntry()
	input.SetPlaceHolder("Ketik pesan...")

	getLocalAddress := func() string {
		conn, err := net.Dial("udp", "224.0.0.250:9999")
		if err != nil {
			return ""
		}
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	sendMessage := func() {
		text := strings.TrimSpace(input.Text)
		if text == "" || selectedConversationID == "" {
			return
		}

		if strings.HasPrefix(selectedConversationID, "group:") {
			groupID := strings.TrimPrefix(selectedConversationID, "group:")
			g, exists := groupManager.GetGroup(groupID)
			if !exists {
				dialog.ShowError(fmt.Errorf("group not found"), window)
				return
			}
			if g.Closed {
				dialog.ShowError(fmt.Errorf("group is closed"), window)
				return
			}
			if !g.HasMember(ident.ID) {
				dialog.ShowError(fmt.Errorf("you are not a member of this group"), window)
				return
			}

			msgID, err := group.NewMessageID()
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			gmsg := group.GroupMessage{
				MessageID:  msgID,
				GroupID:    groupID,
				SenderID:   ident.ID,
				SenderName: ident.DisplayName,
				Body:       text,
				Timestamp:  time.Now(),
			}

			if err := historyAdapter.SaveOutgoing(g, gmsg); err != nil {
				dialog.ShowError(err, window)
				return
			}

			if g.IsHost(ident.ID) {
				targets, err := relay.GetRelayTargets(groupID, ident.ID)
				if err == nil && len(targets) > 0 {
					relayMsg := network.Message{
						Type:       "group",
						MessageID:  msgID,
						GroupID:    groupID,
						SenderID:   ident.ID,
						SenderName: ident.DisplayName,
						Body:       text,
						Time:       time.Now(),
					}
					for _, target := range targets {
						go func(addr string, port int) {
							_ = network.SendToAddress(addr, port, relayMsg)
						}(target.Address, target.Port)
					}
				}
			} else {
				peers.mu.Lock()
				hostPeer, hostExists := peers.peers[g.HostID]
				peers.mu.Unlock()
				if hostExists && isPeerOnline(hostPeer) {
					relayMsg := network.Message{
						Type:       "group",
						MessageID:  msgID,
						GroupID:    groupID,
						SenderID:   ident.ID,
						SenderName: ident.DisplayName,
						Body:       text,
						Time:       time.Now(),
					}
					go func() {
						if err := network.SendMessage(hostPeer, relayMsg); err != nil {
							fyne.Do(func() { dialog.ShowError(err, window) })
						}
					}()
				} else {
					dialog.ShowError(fmt.Errorf("host is offline"), window)
				}
			}

			renderConversation(selectedConversationID)
			input.SetText("")
			window.Canvas().Focus(input)
		} else {
			peerID := strings.TrimPrefix(selectedConversationID, "direct:")
			peers.mu.Lock()
			peer, exists := peers.peers[peerID]
			peers.mu.Unlock()
			if !exists {
				return
			}
			msg := network.Message{From: ident.DisplayName, FromID: ident.ID, Text: text, Time: time.Now()}
			go func() {
				if err := network.SendMessage(peer, msg); err != nil {
					fyne.Do(func() { dialog.ShowError(err, window) })
					return
				}
				fyne.Do(func() {
					appendStoredMessage(history.DirectID(peer.ID), history.DirectConversation, peer.Name, history.Message{
						SenderID: ident.ID, SenderName: ident.DisplayName, Text: text, SentAt: msg.Time, Outgoing: true,
					})
				})
			}()
			input.SetText("")
			window.Canvas().Focus(input)
		}
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

	saveGroup := func(g *group.Group) {
		_ = groupPersistence.SaveGroup(g)
	}

	showCreateGroupDialog := func() {
		nameEntry := widget.NewEntry()
		nameEntry.SetPlaceHolder("Group Name")

		policySelect := widget.NewSelect([]string{"OPEN", "LOCKED"}, func(_ string) {})
		policySelect.SetSelected("OPEN")

		form := container.NewVBox(
			widget.NewLabel("Group Name:"),
			nameEntry,
			widget.NewLabel("Join Policy:"),
			policySelect,
		)

		dialog.ShowCustomConfirm("Create New Group", "Create", "Cancel", form, func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				dialog.ShowError(fmt.Errorf("group name is required"), window)
				return
			}

			var policy group.JoinPolicy
			if policySelect.Selected == "LOCKED" {
				policy = group.Locked
			} else {
				policy = group.Open
			}

			g, err := group.Create(name, ident.ID, policy)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if err := g.SetHostDisplayName(ident.DisplayName); err != nil {
				dialog.ShowError(err, window)
				return
			}
			if err := groupManager.AddGroup(g); err != nil {
				dialog.ShowError(err, window)
				return
			}
			if err := relay.RegisterGroup(g); err != nil {
				dialog.ShowError(err, window)
				return
			}
			localAddr := getLocalAddress()
			if localAddr != "" {
				_ = relay.SetMemberPeer(g.ID, ident.ID, localAddr, server.Port())
			}

			saveGroup(g)

			updateGroupAdvertisements(discovery, groupManager, ident.DisplayName, getLocalAddress(), server.Port())

			codeDisplay := widget.NewLabel(g.Code)
			codeDisplay.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

			copyButton := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
				window.Clipboard().SetContent(g.Code)
			})

			resultContent := container.NewVBox(
				widget.NewLabel("Group created successfully!"),
				widget.NewLabel(""),
				widget.NewLabel("Group:"),
				widget.NewLabel(g.Name),
				widget.NewLabel(""),
				widget.NewLabel("Group Code:"),
				container.NewHBox(codeDisplay, copyButton),
			)
			dialog.ShowCustom("Group Created", "OK", resultContent, window)

			selectedConversationID = history.GroupID(g.ID)
			renderConversation(selectedConversationID)
			updateConversationListUI()
		}, window)
	}

	showJoinGroupDialog := func() {
		codeEntry := widget.NewEntry()
		codeEntry.SetPlaceHolder("8-digit Group Code")

		form := container.NewVBox(
			widget.NewLabel("Enter Group Code:"),
			codeEntry,
		)

		dialog.ShowCustomConfirm("Join Group", "Join", "Cancel", form, func(ok bool) {
			if !ok {
				return
			}
			code := group.FormatGroupCode(codeEntry.Text)
			if code == "" {
				dialog.ShowError(fmt.Errorf("group code must be exactly 8 digits"), window)
				return
			}

			go func() {
				ad, err := groupDiscovery.FindByCode(code)
				if err == group.ErrGroupNotFound {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("group not found"), window) })
					return
				}
				if err == group.ErrGroupCodeAmbiguous {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("multiple groups found with this code"), window) })
					return
				}
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, window) })
					return
				}

				fyne.Do(func() {
					if ad.JoinPolicy == group.Open {
						g := &group.Group{
							ID:         ad.ID,
							Code:       ad.Code,
							Name:       ad.Name,
							HostID:     ad.HostID,
							JoinPolicy: group.Open,
							Members:    make(map[string]group.Member),
							CreatedAt:  time.Now(),
							Closed:     false,
						}
						g.Members[ad.HostID] = group.Member{ID: ad.HostID, Name: ad.HostName, Role: group.RoleHost, JoinedAt: time.Now()}
						g.Members[ident.ID] = group.Member{ID: ident.ID, Name: ident.DisplayName, Role: group.RoleMember, JoinedAt: time.Now()}

						if err := groupManager.AddGroup(g); err != nil {
							dialog.ShowError(err, window)
							return
						}
						if err := relay.RegisterGroup(g); err != nil {
							dialog.ShowError(err, window)
							return
						}
						localAddr := getLocalAddress()
						if localAddr != "" {
							_ = relay.SetMemberPeer(g.ID, ident.ID, localAddr, server.Port())
						}
						saveGroup(g)
						selectedConversationID = history.GroupID(g.ID)
						renderConversation(selectedConversationID)
						updateConversationListUI()
						dialog.ShowInformation("Joined", fmt.Sprintf("You joined %s", ad.Name), window)
					} else {
						peers.mu.Lock()
						hostPeer, hostExists := peers.peers[ad.HostID]
						peers.mu.Unlock()

						if !hostExists || !isPeerOnline(hostPeer) {
							dialog.ShowError(fmt.Errorf("host is offline"), window)
							return
						}

						joinMsg := network.Message{
							Type:          "join_request",
							GroupID:       ad.ID,
							RequesterID:   ident.ID,
							RequesterName: ident.DisplayName,
							Time:          time.Now(),
						}
						go func() {
							if err := network.SendMessage(hostPeer, joinMsg); err != nil {
								fyne.Do(func() { dialog.ShowError(err, window) })
								return
							}
							fyne.Do(func() {
								dialog.ShowInformation("Pending", "Join request sent. Waiting for host approval.", window)
							})
						}()
					}
				})
			}()
		}, window)
	}

	groupMenu := fyne.NewMenu("Groups",
		fyne.NewMenuItem("Create New Group", showCreateGroupDialog),
		fyne.NewMenuItem("Join Group", showJoinGroupDialog),
	)

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
		if selectedConversationID != "" {
			var title string
			if strings.HasPrefix(selectedConversationID, "group:") {
				groupID := strings.TrimPrefix(selectedConversationID, "group:")
				if g, exists := groupManager.GetGroup(groupID); exists {
					title = g.Name
				}
			} else {
				title = selectedConversationID
			}
			dialog.ShowConfirm("Hapus percakapan", "Hapus riwayat lokal "+title+"? Riwayat di perangkat lain tidak terpengaruh.", func(ok bool) {
				if ok {
					_ = historyStore.DeleteConversation(selectedConversationID)
					selectedConversationID = ""
					chatMessagesContainer.Objects = nil
					emptyLabel := widget.NewLabel("Pilih percakapan")
					emptyLabel.Alignment = fyne.TextAlignCenter
					chatMessagesContainer.Add(emptyLabel)
					chatMessagesContainer.Refresh()
					updateConversationListUI()
				}
			}, window)
		}
	}
	deleteAll := func() {
		dialog.ShowConfirm("Hapus semua riwayat", "Hapus semua riwayat chat lokal di perangkat ini?", func(ok bool) {
			if ok {
				_ = historyStore.DeleteAll()
				selectedConversationID = ""
				chatMessagesContainer.Objects = nil
				emptyLabel := widget.NewLabel("Pesan LAN akan muncul di sini...")
				emptyLabel.Alignment = fyne.TextAlignCenter
				chatMessagesContainer.Add(emptyLabel)
				chatMessagesContainer.Refresh()
				updateConversationListUI()
			}
		}, window)
	}

	window.SetMainMenu(fyne.NewMainMenu(
		groupMenu,
		fyne.NewMenu("Settings",
			fyne.NewMenuItem("Change display name", func() { showNameDialog(false) }),
			fyne.NewMenuItem("Delete conversation", deleteConversation),
			fyne.NewMenuItem("Delete all history", deleteAll),
		),
	))

	peersTitle := widget.NewLabel("Chats")
	peersTitle.TextStyle = fyne.TextStyle{Bold: true}

	createGroupBtn := widget.NewButtonWithIcon("New Group", theme.ContentAddIcon(), showCreateGroupDialog)
	joinGroupBtn := widget.NewButtonWithIcon("Join", theme.LoginIcon(), showJoinGroupDialog)

	sidebarActions := container.NewHBox(createGroupBtn, joinGroupBtn)

	leftPanel := container.NewBorder(
		container.NewVBox(peersTitle, sidebarActions),
		nil, nil, nil,
		conversationListWidget,
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

	var approveRequest func(string, string)
	var rejectRequest func(string, string)

	showJoinRequestsPlaceholder := func() {
		if joinRequestsDialog != nil {
			joinRequestsDialog.Hide()
			joinRequestsDialog = nil
		}

		groups := groupManager.AllGroups()

		var pendingRequests []pendingRequestItem

		for _, g := range groups {
			pr, exists := groupManager.GetPendingRequests(g.ID)
			if !exists || pr.Count() == 0 {
				continue
			}
			for _, req := range pr.All() {
				pendingRequests = append(pendingRequests, pendingRequestItem{
					GroupID:       g.ID,
					GroupName:     g.Name,
					RequesterID:   req.RequesterID,
					RequesterName: req.RequesterName,
				})
			}
		}

		content := container.NewVBox(
			widget.NewLabelWithStyle("Join Requests", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
		)

		if len(pendingRequests) == 0 {
			content.Add(widget.NewLabel("No pending join requests."))
		} else {
			for _, req := range pendingRequests {
				row := container.NewVBox(
					widget.NewLabelWithStyle(fmt.Sprintf("Group: %s", req.GroupName), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel(fmt.Sprintf("Requester: %s", req.RequesterName)),
					container.NewHBox(
						widget.NewButton("Approve", func() {
							approveRequest(req.GroupID, req.RequesterID)
						}),
						widget.NewButton("Reject", func() {
							rejectRequest(req.GroupID, req.RequesterID)
						}),
					),
					widget.NewSeparator(),
				)
				content.Add(row)
			}
		}

		joinRequestsDialog = dialog.NewCustom("Join Requests", "Tutup", content, window)
		joinRequestsDialog.SetOnClosed(func() {
			joinRequestsDialog = nil
		})
		joinRequestsDialog.Show()
	}

	approveRequest = func(groupID, requesterID string) {
		g, exists := groupManager.GetGroup(groupID)
		if !exists {
			dialog.ShowError(fmt.Errorf("group not found"), window)
			return
		}
		if err := groupManager.AcceptJoinRequest(groupID, requesterID, ident.ID); err != nil {
			dialog.ShowError(err, window)
			return
		}
		if err := groupPersistence.SaveGroup(g); err != nil {
			dialog.ShowError(err, window)
			return
		}
		peers.mu.Lock()
		requesterPeer, exists := peers.peers[requesterID]
		peers.mu.Unlock()
		if exists {
			_ = relay.SetMemberPeer(groupID, requesterID, requesterPeer.Address, requesterPeer.Port)
		}
		if err := sendJoinAccept(peers, &ident, g, requesterID, &pendingPeers); err != nil {
			dialog.ShowError(fmt.Errorf("approve succeeded but failed to notify requester: %v", err), window)
		}
		pendingPeers.Delete(requesterID)
		showJoinRequestsPlaceholder()
	}

	rejectRequest = func(groupID, requesterID string) {
		if err := groupManager.RejectJoinRequest(groupID, requesterID, ident.ID); err != nil {
			dialog.ShowError(err, window)
			return
		}
		if err := sendJoinReject(peers, &ident, requesterID, groupID, &pendingPeers); err != nil {
			dialog.ShowError(fmt.Errorf("reject succeeded but failed to notify requester: %v", err), window)
		}
		pendingPeers.Delete(requesterID)
		showJoinRequestsPlaceholder()
	}

	var menuButton *widget.Button

	menuButton = widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Settings", func() { showNameDialog(false) }),
			fyne.NewMenuItem("Join Requests", showJoinRequestsPlaceholder),
		)
		popup := widget.NewPopUpMenu(menu, window.Canvas())
		pos := menuButton.Position().Add(fyne.NewPos(0, menuButton.Size().Height))
		popup.ShowAtPosition(pos)
	})

	topBar := container.NewVBox(
		container.NewHBox(
			menuButton,
			appTitle,
			layout.NewSpacer(),
			refreshButton,
		),
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

	go func() {
		if err := discovery.Run(ctx); err != nil && ctx.Err() == nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
		}
	}()

	updateGroupAdvertisements(discovery, groupManager, ident.DisplayName, getLocalAddress(), server.Port())

	updateConversationListUI()

	go func() {
		if err := server.Run(ctx); err != nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case peer, ok := <-discovery.Peers():
				if !ok {
					return
				}
				peers.mu.Lock()
				peers.peers[peer.ID] = peer
				peers.mu.Unlock()

				if len(peer.Groups) > 0 {
					for _, ad := range peer.Groups {
						groupDiscovery.Upsert(ad)
					}
				}

				for _, g := range groupManager.AllGroups() {
					if g.Closed {
						continue
					}
					if g.HasMember(peer.ID) {
						_ = relay.SetMemberPeer(g.ID, peer.ID, peer.Address, peer.Port)
					}
				}

				fyne.Do(func() {
					updateConversationListUI()
				})
			}
		}
	}()

	time.AfterFunc(3*time.Second, func() {
		for _, g := range groupManager.AllGroups() {
			if g.Closed {
				continue
			}
			for memberID := range g.Members {
				peers.mu.Lock()
				peer, exists := peers.peers[memberID]
				peers.mu.Unlock()
				if exists {
					_ = relay.SetMemberPeer(g.ID, memberID, peer.Address, peer.Port)
				}
			}
		}
	})

	go func() {
		for im := range server.Incoming() {
			msg := im.Message
			fyne.Do(func() {
				switch msg.Type {
				case "group":
					handleIncomingGroupMessage(msg, groupManager, historyAdapter, notificationService, relay, &selectedConversationID, renderConversation, updateConversationListUI, ident.ID)
				case "join_request":
					handleIncomingJoinRequest(msg, groupManager, im.RemoteAddr, &pendingPeers)
				case "join_accept":
					handleIncomingJoinAccept(msg, groupManager, groupPersistence, relay, ident.ID)
				case "join_reject":
					handleIncomingJoinReject(msg, groupManager)
				default:
					peers.mu.Lock()
					knownPeer, exists := peers.peers[msg.FromID]
					peers.mu.Unlock()

					var peer network.Peer
					if exists {
						peer = knownPeer
					} else {
						peer = network.Peer{ID: msg.FromID, Name: msg.From}
						if peer.ID == "" {
							peer.ID = msg.From
						}
					}

					appendStoredMessage(history.DirectID(peer.ID), history.DirectConversation, peer.Name, history.Message{
						SenderID: peer.ID, SenderName: msg.From, Text: msg.Text, SentAt: msg.Time,
					})
					sendNotification(application, peer.Name, msg.Text)
				}
			})
		}
	}()

	go checkPeerTimeout(ctx, peers, updateConversationListUI)

	window.SetCloseIntercept(func() { cancel(); window.Close() })
	window.ShowAndRun()
}

// fyneNotifier implements group.Notifier using Fyne notifications.
type fyneNotifier struct {
	app fyne.App
}

func (n *fyneNotifier) SendNotification(title, content string) {
	n.app.SendNotification(&fyne.Notification{
		Title:   title,
		Content: content,
	})
}

func sendNotification(app fyne.App, title, content string) {
	app.SendNotification(&fyne.Notification{
		Title:   title,
		Content: content,
	})
}

func sendJoinAccept(peers *peerStore, ident *identity.Identity, g *group.Group, requesterID string, pendingPeers *sync.Map) error {
	peers.mu.Lock()
	peer, exists := peers.peers[requesterID]
	peers.mu.Unlock()
	if exists {
		msg := network.Message{
			Type:        "join_accept",
			GroupID:     g.ID,
			GroupName:   g.Name,
			GroupCode:   g.Code,
			JoinPolicy:  g.JoinPolicy,
			RequesterID: requesterID,
			From:        ident.DisplayName,
			FromID:      ident.ID,
			Time:        time.Now(),
		}
		return network.SendMessage(peer, msg)
	}

	if val, ok := pendingPeers.Load(requesterID); ok {
		pp := val.(pendingPeer)
		peer = network.Peer{
			ID:      requesterID,
			Address: pp.Address,
			Port:    pp.Port,
		}
		msg := network.Message{
			Type:        "join_accept",
			GroupID:     g.ID,
			GroupName:   g.Name,
			GroupCode:   g.Code,
			JoinPolicy:  g.JoinPolicy,
			RequesterID: requesterID,
			From:        ident.DisplayName,
			FromID:      ident.ID,
			Time:        time.Now(),
		}
		return network.SendMessage(peer, msg)
	}

	return fmt.Errorf("requester %s not found in peer store or pending peers", requesterID)
}

func sendJoinReject(peers *peerStore, ident *identity.Identity, requesterID, groupID string, pendingPeers *sync.Map) error {
	peers.mu.Lock()
	peer, exists := peers.peers[requesterID]
	peers.mu.Unlock()
	if exists {
		msg := network.Message{
			Type:        "join_reject",
			GroupID:     groupID,
			RequesterID: requesterID,
			From:        ident.DisplayName,
			FromID:      ident.ID,
			Time:        time.Now(),
		}
		return network.SendMessage(peer, msg)
	}

	if val, ok := pendingPeers.Load(requesterID); ok {
		pp := val.(pendingPeer)
		peer = network.Peer{
			ID:      requesterID,
			Address: pp.Address,
			Port:    pp.Port,
		}
		msg := network.Message{
			Type:        "join_reject",
			GroupID:     groupID,
			RequesterID: requesterID,
			From:        ident.DisplayName,
			FromID:      ident.ID,
			Time:        time.Now(),
		}
		return network.SendMessage(peer, msg)
	}

	return fmt.Errorf("requester %s not found in peer store or pending peers", requesterID)
}

func updateGroupAdvertisements(discovery *network.Discovery, gm *group.Manager, hostName string, hostAddr string, hostPort int) {
	var ads []group.GroupAdvertisement
	for _, g := range gm.AllGroups() {
		if !g.Closed {
			ads = append(ads, group.GroupAdvertisement{
				ID:         g.ID,
				Code:       g.Code,
				Name:       g.Name,
				HostID:     g.HostID,
				JoinPolicy: g.JoinPolicy,
				HostName:   hostName,
				HostAddr:   hostAddr,
				HostPort:   hostPort,
			})
		}
	}
	discovery.SetGroups(ads)
	discovery.Refresh()
}

func handleIncomingGroupMessage(
	msg network.Message,
	gm *group.Manager,
	adapter *group.HistoryAdapter,
	notifier *group.NotificationService,
	relay *group.Relay,
	selectedConvID *string,
	renderFn func(string),
	updateFn func(),
	localIdentityID string,
) {
	groupID := msg.GroupID
	g, exists := gm.GetGroup(groupID)
	if !exists {
		return
	}
	if g.Closed {
		return
	}
	if !g.HasMember(msg.SenderID) {
		return
	}

	gmsg := group.GroupMessage{
		MessageID:  msg.MessageID,
		GroupID:    msg.GroupID,
		SenderID:   msg.SenderID,
		SenderName: msg.SenderName,
		Body:       msg.Body,
		Timestamp:  msg.Time,
	}

	if err := adapter.SaveIncoming(g, gmsg); err != nil {
		return
	}

	_ = notifier.NotifyIncoming(g, gmsg)

	if g.IsHost(localIdentityID) {
		targets, err := relay.GetRelayTargets(groupID, msg.SenderID)
		if err == nil && len(targets) > 0 {
			relayMsg := network.Message{
				Type:       "group",
				MessageID:  msg.MessageID,
				GroupID:    msg.GroupID,
				SenderID:   msg.SenderID,
				SenderName: msg.SenderName,
				Body:       msg.Body,
				Time:       msg.Time,
			}
			for _, target := range targets {
				go func(addr string, port int) {
					_ = network.SendToAddress(addr, port, relayMsg)
				}(target.Address, target.Port)
			}
		}
	}

	if *selectedConvID == history.GroupID(groupID) {
		renderFn(*selectedConvID)
	} else {
		updateFn()
	}
}

func handleIncomingJoinRequest(msg network.Message, gm *group.Manager, remoteAddr string, pendingPeers *sync.Map) {
	groupID := msg.GroupID
	g, exists := gm.GetGroup(groupID)
	if !exists {
		return
	}
	if g.Closed {
		return
	}

	host, portStr, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		port, _ := strconv.Atoi(portStr)
		pendingPeers.Store(msg.RequesterID, pendingPeer{Address: host, Port: port})
	}

	req := group.JoinRequest{
		GroupID:       groupID,
		RequesterID:   msg.RequesterID,
		RequesterName: msg.RequesterName,
		Status:        group.Pending,
		Timestamp:     msg.Time,
	}
	_ = gm.AddJoinRequest(groupID, req)
}

func handleIncomingJoinAccept(msg network.Message, gm *group.Manager, persistence *group.Persistence, relay *group.Relay, localIdentityID string) {
	groupID := msg.GroupID
	g, exists := gm.GetGroup(groupID)

	if !exists {
		g = &group.Group{
			ID:         msg.GroupID,
			Code:       msg.GroupCode,
			Name:       msg.GroupName,
			HostID:     msg.FromID,
			JoinPolicy: msg.JoinPolicy,
			Members:    make(map[string]group.Member),
			CreatedAt:  time.Now(),
			Closed:     false,
		}
		g.Members[msg.FromID] = group.Member{ID: msg.FromID, Name: msg.From, Role: group.RoleHost, JoinedAt: time.Now()}
		g.Members[localIdentityID] = group.Member{ID: localIdentityID, Name: msg.RequesterName, Role: group.RoleMember, JoinedAt: time.Now()}

		if err := gm.AddGroup(g); err != nil {
			return
		}
		if err := relay.RegisterGroup(g); err != nil {
			return
		}
		if err := persistence.SaveGroup(g); err != nil {
			return
		}
		return
	}

	if g.Closed {
		return
	}

	if _, exists := g.GetMember(msg.FromID); !exists {
		_ = g.AddMember(msg.FromID, msg.From)
	}
	if _, exists := g.GetMember(msg.RequesterID); !exists {
		_ = g.AddMember(msg.RequesterID, msg.RequesterName)
	}

	_ = gm.AcceptJoinRequest(groupID, msg.RequesterID, g.HostID)
	_ = persistence.SaveGroup(g)
}

func handleIncomingJoinReject(msg network.Message, gm *group.Manager) {
	groupID := msg.GroupID
	g, exists := gm.GetGroup(groupID)
	if !exists {
		return
	}
	_ = gm.RejectJoinRequest(groupID, msg.RequesterID, g.HostID)
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
