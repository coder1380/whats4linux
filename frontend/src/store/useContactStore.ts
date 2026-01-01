import { create } from "zustand"
import { immer } from "zustand/middleware/immer"
import { GetProfile } from "../../wailsjs/go/api/Api"

interface ContactStore {
  contacts: Record<string, { name: string; timestamp: number }>
  getContactName: (jidString: string) => Promise<string>
  disposeCache: () => void
}

export const useContactStore = create<ContactStore>()(
  immer((set, get) => ({
    contacts: {},

    getContactName: async jidString => {
      const userId = jidString.split("@")[0]

      const cached = get().contacts[userId]
      if (cached) return cached.name

      try {
        const contact = await GetProfile(jidString)
        const displayName = contact.full_name || contact.push_name || userId

        set(state => {
          state.contacts[userId] = {
            name: displayName,
            timestamp: Date.now(),
          }
        })
        return displayName
      } catch {
        return userId
      }
    },

    disposeCache: () =>
      set(state => {
        state.contacts = {}
      }),
  })),
)
