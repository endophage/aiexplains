import { useMemo, useEffect, useRef, useState, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import mermaid from 'mermaid'
import { api } from '../api/client'
import type { Section } from '../types'

mermaid.initialize({ startOnLoad: false, theme: 'default' })

type ExpandedDiagram = { type: 'svg'; html: string } | { type: 'img'; src: string; alt: string }

// ── Markdown ↔ HTML helpers ──────────────────────────────────────────────────

function nodeToMd(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? ''
  if (node.nodeType !== Node.ELEMENT_NODE) return ''
  const el = node as Element
  const tag = el.tagName.toLowerCase()
  const kids = () => Array.from(el.childNodes).map(nodeToMd).join('')
  switch (tag) {
    case 'h1': return `# ${kids()}\n\n`
    case 'h2': return `## ${kids()}\n\n`
    case 'h3': return `### ${kids()}\n\n`
    case 'h4': return `#### ${kids()}\n\n`
    case 'strong': case 'b': return `**${kids()}**`
    case 'em': case 'i': return `*${kids()}*`
    case 'code': return el.closest('pre') ? el.textContent ?? '' : `\`${kids()}\``
    case 'pre': {
      const isMermaid = el.classList.contains('mermaid') || el.querySelector('.mermaid') !== null
      const lang = isMermaid ? 'mermaid' : ''
      return `\`\`\`${lang}\n${el.textContent ?? ''}\n\`\`\`\n\n`
    }
    case 'blockquote': return Array.from(el.childNodes).map(nodeToMd).join('').trim()
      .split('\n').map(l => `> ${l}`).join('\n') + '\n\n'
    case 'ul': return Array.from(el.querySelectorAll(':scope > li'))
      .map(li => `- ${nodeToMd(li).trim()}`).join('\n') + '\n\n'
    case 'ol': return Array.from(el.querySelectorAll(':scope > li'))
      .map((li, i) => `${i + 1}. ${nodeToMd(li).trim()}`).join('\n') + '\n\n'
    case 'li': return kids()
    case 'p': return kids() + '\n\n'
    case 'br': return '\n'
    default:
      if (el.classList.contains('mermaid')) {
        return `\`\`\`mermaid\n${el.textContent ?? ''}\n\`\`\`\n\n`
      }
      return kids()
  }
}

function htmlToMarkdown(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return Array.from(doc.body.childNodes).map(nodeToMd).join('').trim()
}

function inlineMd(text: string): string {
  return text
    .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`(.+?)`/g, '<code>$1</code>')
}

function markdownToHTML(md: string): string {
  const lines = md.split('\n')
  const out: string[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.startsWith('#### ')) { out.push(`<h4>${inlineMd(line.slice(5))}</h4>`); i++; continue }
    if (line.startsWith('### '))  { out.push(`<h3>${inlineMd(line.slice(4))}</h3>`); i++; continue }
    if (line.startsWith('## '))   { out.push(`<h2>${inlineMd(line.slice(3))}</h2>`); i++; continue }
    if (line.startsWith('# '))    { out.push(`<h1>${inlineMd(line.slice(2))}</h1>`); i++; continue }
    if (line.startsWith('> ')) {
      const items: string[] = []
      while (i < lines.length && lines[i].startsWith('> ')) { items.push(lines[i].slice(2)); i++ }
      out.push(`<blockquote><p>${inlineMd(items.join('\n'))}</p></blockquote>`)
      continue
    }
    if (line.startsWith('- ')) {
      const items: string[] = []
      while (i < lines.length && lines[i].startsWith('- ')) { items.push(`<li>${inlineMd(lines[i].slice(2))}</li>`); i++ }
      out.push(`<ul>${items.join('')}</ul>`)
      continue
    }
    if (/^\d+\. /.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\d+\. /.test(lines[i])) { items.push(`<li>${inlineMd(lines[i].replace(/^\d+\. /, ''))}</li>`); i++ }
      out.push(`<ol>${items.join('')}</ol>`)
      continue
    }
    if (line.startsWith('```')) {
      const lang = line.slice(3).trim()
      const codeLines: string[] = []
      i++
      while (i < lines.length && !lines[i].startsWith('```')) { codeLines.push(lines[i]); i++ }
      i++ // consume closing ```
      if (lang === 'mermaid') {
        out.push(`<pre class="mermaid">${codeLines.join('\n')}</pre>`)
      } else {
        const cls = lang ? ` class="language-${lang}"` : ''
        out.push(`<pre><code${cls}>${codeLines.join('\n')}</code></pre>`)
      }
      continue
    }
    if (line.trim() === '') { i++; continue }
    out.push(`<p>${inlineMd(line)}</p>`)
    i++
  }
  return out.join('\n')
}

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

  const [isEditing, setIsEditing] = useState(false)
  const [editTitle, setEditTitle] = useState('')
  const [editBody, setEditBody] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

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

  function startEditing() {
    setEditTitle(title)
    setEditBody(htmlToMarkdown(bodyHTML))
    setSaveError(null)
    setIsEditing(true)
  }

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (saving) return
    setSaving(true)
    setSaveError(null)
    try {
      const html = `<h2>${editTitle.trim()}</h2>${markdownToHTML(editBody)}`
      const { section: updated } = await api.editSection(explanationId, section.id, html)
      onUpdate(updated)
      setDisplayVersion(updated.current_version)
      setIsEditing(false)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

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

  const busy = asking || extending || branching || saving

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
          className={`section-btn${isEditing ? ' active' : ''}`}
          title={isEditing ? 'Cancel edit' : 'Edit section content'}
          onClick={() => isEditing ? setIsEditing(false) : startEditing()}
          disabled={busy && !isEditing}
        >
          ✎
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
          {isEditing
            ? <input
                className="edit-title-input"
                value={editTitle}
                onChange={e => setEditTitle(e.target.value)}
                placeholder="Section title"
                disabled={saving}
              />
            : <h2 className="section-title">{title}</h2>
          }
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

        {isEditing ? (
          <form className="edit-form" onSubmit={handleSave}>
            {saveError && <div className="error">{saveError}</div>}
            <textarea
              className="edit-body-textarea"
              value={editBody}
              onChange={e => setEditBody(e.target.value)}
              disabled={saving}
              autoFocus
              spellCheck
            />
            <div className="form-actions">
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => setIsEditing(false)}>Cancel</button>
              <button type="submit" className="btn btn-primary btn-sm" disabled={saving}>
                {saving ? 'Saving…' : 'Save'}
              </button>
            </div>
          </form>
        ) : (
          <div ref={bodyRef} className="section-body" dangerouslySetInnerHTML={{ __html: bodyHTML }} />
        )}

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
