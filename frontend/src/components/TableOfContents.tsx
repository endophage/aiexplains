import { useEffect, useRef, useState } from 'react'

interface TocEntry {
  id: string
  title: string
}

interface Props {
  entries: TocEntry[]
}

export default function TableOfContents({ entries }: Props) {
  const [activeId, setActiveId] = useState<string | null>(null)
  const observerRef = useRef<IntersectionObserver | null>(null)

  useEffect(() => {
    if (entries.length === 0) return

    // Track which sections are currently intersecting and pick the topmost one
    const visible = new Map<string, number>()

    observerRef.current?.disconnect()
    observerRef.current = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const id = entry.target.id.replace(/^sec-/, '')
          if (entry.isIntersecting) {
            visible.set(id, entry.boundingClientRect.top)
          } else {
            visible.delete(id)
          }
        }
        if (visible.size === 0) return
        // Pick the entry with the smallest (topmost) y position
        let topId = ''
        let topY = Infinity
        for (const [id, y] of visible) {
          if (y < topY) { topY = y; topId = id }
        }
        if (topId) setActiveId(topId)
      },
      { rootMargin: '0px 0px -60% 0px', threshold: 0 }
    )

    for (const entry of entries) {
      const el = document.getElementById(`sec-${entry.id}`)
      if (el) observerRef.current.observe(el)
    }

    return () => observerRef.current?.disconnect()
  }, [entries])

  if (entries.length === 0) return null

  return (
    <nav className="toc">
      <ul className="toc-list">
        {entries.map(entry => (
          <li key={entry.id}>
            <a
              href={`#sec-${entry.id}`}
              className={`toc-item${activeId === entry.id ? ' toc-item--active' : ''}`}
              onClick={e => {
                e.preventDefault()
                document.getElementById(`sec-${entry.id}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
              }}
            >
              {entry.title}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  )
}
