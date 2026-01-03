import { forwardRef, useImperativeHandle, useRef, useCallback, memo, useEffect } from "react"
import { store } from "../../../wailsjs/go/models"
import { MessageItem } from "./MessageItem"

interface MessageListProps {
  chatId: string
  messages: store.Message[]
  sentMediaCache: React.MutableRefObject<Map<string, string>>
  onReply?: (message: store.Message) => void
  onLoadMore?: () => void
  onPrefetch?: () => void
  onTrimOldMessages?: () => void
  onRangeChanged?: (range: { startIndex: number; endIndex: number }) => void
  onAtBottomChange?: (atBottom: boolean) => void
  firstItemIndex: number
  isLoading?: boolean
  hasMore?: boolean
}

export interface MessageListHandle {
  scrollToBottom: (behavior?: "auto" | "smooth") => void
}

const MemoizedMessageItem = memo(MessageItem)

export const MessageList = forwardRef<MessageListHandle, MessageListProps>(function MessageList(
  {
    chatId,
    messages,
    sentMediaCache,
    onReply,
    onLoadMore,
    onPrefetch,
    onTrimOldMessages,
    onRangeChanged,
    onAtBottomChange,
    firstItemIndex,
    isLoading,
    hasMore,
  },
  ref,
) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const prefetchTriggeredRef = useRef(false)

  const renderItem = useCallback(
    (_: number, msg: store.Message) => (
      <div className="px-4 py-1">
        <MemoizedMessageItem
          message={msg}
          chatId={chatId}
          sentMediaCache={sentMediaCache}
          onReply={onReply}
        />
      </div>
    ),
    [chatId, onReply, sentMediaCache],
  )

  const scrollToBottom = useCallback(
    (behavior: "auto" | "smooth" = "smooth") => {
      const el = containerRef.current
      if (el) {
        const top = el.scrollHeight - el.clientHeight
        try {
          el.scrollTo({ top, behavior })
        } catch {
          el.scrollTop = top
        }
      }
    },
    [],
  )

  useImperativeHandle(ref, () => ({ scrollToBottom }))

  const handleStartReached = useCallback(() => {
    if (!isLoading && hasMore && onLoadMore) {
      onLoadMore()
      prefetchTriggeredRef.current = false
    }
  }, [isLoading, hasMore, onLoadMore])

  useEffect(() => {
    // Scroll to bottom on mount
    if (containerRef.current && messages.length > 0) {
      const el = containerRef.current
      el.scrollTop = el.scrollHeight
    }
  }, [])

  const onScroll = useCallback(
    (e: React.UIEvent<HTMLDivElement>) => {
      const el = e.currentTarget
      if (el.scrollTop <= 50) {
        handleStartReached()
      }
      const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 5
      onAtBottomChange?.(atBottom)
    },
    [handleStartReached, onAtBottomChange],
  )

  return (
    <div
      ref={containerRef}
      onScroll={onScroll}
      className="h-full overflow-y-auto bg-repeat virtuoso-scroller"
      style={{ backgroundImage: "url('/assets/images/bg-chat-tile-dark.png')" }}
    >
      <div className="flex justify-center py-4">
        {isLoading ? (
          <div className="animate-spin h-5 w-5 border-2 border-green-500 rounded-full border-t-transparent" />
        ) : null}
      </div>
      {messages.map((msg) => (
        <div key={msg.Info.ID} className="px-4 py-1">
          <MemoizedMessageItem
            message={msg}
            chatId={chatId}
            sentMediaCache={sentMediaCache}
            onReply={onReply}
          />
        </div>
      ))}
    </div>
  )
})
