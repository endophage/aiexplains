import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { Explanation, Section } from '../types'
import SectionComponent from '../components/Section'

export default function ExplanationPage() {
  const { id } = useParams<{ id: string }>()
  const [explanation, setExplanation] = useState<Explanation | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState('')
  const titleInputRef = useRef<HTMLInputElement>(null)
  const [regenPrompt, setRegenPrompt] = useState('')
  const [regenerating, setRegenerating] = useState(false)
  const [regenError, setRegenError] = useState<string | null>(null)
  const [allTags, setAllTags] = useState<string[]>([])
  const [addingTag, setAddingTag] = useState(false)
  const [tagDraft, setTagDraft] = useState('')
  const tagInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!id) return
    api.getExplanation(id)
      .then(setExplanation)
      .catch(() => setError('Failed to load explanation'))
      .finally(() => setLoading(false))
    api.listTags().then(setAllTags).catch(() => {})
  }, [id])

  function handleSectionUpdate(updatedSection: Section) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      return {
        ...prev,
        sections: prev.sections.map(s => s.id === updatedSection.id ? updatedSection : s),
      }
    })
  }

  function handleInsertAfter(afterSectionId: string, newSections: Section[]) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      const idx = prev.sections.findIndex(s => s.id === afterSectionId)
      if (idx === -1) return prev
      const sections = [
        ...prev.sections.slice(0, idx + 1),
        ...newSections,
        ...prev.sections.slice(idx + 1),
      ]
      return { ...prev, sections }
    })
  }

  function handleMoveUp(sectionId: string) {
    if (!explanation?.sections) return
    const active = explanation.sections.filter(s => !s.deleted)
    const idx = active.findIndex(s => s.id === sectionId)
    if (idx <= 0) return
    const reordered = [...active]
    ;[reordered[idx - 1], reordered[idx]] = [reordered[idx], reordered[idx - 1]]
    const deleted = explanation.sections.filter(s => s.deleted)
    setExplanation({ ...explanation, sections: [...reordered, ...deleted] })
    api.reorderSections(explanation.id, reordered.map(s => s.id)).catch(console.error)
  }

  function handleMoveDown(sectionId: string) {
    if (!explanation?.sections) return
    const active = explanation.sections.filter(s => !s.deleted)
    const idx = active.findIndex(s => s.id === sectionId)
    if (idx === -1 || idx >= active.length - 1) return
    const reordered = [...active]
    ;[reordered[idx], reordered[idx + 1]] = [reordered[idx + 1], reordered[idx]]
    const deleted = explanation.sections.filter(s => s.deleted)
    setExplanation({ ...explanation, sections: [...reordered, ...deleted] })
    api.reorderSections(explanation.id, reordered.map(s => s.id)).catch(console.error)
  }

  function handleDelete(sectionId: string) {
    if (!explanation) return
    api.deleteSection(explanation.id, sectionId).catch(console.error)
    setExplanation(prev => {
      if (!prev?.sections) return prev
      return { ...prev, sections: prev.sections.map(s => s.id === sectionId ? { ...s, deleted: true } : s) }
    })
  }

  function handleRestore(sectionId: string) {
    if (!explanation) return
    api.restoreSection(explanation.id, sectionId).catch(console.error)
    setExplanation(prev => {
      if (!prev?.sections) return prev
      return { ...prev, sections: prev.sections.map(s => s.id === sectionId ? { ...s, deleted: false } : s) }
    })
  }

  function startEditingTitle() {
    if (!explanation) return
    setTitleDraft(explanation.title)
    setEditingTitle(true)
    setTimeout(() => titleInputRef.current?.select(), 0)
  }

  function commitTitle() {
    if (!explanation || !titleDraft.trim()) { setEditingTitle(false); return }
    const newTitle = titleDraft.trim()
    setEditingTitle(false)
    setExplanation(prev => prev ? { ...prev, title: newTitle } : prev)
    api.updateTitle(explanation.id, newTitle).catch(err => {
      console.error(err)
      setExplanation(prev => prev ? { ...prev, title: explanation.title } : prev)
    })
  }

  async function handleAddTag(e?: FormEvent | KeyboardEvent) {
    e?.preventDefault()
    if (!explanation || !tagDraft.trim()) return
    const tag = tagDraft.trim().toLowerCase()
    setTagDraft('')
    setAddingTag(false)
    setExplanation(prev => prev ? { ...prev, tags: [...(prev.tags ?? []).filter(t => t !== tag), tag] } : prev)
    setAllTags(prev => prev.includes(tag) ? prev : [...prev, tag].sort())
    api.addTag(explanation.id, tag).catch(() => {
      setExplanation(prev => prev ? { ...prev, tags: (prev.tags ?? []).filter(t => t !== tag) } : prev)
    })
  }

  function handleRemoveTag(tag: string) {
    if (!explanation) return
    setExplanation(prev => prev ? { ...prev, tags: (prev.tags ?? []).filter(t => t !== tag) } : prev)
    api.removeTag(explanation.id, tag).catch(() => {
      setExplanation(prev => prev ? { ...prev, tags: [...(prev.tags ?? []), tag] } : prev)
    })
  }

  function startAddingTag() {
    setAddingTag(true)
    setTimeout(() => tagInputRef.current?.focus(), 0)
  }

  async function handleRegenerate(e: FormEvent) {
    e.preventDefault()
    if (!explanation || regenerating) return
    setRegenerating(true)
    setRegenError(null)
    try {
      const { sections: newSections } = await api.regenerateExplanation(explanation.id, regenPrompt.trim())
      setExplanation(prev => {
        if (!prev) return prev
        const deleted = (prev.sections ?? []).filter(s => s.deleted)
        return { ...prev, sections: [...newSections, ...deleted] }
      })
      setRegenPrompt('')
    } catch (err) {
      setRegenError(err instanceof Error ? err.message : 'Failed to generate sections')
    } finally {
      setRegenerating(false)
    }
  }

  if (loading) return (
    <>
      <header><div className="container"><h1>AI Explains</h1></div></header>
      <main className="container"><div className="loading">Loading…</div></main>
    </>
  )

  if (error || !explanation) return (
    <>
      <header><div className="container"><h1>AI Explains</h1></div></header>
      <main className="container"><div className="error">{error ?? 'Not found'}</div></main>
    </>
  )

  const allSections = explanation.sections ?? []
  const activeSections = allSections.filter(s => !s.deleted)
  const deletedSections = allSections.filter(s => s.deleted)

  return (
    <>
      <header>
        <div className="container">
          <Link to="/" className="back">← All explanations</Link>
          {editingTitle ? (
            <input
              ref={titleInputRef}
              className="title-input"
              value={titleDraft}
              onChange={e => setTitleDraft(e.target.value)}
              onBlur={commitTitle}
              onKeyDown={e => {
                if (e.key === 'Enter') commitTitle()
                if (e.key === 'Escape') setEditingTitle(false)
              }}
            />
          ) : (
            <h1 className="title-editable" onClick={startEditingTitle} title="Click to edit title">
              {explanation.title}
            </h1>
          )}
        </div>
      </header>

      <main className="container">
        <div className="explanation-tags">
          {(explanation.tags ?? []).map(tag => (
            <span key={tag} className="tag-pill">
              {tag}
              <button className="tag-pill-remove" title="Remove tag" onClick={() => handleRemoveTag(tag)}>×</button>
            </span>
          ))}
          {addingTag ? (
            <form className="tag-add-form" onSubmit={handleAddTag}>
              <input
                ref={tagInputRef}
                list="tag-suggestions"
                className="tag-add-input"
                value={tagDraft}
                onChange={e => setTagDraft(e.target.value)}
                placeholder="tag name"
                onKeyDown={e => { if (e.key === 'Escape') { setAddingTag(false); setTagDraft('') } }}
              />
              <datalist id="tag-suggestions">
                {allTags.map(t => <option key={t} value={t} />)}
              </datalist>
              <button type="submit" className="btn btn-ghost btn-sm">Add</button>
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setAddingTag(false); setTagDraft('') }}>Cancel</button>
            </form>
          ) : (
            <button className="tag-add-btn" onClick={startAddingTag}>+ Tag</button>
          )}
        </div>

        {activeSections.length === 0 && (
          <div className="regen-form">
            <p className="regen-hint">All sections have been deleted. Generate new content for this explanation.</p>
            {regenError && <div className="error">{regenError}</div>}
            <form onSubmit={handleRegenerate} className="regen-input-row">
              <textarea
                rows={2}
                placeholder="Optionally describe what you'd like covered, or leave blank to regenerate from the topic…"
                value={regenPrompt}
                onChange={e => setRegenPrompt(e.target.value)}
                disabled={regenerating}
                className="regen-textarea"
              />
              <button type="submit" className="btn btn-primary" disabled={regenerating}>
                {regenerating ? 'Generating…' : 'Generate'}
              </button>
            </form>
          </div>
        )}

        {activeSections.map((section, idx) => (
          <SectionComponent
            key={section.id}
            section={section}
            explanationId={explanation.id}
            topic={explanation.topic}
            isFirst={idx === 0}
            isLast={idx === activeSections.length - 1}
            onUpdate={handleSectionUpdate}
            onInsertAfter={(afterId, newSections) => handleInsertAfter(afterId, newSections)}
            onMoveUp={() => handleMoveUp(section.id)}
            onMoveDown={() => handleMoveDown(section.id)}
            onDelete={() => handleDelete(section.id)}
          />
        ))}

        {deletedSections.length > 0 && (
          <details className="deleted-sections">
            <summary>Deleted sections ({deletedSections.length})</summary>
            <ul className="deleted-list">
              {deletedSections.map(section => {
                const content = section.versions.find(v => v.version === section.current_version)?.content
                  ?? section.versions[0]?.content ?? ''
                const doc = new DOMParser().parseFromString(content, 'text/html')
                const title = doc.querySelector('h2')?.textContent?.trim() ?? section.id
                return (
                  <li key={section.id} className="deleted-item">
                    <span className="deleted-item-title">{title}</span>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleRestore(section.id)}
                    >
                      Restore
                    </button>
                  </li>
                )
              })}
            </ul>
          </details>
        )}
      </main>
    </>
  )
}
