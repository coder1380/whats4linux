import React from "react"

const patterns = [
  { regex: /\*([^*]+)\*/g, style: { fontWeight: "bold" } },
  { regex: /_([^_]+)_/g, style: { fontStyle: "italic" } },
  { regex: /~([^~]+)~/g, style: { textDecoration: "line-through" } },
  {
    regex: /`([^`]+)`/g,
    style: {
      fontFamily: "monospace",
      backgroundColor: "rgba(0,0,0,0.1)",
      padding: "2px 4px",
      borderRadius: "3px",
    },
  },
]

export function parseWhatsAppMarkdown(text: string): React.ReactNode[] {
  if (!text) return [text]

  const parts: React.ReactNode[] = []
  let lastIndex = 0

  const matches: Array<{ start: number; end: number; style: any; content: string }> = []

  patterns.forEach(({ regex, style }) => {
    let match
    while ((match = regex.exec(text)) !== null) {
      matches.push({
        start: match.index,
        end: match.index + match[0].length,
        style,
        content: match[1],
      })
    }
  })

  matches.sort((a, b) => a.start - b.start)

  matches.forEach((match, index) => {
    if (match.start > lastIndex) {
      parts.push(text.slice(lastIndex, match.start))
    }

    parts.push(React.createElement("span", { key: index, style: match.style }, match.content))

    lastIndex = match.end
  })

  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex))
  }

  return parts.length > 0 ? parts : [text]
}
