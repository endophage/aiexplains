import { useMemo, useEffect, useRef, useState, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import mermaid from 'mermaid'
import { api } from '../api/client'
import type { Section } from '../types'

type ExpandedDiagram = { type: 'svg'; html: string } | { type: 'img'; src: string; alt: string }

interface Props {
  section: Section
  explanationId: string
  topic: string
  isFirst: boolean
  isLast: boolean
  hasChildren: boolean
  onUpdate: (section: Section) => void
  onInsertAfter: (afterSectionId: string, newSections: Section[]) => void
  onMoveUp: () => void
  onMoveDown: () => void
  onDelete: () => void
  onDrillIn: () => void
  onBranchCreated: (newSections: Section[]) => void
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
  isFirst, isLast, hasChildren,
  onUpdate, onInsertAfter, onMoveUp, onMoveDown, onDelete, onDrillIn, onBranchCreated,
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

  const [showBranch, setShowBranch] = useState(false)
  const [branchPrompt, setBranchPrompt] = useState('')
  const [branching, setBranching] = useState(false)
  const [branchError, setBranchError] = useState<string | null>(null)

  const [extractPos, setExtractPos] = useState<{ top: number; left: number } | null>(null)
  const [extracting, setExtracting] = useState(false)
  const [extractingChild, setExtractingChild] = useState(false)

  const latestVersion = section.current_version
  const sortedVersionNums = [...section.versions.map(v => v.version)].sort((a, b) => a - b)
  const currentIdx = sortedVersionNums.indexOf(displayVersion)

  const currentContent = section.versions.find(v => v.version === displayVersion)?.content
    ?? section.versions[0]?.content
    ?? ''

  const { title, bodyHTML } = useMemo(() => parseContent(currentContent), [currentContent])

  const [expandedDiagram, setExpandedDiagram] = useState<ExpandedDiagram | null>(null)

  const bodyRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (bodyRef.current) {
      mermaid.run({ nodes: Array.from(bodyRef.current.querySelectorAll('.mermaid')) })
        .catch(() => {})
    }
  }, [bodyHTML])

  useEffect(() => {
    function handleSelectionChange() {
      const sel = window.getSelection()
      if (!sel || sel.rangeCount === 0 || sel.isCollapsed) { setExtractPos(null); return }
      const range = sel.getRangeAt(0)
      if (!bodyRef.current?.contains(range.commonAncestorContainer)) { setExtractPos(null); return }
      const rect = range.getBoundingClientRect()
      if (rect.width === 0 && rect.height === 0) { setExtractPos(null); return }
      setExtractPos({ top: rect.top + window.scrollY - 44, left: rect.left + rect.width / 2 })
    }
    document.addEventListener('selectionchange', handleSelectionChange)
    return () => document.removeEventListener('selectionchange', handleSelectionChange)
  }, [])

  function captureSelection() {
    const sel = window.getSelection()
    if (!sel || sel.rangeCount === 0 || !bodyRef.current) return null
    const range = sel.getRangeAt(0)
    if (!bodyRef.current.contains(range.commonAncestorContainer)) return null
    const fragment = range.cloneContents()
    const tmp = document.createElement('div')
    tmp.appendChild(fragment)
    if (!tmp.innerHTML.trim()) return null

    // If the selection starts with a heading, use it as the title and strip it from the body
    let newTitle: string
    const firstEl = tmp.firstElementChild
    if (firstEl && /^H[1-6]$/.test(firstEl.tagName)) {
      newTitle = firstEl.textContent?.trim() || 'Extracted Section'
      firstEl.remove()
    } else {
      const words = (tmp.textContent ?? '').trim().split(/\s+/).filter(Boolean)
      newTitle = words.slice(0, 8).join(' ') + (words.length > 8 ? '…' : '') || 'Extracted Section'
    }

    const extractedHtml = tmp.innerHTML
    const originalHTML = bodyRef.current.innerHTML
    range.deleteContents()
    sel.removeAllRanges()
    setExtractPos(null)
    const remainingHtml = bodyRef.current.innerHTML
    return { extractedHtml, remainingHtml, newTitle, originalHTML }
  }

  async function handleExtract() {
    const captured = captureSelection()
    if (!captured) return
    const { extractedHtml, remainingHtml, newTitle, originalHTML } = captured
    setExtracting(true)
    try {
      const { section: updated, new_section } = await api.extractSection(
        explanationId, section.id, extractedHtml, remainingHtml, title, newTitle
      )
      onUpdate(updated)
      setDisplayVersion(updated.current_version)
      onInsertAfter(section.id, [new_section])
    } catch (err) {
      bodyRef.current!.innerHTML = originalHTML
      console.error('Extract failed:', err)
    } finally {
      setExtracting(false)
    }
  }

  async function handleExtractChild() {
    const captured = captureSelection()
    if (!captured) return
    const { extractedHtml, remainingHtml, newTitle, originalHTML } = captured
    setExtractingChild(true)
    try {
      const { section: updated, new_section } = await api.extractChildSection(
        explanationId, section.id, extractedHtml, remainingHtml, title, newTitle
      )
      onUpdate(updated)
      setDisplayVersion(updated.current_version)
      onBranchCreated([new_section])
    } catch (err) {
      bodyRef.current!.innerHTML = originalHTML
      console.error('Extract child failed:', err)
    } finally {
      setExtractingChild(false)
    }
  }

  useEffect(() => {
    const el = bodyRef.current
    if (!el) return
    function handleClick(e: globalThis.MouseEvent) {
      const target = e.target as Element
      const svg = target.closest('svg')
      const img = target.closest('img')
      if (svg) {
        const clone = svg.cloneNode(true) as SVGElement
        clone.removeAttribute('width')
        clone.removeAttribute('height')
        clone.style.removeProperty('width')
        clone.style.removeProperty('height')
        clone.style.removeProperty('max-width')
        setExpandedDiagram({ type: 'svg', html: clone.outerHTML })
      } else if (img) {
        setExpandedDiagram({ type: 'img', src: (img as HTMLImageElement).src, alt: (img as HTMLImageElement).alt })
      }
    }
    el.addEventListener('click', handleClick)
    return () => el.removeEventListener('click', handleClick)
  }, [])

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

  async function handleBranch(e: FormEvent) {
    e.preventDefault()
    if (!branchPrompt.trim() || branching) return
    setBranching(true)
    setBranchError(null)
    try {
      const { sections: newSections } = await api.branchSection(explanationId, section.id, branchPrompt.trim())
      onBranchCreated(newSections)
      setBranchPrompt('')
      setShowBranch(false)
    } catch (err) {
      setBranchError(err instanceof Error ? err.message : 'Failed to generate child sections')
    } finally {
      setBranching(false)
    }
  }

  const busy = asking || extending || branching

  return (
    <>
    {extractPos && createPortal(
      <div
        className="extract-toolbar"
        style={{ top: extractPos.top, left: extractPos.left }}
        onMouseDown={e => e.preventDefault()}
      >
        <button
          className="extract-btn"
          onClick={handleExtract}
          disabled={extracting || extractingChild}
          title="Extract selection as a new sibling section"
        >
          {extracting ? '…' : '⊕ Extract as section'}
        </button>
        <button
          className="extract-btn"
          onClick={handleExtractChild}
          disabled={extracting || extractingChild}
          title="Extract selection as a new child section"
        >
          {extractingChild ? '…' : '⊕ Make child section'}
        </button>
      </div>,
      document.body
    )}
    {expandedDiagram && createPortal(
      <div className="diagram-lightbox" onClick={() => setExpandedDiagram(null)}>
        {expandedDiagram.type === 'svg'
          ? <div className="diagram-lightbox-inner" dangerouslySetInnerHTML={{ __html: expandedDiagram.html }} />
          : <img className="diagram-lightbox-inner" src={expandedDiagram.src} alt={expandedDiagram.alt} />
        }
      </div>,
      document.body
    )}
    <div className="section" id={`sec-${section.id}`}>
      {/* Left controls column */}
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
          title={showExtend ? 'Cancel' : 'Add a sibling section after this one'}
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
              placeholder="What should the new sibling section cover?"
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

        {showBranch && (
          <form className="inline-form inline-form--branch" onSubmit={handleBranch}>
            {branchError && <div className="error">{branchError}</div>}
            <textarea
              rows={2}
              placeholder="What should this branch explore in more depth?"
              value={branchPrompt}
              onChange={e => setBranchPrompt(e.target.value)}
              disabled={branching}
              autoFocus
            />
            <div className="form-actions">
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setShowBranch(false); setBranchError(null) }}>Cancel</button>
              <button type="submit" className="btn btn-primary btn-sm" disabled={branching || !branchPrompt.trim()}>
                {branching ? 'Generating…' : 'Add child sections'}
              </button>
            </div>
          </form>
        )}
      </div>

      {/* Right controls column */}
      <div className="section-right-controls">
        <button
          className={`section-btn${showBranch ? ' active' : ''}`}
          title={showBranch ? 'Cancel' : 'Add child sections'}
          onClick={() => { setShowBranch(v => !v); setBranchError(null) }}
          disabled={busy}
        >
          +
        </button>
        {hasChildren && (
          <button
            className="section-btn section-btn--drill"
            title="View child sections"
            onClick={onDrillIn}
            disabled={busy}
          >
            ›
          </button>
        )}
      </div>
    </div>
    </>
  )
}
