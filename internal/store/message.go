package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	query "github.com/lugvitc/whats4linux/internal/db"
	"github.com/lugvitc/whats4linux/internal/misc"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type Reaction struct {
	ID        int    `json:"id"`
	MessageID string `json:"message_id"`
	SenderID  string `json:"sender_id"`
	Emoji     string `json:"emoji"`
}

type Message struct {
	Info      types.MessageInfo
	Content   *waE2E.Message
	Edited    bool
	Reactions []Reaction
}

// ChatMessage represents a chat in the chat list
type ChatMessage struct {
	JID         types.JID
	MessageText string
	MessageTime int64
	Sender      string
}

// chatListCache is an in-memory cache for the chat list
var chatListCache = misc.NewVMap[string, ChatMessage]()

// messageIDCache tracks processed message IDs to avoid duplicate processing
var messageIDCache = misc.NewVMap[string, uint8]()

// openDB opens a connection to messages.db
func openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", misc.GetSQLiteAddress("messages.db"))
	if err != nil {
		return nil, err
	}

	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA synchronous=NORMAL;`,
		`PRAGMA busy_timeout=5000;`,
		`PRAGMA foreign_keys=ON;`,
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, err
		}
	}

	return db, nil
}

// InitMessagesDB initializes the messages.db database
func InitMessagesDB() error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(query.CreateMessagesTable)
	return err
}

// ExtractMessageText extracts a text representation from a WhatsApp message
func ExtractMessageText(msg *waE2E.Message) string {
	if msg.GetConversation() != "" {
		return msg.GetConversation()
	} else if msg.GetExtendedTextMessage() != nil {
		return msg.GetExtendedTextMessage().GetText()
	} else {
		switch {
		case msg.GetImageMessage() != nil:
			return "image"
		case msg.GetVideoMessage() != nil:
			return "video"
		case msg.GetAudioMessage() != nil:
			return "audio"
		case msg.GetDocumentMessage() != nil:
			return "document"
		case msg.GetStickerMessage() != nil:
			return "sticker"
		default:
			return "message"
		}
	}
}

func updateCanonicalJID(ctx context.Context, js store.LIDStore, jid *types.JID) (changed bool) {
	if jid == nil {
		return
	}
	if jid.ActualAgent() != types.LIDDomain {
		return
	}
	canonicalJID, err := js.GetPNForLID(ctx, *jid)
	if err != nil {
		log.Println("Failed to get PN for LID:", err)
		return
	}
	changed = true
	*jid = canonicalJID
	return
}

// ProcessMessageEvent processes a new message event and stores it in messages.db
func ProcessMessageEvent(ctx context.Context, sd store.LIDStore, msg *events.Message) {
	updateCanonicalJID(ctx, sd, &msg.Info.Chat)
	updateCanonicalJID(ctx, sd, &msg.Info.Sender)

	// Handle message edits
	if protoMsg := msg.Message.GetProtocolMessage(); protoMsg != nil && protoMsg.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT {
		targetID := protoMsg.GetKey().GetID()
		newContent := protoMsg.GetEditedMessage()
		if targetID == "" || newContent == nil {
			return
		}

		err := UpdateMessageContent(targetID, newContent)
		if err != nil {
			log.Println("Failed to update edited message:", err)
		}
		return
	}

	chat := msg.Info.Chat.User

	m := Message{
		Info:    msg.Info,
		Content: msg.Message,
		Edited:  false,
	}

	// Update chatListCache with the new latest message
	messageText := ExtractMessageText(m.Content)
	sender := msg.Info.PushName
	if sender == "" && msg.Info.Sender.User != "" {
		sender = msg.Info.Sender.User
	}

	if msg.Info.IsFromMe {
		sender = "You"
	}

	chatMsg := ChatMessage{
		JID:         msg.Info.Chat,
		MessageText: messageText,
		MessageTime: msg.Info.Timestamp.Unix(),
		Sender:      sender,
	}
	chatListCache.Set(chat, chatMsg)

	// Check if message already processed
	if _, exists := messageIDCache.Get(msg.Info.ID); exists {
		// Update existing message
		err := UpdateMessageContent(msg.Info.ID, m.Content)
		if err != nil {
			log.Println("Failed to update message:", err)
		}
		return
	}

	messageIDCache.Set(msg.Info.ID, 1)
	err := InsertMessage(&m)
	if err != nil {
		log.Println("Failed to insert message:", err)
	}
}

// InsertMessage inserts a new message into messages.db
func InsertMessage(msg *Message) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Handle reaction messages differently
	if msg.Content.GetReactionMessage() != nil {
		reactionMsg := msg.Content.GetReactionMessage()
		targetID := reactionMsg.GetKey().GetID()
		reaction := reactionMsg.GetText()
		senderJID := msg.Info.Sender.String()

		return AddReactionToMessage(targetID, reaction, senderJID)
	}

	var msgType query.MessageType = query.MessageTypeText
	var mediaType query.MediaType
	var text string
	var mentions string

	// Extract mentions if it's an extended text message
	var replyToMessageID string
	if msg.Content.GetExtendedTextMessage() != nil && msg.Content.GetExtendedTextMessage().GetContextInfo() != nil {
		if mentioned := msg.Content.GetExtendedTextMessage().GetContextInfo().GetMentionedJID(); len(mentioned) > 0 {
			mentionsBytes, _ := json.Marshal(mentioned)
			mentions = string(mentionsBytes)
		}
		replyToMessageID = msg.Content.GetExtendedTextMessage().GetContextInfo().GetStanzaID()
	}

	// Serialize raw message for media types

	if msg.Content.GetConversation() != "" {
		text = msg.Content.GetConversation()
	} else if msg.Content.GetExtendedTextMessage() != nil {
		text = msg.Content.GetExtendedTextMessage().GetText()
	} else {
		switch {
		case msg.Content.GetImageMessage() != nil:
			msgType = query.MessageTypeImage
			mediaType = query.MediaTypeImage
			text = msg.Content.GetImageMessage().GetCaption()
			if text == "" {
				text = "image"
			}
		case msg.Content.GetVideoMessage() != nil:
			msgType = query.MessageTypeVideo
			mediaType = query.MediaTypeVideo
			text = msg.Content.GetVideoMessage().GetCaption()
			if text == "" {
				text = "video"
			}
		case msg.Content.GetAudioMessage() != nil:
			msgType = query.MessageTypeAudio
			mediaType = query.MediaTypeAudio
			text = "audio"
		case msg.Content.GetDocumentMessage() != nil:
			msgType = query.MessageTypeDocument
			mediaType = query.MediaTypeDocument
			text = msg.Content.GetDocumentMessage().GetFileName()
			if text == "" {
				text = "document"
			}
		case msg.Content.GetStickerMessage() != nil:
			msgType = query.MessageTypeSticker
			mediaType = query.MediaTypeSticker
			text = "sticker"
		default:
			// Log unsupported message type with detailed information
			log.Printf("Skipping unsupported message type for message ID %s in chat %s", msg.Info.ID, msg.Info.Chat.String())
			log.Printf("Message content details:")
			if msg.Content.GetConversation() != "" {
				log.Printf("  - Has conversation: %s", msg.Content.GetConversation())
			}
			if msg.Content.GetExtendedTextMessage() != nil {
				log.Printf("  - Has extended text: %s", msg.Content.GetExtendedTextMessage().GetText())
			}
			if msg.Content.GetImageMessage() != nil {
				log.Printf("  - Has image message")
			}
			if msg.Content.GetVideoMessage() != nil {
				log.Printf("  - Has video message")
			}
			if msg.Content.GetAudioMessage() != nil {
				log.Printf("  - Has audio message")
			}
			if msg.Content.GetDocumentMessage() != nil {
				log.Printf("  - Has document message")
			}
			if msg.Content.GetStickerMessage() != nil {
				log.Printf("  - Has sticker message")
			}
			if msg.Content.GetContactMessage() != nil {
				log.Printf("  - Has contact message")
			}
			if msg.Content.GetLocationMessage() != nil {
				log.Printf("  - Has location message")
			}
			if msg.Content.GetLiveLocationMessage() != nil {
				log.Printf("  - Has live location message")
			}
			if msg.Content.GetPollCreationMessage() != nil {
				log.Printf("  - Has poll creation message")
			}
			if msg.Content.GetPollUpdateMessage() != nil {
				log.Printf("  - Has poll update message")
			}
			if msg.Content.GetProtocolMessage() != nil {
				log.Printf("  - Has protocol message (type: %v)", msg.Content.GetProtocolMessage().GetType())
			}
			if msg.Content.GetReactionMessage() != nil {
				log.Printf("  - Has reaction message")
			}
			if msg.Content.GetSenderKeyDistributionMessage() != nil {
				log.Printf("  - Has sender key distribution message")
			}
			log.Printf("Full message content: %+v", msg.Content)
			return nil
		}
	}

	_, err = db.Exec(query.InsertDecodedMessage,
		msg.Info.ID,
		msg.Info.Chat.String(),
		msg.Info.Sender.String(),
		msg.Info.Timestamp.Unix(),
		msg.Info.IsFromMe,
		msgType,
		text,
		mediaType,
		replyToMessageID,
		mentions,
		msg.Edited,
	)

	return err
}

// UpdateMessageContent updates an existing message's content
func UpdateMessageContent(messageID string, content *waE2E.Message) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	var msgType query.MessageType = query.MessageTypeText
	var text string

	if content.GetConversation() != "" {
		text = content.GetConversation()
	} else if content.GetExtendedTextMessage() != nil {
		text = content.GetExtendedTextMessage().GetText()
	} else {
		switch {
		case content.GetImageMessage() != nil:
			msgType = query.MessageTypeImage
			text = content.GetImageMessage().GetCaption()
			if text == "" {
				text = "image"
			}
		case content.GetVideoMessage() != nil:
			msgType = query.MessageTypeVideo
			text = content.GetVideoMessage().GetCaption()
			if text == "" {
				text = "video"
			}
		case content.GetAudioMessage() != nil:
			msgType = query.MessageTypeAudio
			text = "audio"
		case content.GetDocumentMessage() != nil:
			msgType = query.MessageTypeDocument
			text = content.GetDocumentMessage().GetFileName()
			if text == "" {
				text = "document"
			}
		case content.GetStickerMessage() != nil:
			msgType = query.MessageTypeSticker
			text = "sticker"
		default:
			text = "message"
		}
	}

	_, err = db.Exec(query.UpdateDecodedMessage,
		text,
		msgType,
		messageID,
	)

	return err
}

// GetMessageWithRaw returns a message with its raw protobuf content for media download
func GetMessageWithRaw(chatJID string, messageID string) (*Message, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var (
		id               string
		chat             string
		sender           string
		timestamp        int64
		isFromMe         bool
		msgType          int
		text             sql.NullString
		mediaType        sql.NullInt64
		replyTo          sql.NullString
		mentions         sql.NullString
		edited           bool
	)

	err = db.QueryRow(query.SelectMessageWithRawByChatAndID, chatJID, messageID).Scan(
		&id,
		&chat,
		&sender,
		&timestamp,
		&isFromMe,
		&msgType,
		&text,
		&mediaType,
		&replyTo,
		&mentions,
		&edited,
	)

	if err != nil {
		return nil, err
	}

	chatParsed, _ := types.ParseJID(chat)
	senderParsed, _ := types.ParseJID(sender)

	msg := &Message{
		Info: types.MessageInfo{
			ID:        id,
			Timestamp: time.Unix(timestamp, 0),
			MessageSource: types.MessageSource{
				Chat:     chatParsed,
				Sender:   senderParsed,
				IsFromMe: isFromMe,
			},
		},
		Edited: edited,
	}

	// Note: Raw message content is no longer stored, so Content will be nil
	// Media downloads will not work without raw message content

	return msg, nil
}

// GetMessageByIDWithRaw returns a message by ID with its raw protobuf content
func GetMessageByIDWithRaw(messageID string) (*Message, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var (
		id               string
		chat             string
		sender           string
		timestamp        int64
		isFromMe         bool
		msgType          int
		text             sql.NullString
		mediaType        sql.NullInt64
		replyTo          sql.NullString
		mentions         sql.NullString
		edited           bool
	)

	err = db.QueryRow(query.SelectMessageWithRawByID, messageID).Scan(
		&id,
		&chat,
		&sender,
		&timestamp,
		&isFromMe,
		&msgType,
		&text,
		&mediaType,
		&replyTo,
		&mentions,
		&edited,
	)

	if err != nil {
		return nil, err
	}

	chatParsed, _ := types.ParseJID(chat)
	senderParsed, _ := types.ParseJID(sender)

	msg := &Message{
		Info: types.MessageInfo{
			ID:        id,
			Timestamp: time.Unix(timestamp, 0),
			MessageSource: types.MessageSource{
				Chat:     chatParsed,
				Sender:   senderParsed,
				IsFromMe: isFromMe,
			},
		},
		Edited: edited,
	}

	// Note: Raw message content is no longer stored, so Content will be nil
	// Media downloads will not work without raw message content

	return msg, nil
}

// GetChatList returns the chat list from messages.db
func GetChatList() []ChatMessage {
	db, err := openDB()
	if err != nil {
		return []ChatMessage{}
	}
	defer db.Close()

	rows, err := db.Query(query.SelectDecodedChatList)
	if err != nil {
		return []ChatMessage{}
	}
	defer rows.Close()

	var chatList []ChatMessage

	for rows.Next() {
		var (
			messageID string
			chatJID   string
			senderJID string
			timestamp int64
			isFromMe  bool
			msgType   int
			text      sql.NullString
			mediaType sql.NullInt64
			replyTo   sql.NullString
			mentions  sql.NullString
			edited    bool
		)

		if err := rows.Scan(
			&messageID,
			&chatJID,
			&senderJID,
			&timestamp,
			&isFromMe,
			&msgType,
			&text,
			&mediaType,
			&replyTo,
			&mentions,
			&edited,
		); err != nil {
			continue
		}

		jid, err := types.ParseJID(chatJID)
		if err != nil {
			continue
		}

		// Check per-chat cache first
		if cachedChat, ok := chatListCache.Get(jid.User); ok {
			chatList = append(chatList, cachedChat)
			continue
		}

		var messageText string
		if text.Valid {
			messageText = text.String
		}

		// Determine sender for display
		sender := ""
		if isFromMe {
			sender = "You"
		}

		chatMsg := ChatMessage{
			JID:         jid,
			MessageText: messageText,
			MessageTime: timestamp,
			Sender:      sender,
		}

		// Cache per-chat entry
		chatListCache.Set(jid.User, chatMsg)
		chatList = append(chatList, chatMsg)
	}

	return chatList
}

// MigrateLIDToPNForMessagesDB migrates LID JIDs to PN JIDs in messages.db
func MigrateLIDToPNForMessagesDB(ctx context.Context, sd store.LIDStore) error {
	log.Println("Starting LID to PN migration for messages.db...")

	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	log.Println("Fetching all messages for migration...")
	defer log.Println("Migration task completed.")

	rows, err := db.Query(query.SelectAllMessagesJIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmtUpdate, err := db.Prepare(query.UpdateMessageJIDs)
	if err != nil {
		return err
	}
	defer stmtUpdate.Close()

	var (
		messageID    string
		chatJIDStr   string
		senderJIDStr string
		oC, oS       string
	)

	for rows.Next() {
		if err := rows.Scan(&messageID, &chatJIDStr, &senderJIDStr); err != nil {
			continue
		}

		chatJID, err := types.ParseJID(chatJIDStr)
		if err != nil {
			log.Println("Failed to parse chat JID:", err)
			continue
		}

		senderJID, err := types.ParseJID(senderJIDStr)
		if err != nil {
			log.Println("Failed to parse sender JID:", err)
			continue
		}

		oC = chatJID.String()
		oS = senderJID.String()

		cc := updateCanonicalJID(ctx, sd, &chatJID)
		sc := updateCanonicalJID(ctx, sd, &senderJID)

		if !cc && !sc {
			continue
		}

		_, err = stmtUpdate.Exec(
			chatJID.String(),
			senderJID.String(),
			messageID,
		)

		if err != nil {
			log.Println("Failed to update message during LID to PN migration:", err)
			continue
		}

		if cc {
			log.Printf("Migrated message %s chat from LID %s to PN %s\n",
				messageID, oC, chatJID.String())
		}
		if sc {
			log.Printf("Migrated message %s sender from LID %s to PN %s\n",
				messageID, oS, senderJID.String())
		}
	}
	return nil
}

// GetReactionsByMessageID returns all reactions for a message
func GetReactionsByMessageID(messageID string) ([]Reaction, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query.SelectReactionsByMessageID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []Reaction
	for rows.Next() {
		var reaction Reaction
		err := rows.Scan(&reaction.ID, &reaction.MessageID, &reaction.SenderID, &reaction.Emoji)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, nil
}

// AddReactionToMessage adds or removes a reaction to/from a message
func AddReactionToMessage(targetID, reaction, senderJID string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// If reaction is empty, remove all reactions from this sender for this message
	if reaction == "" {
		_, err = db.Exec(`DELETE FROM reactions WHERE message_id = ? AND sender_id = ?`, targetID, senderJID)
		return err
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete any existing reaction from this sender for this message
	_, err = tx.Exec(`DELETE FROM reactions WHERE message_id = ? AND sender_id = ?`, targetID, senderJID)
	if err != nil {
		return err
	}

	// Insert the new reaction
	_, err = tx.Exec(query.InsertReaction, targetID, senderJID, reaction)
	if err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit()
}

// DecodedMessage represents a message from messages.db with decoded fields
type DecodedMessage struct {
	MessageID        string     `json:"message_id"`
	ChatJID          string     `json:"chat_jid"`
	SenderJID        string     `json:"sender_jid"`
	Timestamp        int64      `json:"timestamp"`
	IsFromMe         bool       `json:"is_from_me"`
	Type             int        `json:"type"`
	Text             string     `json:"text"`
	MediaType        int        `json:"media_type"`
	ReplyToMessageID string     `json:"reply_to_message_id"`
	Mentions         string     `json:"mentions"`
	Edited           bool       `json:"edited"`
	Reactions        []Reaction `json:"reactions"`
	// Info provides compatibility with frontend that expects types.MessageInfo structure
	Info DecodedMessageInfo `json:"Info"`
	// Content provides a minimal content structure for frontend rendering
	Content *DecodedMessageContent `json:"Content"`
}

// DecodedMessageInfo is a simplified MessageInfo for frontend compatibility
type DecodedMessageInfo struct {
	ID        string `json:"ID"`
	Timestamp string `json:"Timestamp"`
	IsFromMe  bool   `json:"IsFromMe"`
	PushName  string `json:"PushName"`
	Sender    string `json:"Sender"`
	Chat      string `json:"Chat"`
}

// DecodedMessageContent provides minimal content info for frontend rendering
type DecodedMessageContent struct {
	Conversation        string                  `json:"conversation,omitempty"`
	ExtendedTextMessage *ExtendedTextContent    `json:"extendedTextMessage,omitempty"`
	ImageMessage        *MediaMessageContent    `json:"imageMessage,omitempty"`
	VideoMessage        *MediaMessageContent    `json:"videoMessage,omitempty"`
	AudioMessage        *MediaMessageContent    `json:"audioMessage,omitempty"`
	DocumentMessage     *DocumentMessageContent `json:"documentMessage,omitempty"`
	StickerMessage      *MediaMessageContent    `json:"stickerMessage,omitempty"`
}

type ExtendedTextContent struct {
	Text        string       `json:"text,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type MediaMessageContent struct {
	Caption     string       `json:"caption,omitempty"`
	Mimetype    string       `json:"mimetype,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type DocumentMessageContent struct {
	Caption     string       `json:"caption,omitempty"`
	FileName    string       `json:"fileName,omitempty"`
	Mimetype    string       `json:"mimetype,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type ContextInfo struct {
	StanzaID      string `json:"stanzaId,omitempty"`
	Participant   string `json:"participant,omitempty"`
	QuotedMessage any    `json:"quotedMessage,omitempty"`
}

// GetDecodedMessagesPaged returns a page of decoded messages from messages.db
func GetDecodedMessagesPaged(chatJID string, beforeTimestamp int64, limit int) ([]DecodedMessage, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var rows *sql.Rows

	if beforeTimestamp == 0 {
		rows, err = db.Query(query.SelectLatestDecodedMessagesByChat, chatJID, limit)
	} else {
		rows, err = db.Query(query.SelectDecodedMessagesByChatBeforeTimestamp, chatJID, beforeTimestamp, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []DecodedMessage

	for rows.Next() {
		var msg DecodedMessage
		var mediaType sql.NullInt64
		var replyTo sql.NullString
		var mentions sql.NullString
		var text sql.NullString

		err := rows.Scan(
			&msg.MessageID,
			&msg.ChatJID,
			&msg.SenderJID,
			&msg.Timestamp,
			&msg.IsFromMe,
			&msg.Type,
			&text,
			&mediaType,
			&replyTo,
			&mentions,
			&msg.Edited,
		)
		if err != nil {
			log.Println("Failed to scan decoded message:", err)
			continue
		}

		if text.Valid {
			msg.Text = text.String
		}
		if mediaType.Valid {
			msg.MediaType = int(mediaType.Int64)
		}
		if replyTo.Valid {
			msg.ReplyToMessageID = replyTo.String
		}
		if mentions.Valid {
			msg.Mentions = mentions.String
		}

		// Load reactions for this message
		reactions, err := GetReactionsByMessageID(msg.MessageID)
		if err == nil {
			msg.Reactions = reactions
		}

		// Populate Info for frontend compatibility
		msg.Info = DecodedMessageInfo{
			ID:        msg.MessageID,
			Timestamp: time.Unix(msg.Timestamp, 0).Format(time.RFC3339),
			IsFromMe:  msg.IsFromMe,
			PushName:  "",
			Sender:    msg.SenderJID,
			Chat:      msg.ChatJID,
		}

		// Populate Content for frontend rendering
		msg.Content = buildDecodedContent(&msg)

		messages = append(messages, msg)
	}

	return messages, nil
}

// buildDecodedContent creates a DecodedMessageContent from DecodedMessage fields
func buildDecodedContent(msg *DecodedMessage) *DecodedMessageContent {
	content := &DecodedMessageContent{}

	// Build context info if there's a reply
	var contextInfo *ContextInfo
	if msg.ReplyToMessageID != "" {
		contextInfo = &ContextInfo{
			StanzaID: msg.ReplyToMessageID,
		}
	}

	// Based on message type, populate the appropriate content field
	switch query.MessageType(msg.Type) {
	case query.MessageTypeText:
		if contextInfo != nil {
			content.ExtendedTextMessage = &ExtendedTextContent{
				Text:        msg.Text,
				ContextInfo: contextInfo,
			}
		} else {
			content.Conversation = msg.Text
		}
	case query.MessageTypeImage:
		content.ImageMessage = &MediaMessageContent{
			Caption:     msg.Text,
			ContextInfo: contextInfo,
		}
	case query.MessageTypeVideo:
		content.VideoMessage = &MediaMessageContent{
			Caption:     msg.Text,
			ContextInfo: contextInfo,
		}
	case query.MessageTypeAudio:
		content.AudioMessage = &MediaMessageContent{
			ContextInfo: contextInfo,
		}
	case query.MessageTypeDocument:
		content.DocumentMessage = &DocumentMessageContent{
			FileName:    msg.Text,
			ContextInfo: contextInfo,
		}
	case query.MessageTypeSticker:
		content.StickerMessage = &MediaMessageContent{
			ContextInfo: contextInfo,
		}
	default:
		content.Conversation = msg.Text
	}

	return content
}

// GetDecodedMessage returns a single decoded message from messages.db
func GetDecodedMessage(chatJID string, messageID string) (*DecodedMessage, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var msg DecodedMessage
	var mediaType sql.NullInt64
	var replyTo sql.NullString
	var mentions sql.NullString
	var text sql.NullString

	err = db.QueryRow(query.SelectDecodedMessageByChatAndID, chatJID, messageID).Scan(
		&msg.MessageID,
		&msg.ChatJID,
		&msg.SenderJID,
		&msg.Timestamp,
		&msg.IsFromMe,
		&msg.Type,
		&text,
		&mediaType,
		&replyTo,
		&mentions,
		&msg.Edited,
	)

	if err != nil {
		return nil, err
	}

	if text.Valid {
		msg.Text = text.String
	}
	if mediaType.Valid {
		msg.MediaType = int(mediaType.Int64)
	}
	if replyTo.Valid {
		msg.ReplyToMessageID = replyTo.String
	}
	if mentions.Valid {
		msg.Mentions = mentions.String
	}

	// Load reactions
	reactions, err := GetReactionsByMessageID(msg.MessageID)
	if err == nil {
		msg.Reactions = reactions
	}

	// Populate Info for frontend compatibility
	msg.Info = DecodedMessageInfo{
		ID:        msg.MessageID,
		Timestamp: time.Unix(msg.Timestamp, 0).Format(time.RFC3339),
		IsFromMe:  msg.IsFromMe,
		PushName:  "",
		Sender:    msg.SenderJID,
		Chat:      msg.ChatJID,
	}

	// Populate Content for frontend rendering
	msg.Content = buildDecodedContent(&msg)

	return &msg, nil
}

// GetDecodedChatList returns the chat list from messages.db with the latest message for each chat
func GetDecodedChatList() ([]DecodedMessage, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query.SelectDecodedChatList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []DecodedMessage

	for rows.Next() {
		var msg DecodedMessage
		var mediaType sql.NullInt64
		var replyTo sql.NullString
		var mentions sql.NullString
		var text sql.NullString

		err := rows.Scan(
			&msg.MessageID,
			&msg.ChatJID,
			&msg.SenderJID,
			&msg.Timestamp,
			&msg.IsFromMe,
			&msg.Type,
			&text,
			&mediaType,
			&replyTo,
			&mentions,
			&msg.Edited,
		)
		if err != nil {
			log.Println("Failed to scan decoded message for chat list:", err)
			continue
		}

		if text.Valid {
			msg.Text = text.String
		}
		if mediaType.Valid {
			msg.MediaType = int(mediaType.Int64)
		}
		if replyTo.Valid {
			msg.ReplyToMessageID = replyTo.String
		}
		if mentions.Valid {
			msg.Mentions = mentions.String
		}

		// Populate Info for frontend compatibility
		msg.Info = DecodedMessageInfo{
			ID:        msg.MessageID,
			Timestamp: time.Unix(msg.Timestamp, 0).Format(time.RFC3339),
			IsFromMe:  msg.IsFromMe,
			PushName:  "",
			Sender:    msg.SenderJID,
			Chat:      msg.ChatJID,
		}

		// Populate Content for frontend rendering
		msg.Content = buildDecodedContent(&msg)

		messages = append(messages, msg)
	}

	return messages, nil
}
