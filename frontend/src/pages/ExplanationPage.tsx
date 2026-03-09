import { useEffect, useRef, useState } from 'react'
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

  useEffect(() => {
    if (!id) return
    api.getExplanation(id)
      .then(setExplanation)
      .catch(() => setError('Failed to load explanation'))
      .finally(() => setLoading(false))
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
