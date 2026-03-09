import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { Explanation, Section } from '../types'
import SectionComponent from '../components/Section'

export default function ExplanationPage() {
  const { id } = useParams<{ id: string }>()
  const [explanation, setExplanation] = useState<Explanation | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

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

  function handleInsertAfter(afterSectionId: string, newSection: Section) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      const idx = prev.sections.findIndex(s => s.id === afterSectionId)
      if (idx === -1) return prev
      const sections = [
        ...prev.sections.slice(0, idx + 1),
        newSection,
        ...prev.sections.slice(idx + 1),
      ]
      return { ...prev, sections }
    })
  }

  function handleMoveUp(sectionId: string) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      const active = prev.sections.filter(s => !s.deleted)
      const idx = active.findIndex(s => s.id === sectionId)
      if (idx <= 0) return prev
      const reordered = [...active]
      ;[reordered[idx - 1], reordered[idx]] = [reordered[idx], reordered[idx - 1]]
      const deleted = prev.sections.filter(s => s.deleted)
      const sections = [...reordered, ...deleted]
      api.reorderSections(prev.id, reordered.map(s => s.id)).catch(console.error)
      return { ...prev, sections }
    })
  }

  function handleMoveDown(sectionId: string) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      const active = prev.sections.filter(s => !s.deleted)
      const idx = active.findIndex(s => s.id === sectionId)
      if (idx === -1 || idx >= active.length - 1) return prev
      const reordered = [...active]
      ;[reordered[idx], reordered[idx + 1]] = [reordered[idx + 1], reordered[idx]]
      const deleted = prev.sections.filter(s => s.deleted)
      const sections = [...reordered, ...deleted]
      api.reorderSections(prev.id, reordered.map(s => s.id)).catch(console.error)
      return { ...prev, sections }
    })
  }

  function handleDelete(sectionId: string) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      const sections = prev.sections.map(s => s.id === sectionId ? { ...s, deleted: true } : s)
      api.deleteSection(prev.id, sectionId).catch(console.error)
      return { ...prev, sections }
    })
  }

  function handleRestore(sectionId: string) {
    setExplanation(prev => {
      if (!prev?.sections) return prev
      const sections = prev.sections.map(s => s.id === sectionId ? { ...s, deleted: false } : s)
      api.restoreSection(prev.id, sectionId).catch(console.error)
      return { ...prev, sections }
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
          <h1>{explanation.title}</h1>
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
            onInsertAfter={handleInsertAfter}
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
