import { useMemo, useEffect, useRef, useState, type FormEvent } from 'react'
import mermaid from 'mermaid'
import { api } from '../api/client'
import type { Section } from '../types'

interface Props {
  section: Section
  explanationId: string
  topic: string
  isFirst: boolean
  isLast: boolean
  onUpdate: (section: Section) => void
  onInsertAfter: (afterSectionId: string, newSections: Section[]) => void
  onMoveUp: () => void
  onMoveDown: () => void
  onDelete: () => void
}

function parseContent(html: string): { title: string; bodyHTML: string } {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const h2 = doc.querySelector('h2')
  const title = h2?.textContent?.trim() ?? ''
  h2?.remove()
  return { title, bodyHTML: doc.body.innerHTML }
}

export default function SectionComponent({
  section, explanationId, topic: _topic,
  isFirst, isLast,
  onUpdate, onInsertAfter, onMoveUp, onMoveDown, onDelete,
}: Props) {
  const [displayVersion, setDisplayVersion] = useState(section.current_version)
  const [showAsk, setShowAsk] = useState(false)
  const [askPrompt, setAskPrompt] = useState('')
  const [asking, setAsking] = useState(false)
  const [askError, setAskError] = useState<string | null>(null)

  const [showExtend, setShowExtend] = useState(false)
  const [extendPrompt, setExtendPrompt] = useState('')
  const [extending, setExtending] = useState(false)
  const [extendError, setExtendError] = useState<string | null>(null)

  const latestVersion = section.current_version
  const sortedVersionNums = [...section.versions.map(v => v.version)].sort((a, b) => a - b)
  const currentIdx = sortedVersionNums.indexOf(displayVersion)

  const currentContent = section.versions.find(v => v.version === displayVersion)?.content
    ?? section.versions[0]?.content
    ?? ''

  const { title, bodyHTML } = useMemo(() => parseContent(currentContent), [currentContent])

  const bodyRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (bodyRef.current) {
      mermaid.run({ nodes: Array.from(bodyRef.current.querySelectorAll('.mermaid')) })
        .catch(() => {})
    }
  }, [bodyHTML])

  async function handleAsk(e: FormEvent) {
    e.preventDefault()
    if (!askPrompt.trim() || asking) return
    setAsking(true)
    setAskError(null)
    try {
      const { section: updated, new_sections } = await api.explainSection(explanationId, section.id, askPrompt.trim())
      onUpdate(updated)
      setDisplayVersion(updated.current_version)
      if (new_sections && new_sections.length > 0) {
        onInsertAfter(section.id, new_sections)
      }
      setAskPrompt('')
      setShowAsk(false)
    } catch (err) {
      setAskError(err instanceof Error ? err.message : 'Failed to get explanation')
    } finally {
      setAsking(false)
    }
  }

  async function handleExtend(e: FormEvent) {
    e.preventDefault()
    if (!extendPrompt.trim() || extending) return
    setExtending(true)
    setExtendError(null)
    try {
      const { sections: newSections } = await api.extendSection(explanationId, section.id, extendPrompt.trim())
      onInsertAfter(section.id, newSections)
      setExtendPrompt('')
      setShowExtend(false)
    } catch (err) {
      setExtendError(err instanceof Error ? err.message : 'Failed to generate section')
    } finally {
      setExtending(false)
    }
  }

  const busy = asking || extending

  return (
    <div className="section">
      {/* Left controls column — order: ↑ ? + 🗑 ↓ */}
      <div className="section-controls">
        <button
          className="section-btn"
          title="Move section up"
          onClick={onMoveUp}
          disabled={isFirst || busy}
        >
          ↑
        </button>
        <button
          className={`section-btn${showAsk ? ' active' : ''}`}
          title={showAsk ? 'Cancel question' : 'Ask a question about this section'}
          onClick={() => { setShowAsk(v => !v); setAskError(null) }}
          disabled={busy}
        >
          ?
        </button>
        <button
          className={`section-btn${showExtend ? ' active' : ''}`}
          title={showExtend ? 'Cancel' : 'Add a new section after this one'}
          onClick={() => { setShowExtend(v => !v); setExtendError(null) }}
          disabled={busy}
        >
          +
        </button>
        <button
          className="section-btn section-btn--delete"
          title="Delete this section"
          onClick={onDelete}
          disabled={busy}
        >
          🗑
        </button>
        <button
          className="section-btn"
          title="Move section down"
          onClick={onMoveDown}
          disabled={isLast || busy}
        >
          ↓
        </button>
      </div>

      {/* Main content column */}
      <div className="section-main">
        <div className="section-header">
          <h2 className="section-title">{title}</h2>
          {sortedVersionNums.length > 1 && (
            <div className="version-nav">
              <button onClick={() => setDisplayVersion(sortedVersionNums[currentIdx - 1])} disabled={currentIdx === 0}>←</button>
              <span>v{displayVersion}/{latestVersion}</span>
              <button onClick={() => setDisplayVersion(sortedVersionNums[currentIdx + 1])} disabled={currentIdx === sortedVersionNums.length - 1}>→</button>
              {displayVersion !== latestVersion && (
                <button className="latest-btn" onClick={() => setDisplayVersion(latestVersion)}>latest</button>
              )}
            </div>
          )}
        </div>

        {showAsk && (
          <form className="inline-form" onSubmit={handleAsk}>
            {askError && <div className="error">{askError}</div>}
            <textarea
              rows={2}
              placeholder="What would you like to know more about?"
              value={askPrompt}
              onChange={e => setAskPrompt(e.target.value)}
              disabled={asking}
              autoFocus
            />
            <div className="form-actions">
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setShowAsk(false); setAskError(null) }}>Cancel</button>
              <button type="submit" className="btn btn-primary btn-sm" disabled={asking || !askPrompt.trim()}>
                {asking ? 'Thinking…' : 'Submit'}
              </button>
            </div>
          </form>
        )}

        <div ref={bodyRef} className="section-body" dangerouslySetInnerHTML={{ __html: bodyHTML }} />

        {showExtend && (
          <form className="inline-form inline-form--extend" onSubmit={handleExtend}>
            {extendError && <div className="error">{extendError}</div>}
            <textarea
              rows={2}
              placeholder="What should the new section cover?"
              value={extendPrompt}
              onChange={e => setExtendPrompt(e.target.value)}
              disabled={extending}
              autoFocus
            />
            <div className="form-actions">
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setShowExtend(false); setExtendError(null) }}>Cancel</button>
              <button type="submit" className="btn btn-primary btn-sm" disabled={extending || !extendPrompt.trim()}>
                {extending ? 'Generating…' : 'Add section'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
