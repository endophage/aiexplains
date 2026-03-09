import type { Explanation, Section } from '../types'

const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export const api = {
  listExplanations: () =>
    request<Explanation[]>('/explanations'),

  createExplanation: (topic: string) =>
    request<Explanation>('/explanations', {
      method: 'POST',
      body: JSON.stringify({ topic }),
    }),

  getExplanation: (id: string) =>
    request<Explanation>(`/explanations/${id}`),

  explainSection: (explanationId: string, sectionId: string, prompt: string) =>
    request<{ section: Section; new_sections?: Section[] }>(`/explanations/${explanationId}/sections/${sectionId}/explain`, {
      method: 'POST',
      body: JSON.stringify({ prompt }),
    }),

  extendSection: (explanationId: string, sectionId: string, prompt: string) =>
    request<{ sections: Section[] }>(`/explanations/${explanationId}/sections/${sectionId}/extend`, {
      method: 'POST',
      body: JSON.stringify({ prompt }),
    }),

  updateTitle: (explanationId: string, title: string) =>
    request<Explanation>(`/explanations/${explanationId}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),

  deleteSection: (explanationId: string, sectionId: string) =>
    request<void>(`/explanations/${explanationId}/sections/${sectionId}`, { method: 'DELETE' }),

  restoreSection: (explanationId: string, sectionId: string) =>
    request<void>(`/explanations/${explanationId}/sections/${sectionId}/restore`, { method: 'POST' }),

  reorderSections: (explanationId: string, sectionIds: string[]) =>
    request<void>(`/explanations/${explanationId}/reorder`, {
      method: 'POST',
      body: JSON.stringify({ section_ids: sectionIds }),
    }),
}
