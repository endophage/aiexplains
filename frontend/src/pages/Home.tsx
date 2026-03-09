import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { Explanation } from '../types'

export default function Home() {
  const navigate = useNavigate()
  const [explanations, setExplanations] = useState<Explanation[]>([])
  const [topic, setTopic] = useState('')
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.listExplanations()
      .then(setExplanations)
      .catch(() => setError('Failed to load explanations'))
      .finally(() => setLoading(false))
  }, [])

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

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric', month: 'short', day: 'numeric',
    })
  }

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

        {loading ? (
          <div className="loading">Loading…</div>
        ) : explanations.length === 0 ? (
          <div className="empty">No explanations yet. Ask something above to get started.</div>
        ) : (
          <div className="explanation-list">
            {explanations.map(e => (
              <div
                key={e.id}
                className="explanation-card"
                onClick={() => navigate(`/explanations/${e.id}`)}
              >
                <h3>{e.title}</h3>
                <div className="meta">Updated {formatDate(e.updated_at)}</div>
              </div>
            ))}
          </div>
        )}
      </main>
    </>
  )
}
