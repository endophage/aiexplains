import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { Explanation } from '../types'

export default function Home() {
  const navigate = useNavigate()
  const [explanations, setExplanations] = useState<Explanation[]>([])
  const [allTags, setAllTags] = useState<string[]>([])
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const [tagSearch, setTagSearch] = useState('')
  const [topic, setTopic] = useState('')
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.listTags().then(setAllTags).catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    api.listExplanations(selectedTags.length ? selectedTags : undefined)
      .then(setExplanations)
      .catch(() => setError('Failed to load explanations'))
      .finally(() => setLoading(false))
  }, [selectedTags])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!topic.trim() || generating) return
    setGenerating(true)
    setError(null)
    try {
      const explanation = await api.createExplanation(topic.trim())
      navigate(`/explanations/${explanation.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate explanation')
      setGenerating(false)
    }
  }

  function handleDelete(e: React.MouseEvent, id: string) {
    e.stopPropagation()
    api.deleteExplanation(id).catch(console.error)
    setExplanations(prev => prev.filter(ex => ex.id !== id))
  }

  function toggleTag(tag: string) {
    setSelectedTags(prev =>
      prev.includes(tag) ? prev.filter(t => t !== tag) : [...prev, tag]
    )
  }

  function handleDeleteTag(e: React.MouseEvent, tag: string) {
    e.stopPropagation()
    setAllTags(prev => prev.filter(t => t !== tag))
    setSelectedTags(prev => prev.filter(t => t !== tag))
    setExplanations(prev => prev.map(ex => ({ ...ex, tags: ex.tags.filter(t => t !== tag) })))
    api.deleteTag(tag).catch(console.error)
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric', month: 'short', day: 'numeric',
    })
  }

  const filteredTagList = allTags.filter(t =>
    t.toLowerCase().includes(tagSearch.toLowerCase())
  )

  return (
    <>
      <header>
        <div className="container">
          <h1>AI Explains</h1>
        </div>
      </header>

      <main className="container">
        <div className="new-explanation-form">
          <h2>What would you like explained?</h2>
          {error && <div className="error">{error}</div>}
          <form onSubmit={handleSubmit} className="input-row">
            <input
              type="text"
              placeholder="e.g. How does quantum computing work?"
              value={topic}
              onChange={e => setTopic(e.target.value)}
              disabled={generating}
              autoFocus
            />
            <button type="submit" className="btn btn-primary" disabled={generating || !topic.trim()}>
              {generating ? 'Generating…' : 'Explain'}
            </button>
          </form>
          {generating && (
            <p style={{ margin: '0.75rem 0 0', fontSize: '0.85rem', color: '#6b7280' }}>
              Generating your explanation, this may take a moment…
            </p>
          )}
        </div>

        <div className="home-layout">
          <div className="explanation-list-col">
            {loading ? (
              <div className="loading">Loading…</div>
            ) : explanations.length === 0 ? (
              <div className="empty">
                {selectedTags.length > 0
                  ? 'No explanations match the selected tags.'
                  : 'No explanations yet. Ask something above to get started.'}
              </div>
            ) : (
              <div className="explanation-list">
                {explanations.map(e => (
                  <div
                    key={e.id}
                    className="explanation-card"
                    onClick={() => navigate(`/explanations/${e.id}`)}
                  >
                    <div className="card-body">
                      <h3>{e.title}</h3>
                      <div className="meta">Updated {formatDate(e.updated_at)}</div>
                      {e.tags?.length > 0 && (
                        <div className="card-tags">
                          {e.tags.map(tag => (
                            <span key={tag} className="tag-pill">{tag}</span>
                          ))}
                        </div>
                      )}
                    </div>
                    <button
                      className="card-delete-btn"
                      title="Delete explanation"
                      onClick={ev => handleDelete(ev, e.id)}
                    >
                      🗑
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {allTags.length > 0 && (
            <aside className="tag-filter-panel">
              <h3>Filter by tag</h3>
              <input
                type="text"
                className="tag-filter-search"
                placeholder="Search tags…"
                value={tagSearch}
                onChange={e => setTagSearch(e.target.value)}
              />
              <div className="tag-filter-list">
                {filteredTagList.length === 0 ? (
                  <span className="tag-filter-empty">No tags found</span>
                ) : (
                  filteredTagList.map(tag => (
                    <button
                      key={tag}
                      className={`tag-filter-item${selectedTags.includes(tag) ? ' selected' : ''}`}
                      onClick={() => toggleTag(tag)}
                    >
                      <span style={{ fontSize: '0.7rem', flexShrink: 0 }}>{selectedTags.includes(tag) ? '✓' : '○'}</span>
                      <span style={{ flex: 1 }}>{tag}</span>
                      <span
                        role="button"
                        title="Delete tag"
                        style={{ opacity: 0.4, fontSize: '0.8rem', lineHeight: 1 }}
                        onClick={e => handleDeleteTag(e, tag)}
                        onMouseEnter={e => (e.currentTarget.style.opacity = '1')}
                        onMouseLeave={e => (e.currentTarget.style.opacity = '0.4')}
                      >×</span>
                    </button>
                  ))
                )}
              </div>
              {selectedTags.length > 0 && (
                <button className="tag-filter-clear" onClick={() => setSelectedTags([])}>
                  Clear filters ({selectedTags.length})
                </button>
              )}
            </aside>
          )}
        </div>
      </main>
    </>
  )
}
