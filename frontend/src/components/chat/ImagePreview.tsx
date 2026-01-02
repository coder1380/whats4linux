import { useEffect } from "react"
import { DownloadImageToFile } from "../../../wailsjs/go/api/Api"

interface ImagePreviewProps {
  src: string
  onClose: () => void
  messageId: string
}

export function ImagePreview({ src, onClose, messageId }: ImagePreviewProps) {
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose()
      }
    }

    window.addEventListener("keydown", handleEscape)
    return () => window.removeEventListener("keydown", handleEscape)
  }, [onClose])

  const handleDownload = async () => {
    try {
      await DownloadImageToFile(messageId)
    } catch (e) {}
  }

  return (
    <div
      className="fixed inset-0 z-50 bg-black/90 flex items-center justify-center"
      onClick={onClose}
    >
      <div className="absolute top-4 right-4 flex gap-2">
        <button
          onClick={e => {
            e.stopPropagation()
            handleDownload()
          }}
          className="p-3 bg-black/50 hover:bg-black/70 rounded-full text-white transition-colors"
          title="Download image"
        >
          <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z" />
          </svg>
        </button>
        <button
          onClick={e => {
            e.stopPropagation()
            onClose()
          }}
          className="p-3 bg-black/50 hover:bg-black/70 rounded-full text-white transition-colors"
          title="Close"
        >
          <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
          </svg>
        </button>
      </div>
      <img
        src={src}
        alt="Preview"
        className="max-w-[90vw] max-h-[90vh] object-contain"
        onClick={e => e.stopPropagation()}
      />
    </div>
  )
}
