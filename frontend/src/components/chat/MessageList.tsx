import { Virtuoso, type VirtuosoHandle } from "react-virtuoso"
import { forwardRef, useImperativeHandle, useRef, useCallback, memo } from "react"
import { store } from "../../../wailsjs/go/models"
import { MessageItem } from "./MessageItem"
import clsx from "clsx"

interface MessageListProps {
  chatId: string
  messages: store.Message[]
  sentMediaCache: React.MutableRefObject<Map<string, string>>
  onReply?: (message: store.Message) => void
  onQuotedClick?: (messageId: string) => void
  onLoadMore?: () => void
  onPrefetch?: () => void
  onTrimOldMessages?: () => void
  onRangeChanged?: (range: { startIndex: number; endIndex: number }) => void
  onAtBottomChange?: (atBottom: boolean) => void
  firstItemIndex: number
  isLoading?: boolean
  hasMore?: boolean
  highlightedMessageId?: string | null
}

export interface MessageListHandle {
  scrollToBottom: (behavior?: "auto" | "smooth") => void
  scrollToMessage: (messageId: string) => void
}

const MemoizedMessageItem = memo(MessageItem)

export const MessageList = forwardRef<MessageListHandle, MessageListProps>(function MessageList(
  {
    chatId,
    messages,
    sentMediaCache,
    onReply,
    onQuotedClick,
    onLoadMore,
    onPrefetch,
    onTrimOldMessages,
    onRangeChanged,
    onAtBottomChange,
    firstItemIndex,
    isLoading,
    hasMore,
    highlightedMessageId,
  },
  ref,
) {
  const virtuosoRef = useRef<VirtuosoHandle>(null)
  const prefetchTriggeredRef = useRef(false)

  const renderItem = useCallback(
    (_: number, msg: store.Message) => {
      const isHighlighted = highlightedMessageId === msg.Info?.ID
      return (
        <div
          className={clsx("px-4 py-1 transition-colors duration-500", {
            "bg-green-200/50 dark:bg-green-500/30": isHighlighted,
          })}
        >
          <MemoizedMessageItem
            message={msg}
            chatId={chatId}
            sentMediaCache={sentMediaCache}
            onReply={onReply}
            onQuotedClick={onQuotedClick}
          />
        </div>
      )
    },
    [chatId, onReply, onQuotedClick, sentMediaCache, highlightedMessageId],
  )

  const scrollToBottom = useCallback(
    (behavior: "auto" | "smooth" = "smooth") => {
      if (virtuosoRef.current && messages.length > 0) {
        virtuosoRef.current.scrollToIndex({
          index: messages.length - 1,
          align: "end",
          behavior,
        })
      }
    },
    [messages.length],
  )

  const scrollToMessage = useCallback(
    (messageId: string) => {
      if (!virtuosoRef.current) return

      const messageIndex = messages.findIndex(m => m.Info?.ID === messageId)
      if (messageIndex >= 0) {
        virtuosoRef.current.scrollToIndex({
          index: messageIndex,
          align: "center",
          behavior: "smooth",
        })
      }
    },
    [messages],
  )

  useImperativeHandle(ref, () => ({ scrollToBottom, scrollToMessage }))

  const handleStartReached = useCallback(() => {
    if (!isLoading && hasMore && onLoadMore) {
      onLoadMore()
      prefetchTriggeredRef.current = false
    }
  }, [isLoading, hasMore, onLoadMore])

  return (
    <Virtuoso
      ref={virtuosoRef}
      data={messages}
      firstItemIndex={firstItemIndex}
      initialTopMostItemIndex={Math.max(0, messages.length - 1)}
      startReached={handleStartReached}
      followOutput="smooth"
      alignToBottom
      rangeChanged={onRangeChanged}
      atBottomStateChange={onAtBottomChange}
      increaseViewportBy={{ top: 300, bottom: 0 }}
      className="flex-1 overflow-y-auto bg-repeat virtuoso-scroller"
      style={{ backgroundImage: "url('/assets/images/bg-chat-tile-dark.png')" }}
      itemContent={renderItem}
      components={{
        Header: () => (
          <div className="flex justify-center py-4">
            {isLoading ? (
              <div className="animate-spin h-5 w-5 border-2 border-green-500 rounded-full border-t-transparent" />
            ) : null}
          </div>
        ),
      }}
    />
  )
})
