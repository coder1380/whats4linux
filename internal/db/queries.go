package query

const (
	// Message types
	MessageTypeText     uint8 = 1
	MessageTypeImage    uint8 = 2
	MessageTypeVideo    uint8 = 3
	MessageTypeAudio    uint8 = 4
	MessageTypeDocument uint8 = 5
	MessageTypeSticker  uint8 = 6
	MessageTypeContact  uint8 = 7

	CreateGroupsTable = `
	CREATE TABLE IF NOT EXISTS whats4linux_groups (
		jid TEXT PRIMARY KEY,
		name TEXT,
		topic TEXT,
		owner_jid TEXT,
		participant_count INTEGER
	);
	`

	InsertOrReplaceGroup = `
	INSERT OR REPLACE INTO whats4linux_groups
	(jid, name, topic, owner_jid, participant_count)
	VALUES (?, ?, ?, ?, ?);
	`

	SelectAllGroups = `
	SELECT jid, name, topic, owner_jid, participant_count
	FROM whats4linux_groups;
	`

	SelectGroupByJID = `
	SELECT jid, name, topic, owner_jid, participant_count
	FROM whats4linux_groups
	WHERE jid = ?;
	`

	CreateSchema = `
	CREATE TABLE IF NOT EXISTS messages (
		chat TEXT NOT NULL,
		message_id TEXT PRIMARY KEY,
		timestamp INTEGER,
		msg_info BLOB,
		raw_message BLOB
	);

	CREATE INDEX IF NOT EXISTS idx_messages_chat_time
	ON messages(chat, timestamp DESC);
	`

	InsertMessage = `
	INSERT INTO messages
	(chat, message_id, timestamp, msg_info, raw_message)
	VALUES (?, ?, ?, ?, ?)
	`

	UpdateMessage = `
	UPDATE messages
	SET msg_info = ?, raw_message = ?
	WHERE message_id = ?;
	`

	UpdateMessageInfo = `
	UPDATE messages
	SET msg_info = ?
	WHERE message_id = ?;
	`

	SelectAllMessagesInfo = `
	SELECT msg_info
	FROM messages;
	`

	SelectChatList = `
	SELECT chat, timestamp, msg_info, raw_message
	FROM (
		SELECT 
			chat, timestamp, msg_info, raw_message,
			ROW_NUMBER() OVER (
				PARTITION BY chat
				ORDER BY timestamp DESC, rowid DESC
			) AS rn
		FROM messages
	)
	WHERE rn = 1
	ORDER BY timestamp DESC;
	`

	SelectMessagesByChatBeforeTimestamp = `
	SELECT msg_info, raw_message, timestamp
	FROM (
		SELECT msg_info, raw_message, timestamp
		FROM messages
		WHERE chat = ? AND timestamp < ?
		ORDER BY timestamp DESC
		LIMIT ?
	)
	ORDER BY timestamp ASC
	`

	SelectLatestMessagesByChat = `
	SELECT msg_info, raw_message, timestamp
	FROM (
		SELECT msg_info, raw_message, timestamp
		FROM messages
		WHERE chat = ?
		ORDER BY timestamp DESC
		LIMIT ?
	)
	ORDER BY timestamp ASC
	`

	SelectMessageByChatAndID = `
	SELECT msg_info, raw_message
	FROM messages
	WHERE chat = ? AND message_id = ?
	LIMIT 1
	`

	SelectMessageByID = `
	SELECT chat, message_id, timestamp, msg_info, raw_message
	FROM messages
	WHERE message_id = ?
	LIMIT 1
	`

	// Image cache queries
	CreateImageIndexTable = `
	CREATE TABLE IF NOT EXISTS image_index (
		message_id TEXT PRIMARY KEY,
		sha256     TEXT NOT NULL,
		mime       TEXT,
		width      INTEGER,
		height     INTEGER,
		created_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_sha ON image_index (sha256);
	`

	SaveImageIndex = `
	INSERT OR REPLACE INTO image_index
	(message_id, sha256, mime, width, height, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	DeleteImageIndex = `
	DELETE FROM image_index
	WHERE message_id = ?
	`

	GetImageByID = `
	SELECT message_id, sha256, mime, width, height, created_at
	FROM image_index
	WHERE message_id = ?
	`

	// GetImagesByIDsPrefix is the prefix used to query multiple image IDs.
	// Use it with a dynamically built placeholder list, e.g.
	// q := query.GetImagesByIDsPrefix + strings.Join(placeholders, ",") + ")"
	GetImagesByIDsPrefix = `
	SELECT message_id, sha256, mime, width, height, created_at
	FROM image_index
	WHERE message_id IN (
	`

	// Messages database queries
	CreateMessagesTable = `
	CREATE TABLE IF NOT EXISTS messages (
		message_id TEXT PRIMARY KEY,
		chat_jid TEXT NOT NULL,
		sender_jid TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		is_from_me BOOLEAN NOT NULL,
		type INTEGER NOT NULL,
		text TEXT,
		media_type TEXT,
		mentions TEXT,
		edited BOOLEAN DEFAULT FALSE,
		reactions TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_messages_chat_jid ON messages(chat_jid);
	CREATE INDEX IF NOT EXISTS idx_messages_sender_jid ON messages(sender_jid);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp DESC);
	`

	InsertDecodedMessage = `
	INSERT OR REPLACE INTO messages 
	(message_id, chat_jid, sender_jid, timestamp, is_from_me, type, text, media_type, mentions, edited, reactions)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	SelectDecodedMessageByID = `
	SELECT message_id, chat_jid, sender_jid, timestamp, is_from_me, type, text, media_type, mentions, edited, reactions
	FROM messages
	WHERE message_id = ?
	`

	UpdateDecodedMessage = `
	UPDATE messages
	SET text = ?, type = ?, edited = TRUE
	WHERE message_id = ?
	`

	UpdateMessageReactions = `
	UPDATE messages
	SET reactions = ?
	WHERE message_id = ?
	`
)
