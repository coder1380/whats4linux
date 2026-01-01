import { create } from "zustand"
import { immer } from "zustand/middleware/immer"
import { useContactStore } from "./useContactStore"

interface MessageStore {
  messages: Record<string, any[]>
  activeChatId: string | null
  setActiveChatId: (chatId: string) => void
  setMessages: (chatId: string, messages: any[]) => void
  addMessage: (chatId: string, message: any) => void
  prependMessages: (chatId: string, messages: any[]) => void
  updateMessage: (chatId: string, message: any) => void
  clearMessages: (chatId: string) => void
  trimOldMessages: (chatId: string, keepCount: number) => void
}

export const useMessageStore = create<MessageStore>()(
  immer((set, get) => ({
    messages: {},
    activeChatId: null,

    setActiveChatId: chatId => {
      const prevChatId = get().activeChatId

      set(state => {
        if (prevChatId && prevChatId !== chatId) {
          // keep only last 10 messages of previous chat (for quick switching)
          if (state.messages[prevChatId]) {
            state.messages[prevChatId] = state.messages[prevChatId].slice(-10)
          }

          // dispose contact name cache for old chat context
          useContactStore.getState().disposeCache()
        }
        state.activeChatId = chatId
      })
    },

    setMessages: (chatId, messages) =>
      set(state => {
        state.messages[chatId] = messages
      }),

    addMessage: (chatId, message) =>
      set(state => {
        if (!state.messages[chatId]) state.messages[chatId] = []
        state.messages[chatId].push(message)
      }),

    prependMessages: (chatId, messages) =>
      set(state => {
        const existing = state.messages[chatId] || []
        state.messages[chatId] = [...messages, ...existing]
      }),

    updateMessage: (chatId, message) =>
      set(state => {
        if (!state.messages[chatId]) state.messages[chatId] = []

        const idx = state.messages[chatId].findIndex(m => m.Info?.ID === message.Info?.ID)

        if (idx >= 0) {
          state.messages[chatId][idx] = message
        } else {
          state.messages[chatId].push(message)
        }
      }),

    trimOldMessages: (chatId, keepCount) =>
      set(state => {
        if (state.messages[chatId] && state.messages[chatId].length > keepCount) {
          state.messages[chatId] = state.messages[chatId].slice(-keepCount)
        }
      }),

    clearMessages: chatId =>
      set(state => {
        delete state.messages[chatId]
      }),
  })),
)
