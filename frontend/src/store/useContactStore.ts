import { create } from "zustand"
import { immer } from "zustand/middleware/immer"
import { GetProfile } from "../../wailsjs/go/api/Api"
import type { api } from "../../wailsjs/go/models"

interface ContactStore {
  contacts: Record<string, api.Contact>
  getContact: (jid: string) => Promise<api.Contact | null>
}

export const useContactStore = create<ContactStore>()(
  immer((set, get) => ({
    contacts: {},
    getContact: async (jid: string) => {
      // cache hit
      if (get().contacts[jid]) return get().contacts[jid]

      // cache miss
      try {
        const contact = await GetProfile(jid)
        set(state => {
          state.contacts[jid] = contact
        })
        return contact
      } catch (err) {
        return null
      }
    },
  })),
)
