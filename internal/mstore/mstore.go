package mstore

import (
	"sort"

	"github.com/lugvitc/whats4linux/internal/misc"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type Message struct {
	Info    types.MessageInfo
	Content *waE2E.Message
}

type MessageStore struct {
	msgMap misc.VMap[types.JID, []Message]
	mCache misc.VMap[string, uint8]
}

func NewMessageStore() *MessageStore {
	return &MessageStore{
		msgMap: misc.NewVMap[types.JID, []Message](),
		mCache: misc.NewVMap[string, uint8](),
	}
}

func (ms *MessageStore) ProcessMessageEvent(msg *events.Message) {
	if _, exists := ms.mCache.Get(msg.Info.ID); exists {
		return
	}
	ms.mCache.Set(msg.Info.ID, 1)
	chat := msg.Info.Chat
	ml, _ := ms.msgMap.Get(chat)
	ml = append(ml, Message{
		Info:    msg.Info,
		Content: msg.Message,
	})
	ms.msgMap.Set(chat, ml)
}

func (ms *MessageStore) GetMessages(jid types.JID) []Message {
	ml, _ := ms.msgMap.Get(jid)
	return ml
}

func (ms *MessageStore) GetMessage(chatJID types.JID, messageID string) *Message {
	msgs, ok := ms.msgMap.Get(chatJID)
	if !ok {
		return nil
	}
	for _, msg := range msgs {
		if msg.Info.ID == messageID {
			return &msg
		}
	}
	return nil
}

type ChatMessage struct {
	JID         types.JID
	MessageText string
	MessageTime int64
}

func (ms *MessageStore) GetChatList() []ChatMessage {
	var chatList []ChatMessage
	msgMap, mu := ms.msgMap.GetMapWithMutex()
	mu.RLock()
	defer mu.RUnlock()
	for jid, messages := range msgMap {
		if len(messages) == 0 {
			continue
		}
		latestMsg := messages[len(messages)-1]
		var messageText string
		if latestMsg.Content.GetConversation() != "" {
			messageText = latestMsg.Content.GetConversation()
		} else if latestMsg.Content.GetExtendedTextMessage() != nil {
			messageText = latestMsg.Content.GetExtendedTextMessage().GetText()
		} else {
			switch {
			case latestMsg.Content.GetImageMessage() != nil:
				messageText = "image"
			case latestMsg.Content.GetVideoMessage() != nil:
				messageText = "video"
			case latestMsg.Content.GetAudioMessage() != nil:
				messageText = "audio"
			case latestMsg.Content.GetDocumentMessage() != nil:
				messageText = "document"
			case latestMsg.Content.GetStickerMessage() != nil:
				messageText = "sticker"
			default:
				messageText = "unsupported message type"
			}
		}
		chatList = append(chatList, ChatMessage{
			JID:         jid,
			MessageText: messageText,
			MessageTime: latestMsg.Info.Timestamp.Unix(),
		})
	}
	sort.Slice(chatList, func(i, j int) bool {
		return chatList[i].MessageTime > chatList[j].MessageTime
	})
	return chatList
}
