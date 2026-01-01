import { useEffect, useState } from "react"
import { parseWhatsAppMarkdown } from "../../utils/markdown"
import { useContactStore } from "../../store/useContactStore"

export function QuotedMessage({ contextInfo }: { contextInfo: any }) {
  const [name, setName] = useState<string>("")
  const getContactName = useContactStore(state => state.getContactName)
  const quoted = contextInfo.quotedMessage || contextInfo.QuotedMessage

  useEffect(() => {
    const participant = contextInfo.participant || contextInfo.Participant
    if (participant) {
      getContactName(participant).then((contactName: string) => {
        if (contactName) setName(contactName)
      })
    }
  }, [contextInfo, getContactName])

  if (!quoted) return null

  const getText = () => {
    if (quoted.extendedTextMessage?.text)
      return parseWhatsAppMarkdown(quoted.extendedTextMessage.text)
    if (quoted.conversation) return parseWhatsAppMarkdown(quoted.conversation)
    if (quoted.imageMessage) return quoted.imageMessage.caption || "📷 Photo"
    if (quoted.videoMessage) return quoted.videoMessage.caption || "🎥 Video"
    if (quoted.documentMessage) return quoted.documentMessage.fileName || "📄 Document"
    if (quoted.audioMessage) return "🎵 Audio"
    if (quoted.stickerMessage) return "Sticker"
    return "Message"
  }

  return (
    <div className="bg-black/5 dark:bg-white/10 rounded-md p-2 mb-2 border-l-4 border-green-500 text-xs">
      <div className="font-bold text-green-600 mb-1">{name}</div>
      <div className="line-clamp-2 opacity-70">{getText()}</div>
    </div>
  )
}
