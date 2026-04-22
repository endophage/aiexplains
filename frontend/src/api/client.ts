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
  listExplanations: (tags?: string[]) =>
    request<Explanation[]>('/explanations' + (tags?.length ? `?tags=${tags.join(',')}` : '')),

  listTags: () =>
    request<string[]>('/tags'),

  addTag: (explanationId: string, tag: string) =>
    request<{ tag: string }>(`/explanations/${explanationId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ tag }),
    }),

  removeTag: (explanationId: string, tag: string) =>
    request<void>(`/explanations/${explanationId}/tags/${encodeURIComponent(tag)}`, { method: 'DELETE' }),

  createTag: (tag: string) =>
    request<{ tag: string }>('/tags', {
      method: 'POST',
      body: JSON.stringify({ tag }),
    }),

  deleteTag: (tag: string) =>
    request<void>(`/tags/${encodeURIComponent(tag)}`, { method: 'DELETE' }),

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

  deleteExplanation: (explanationId: string) =>
    request<void>(`/explanations/${explanationId}`, { method: 'DELETE' }),

  regenerateExplanation: (explanationId: string, prompt: string) =>
    request<{ sections: Section[] }>(`/explanations/${explanationId}/regenerate`, {
      method: 'POST',
      body: JSON.stringify({ prompt }),
    }),

  updateTitle: (explanationId: string, title: string) =>
    request<Explanation>(`/explanations/${explanationId}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),

  branchSection: (explanationId: string, sectionId: string, prompt: string) =>
    request<{ sections: Section[] }>(`/explanations/${explanationId}/sections/${sectionId}/branch`, {
      method: 'POST',
      body: JSON.stringify({ prompt }),
    }),

  extractSection: (explanationId: string, sectionId: string, extractedHtml: string, remainingHtml: string, originalTitle: string, newTitle: string) =>
    request<{ section: Section; new_section: Section }>(`/explanations/${explanationId}/sections/${sectionId}/extract`, {
      method: 'POST',
      body: JSON.stringify({ extracted_html: extractedHtml, remaining_html: remainingHtml, original_title: originalTitle, new_title: newTitle }),
    }),

  extractChildSection: (explanationId: string, sectionId: string, extractedHtml: string, remainingHtml: string, originalTitle: string, newTitle: string) =>
    request<{ section: Section; new_section: Section }>(`/explanations/${explanationId}/sections/${sectionId}/extract-child`, {
      method: 'POST',
      body: JSON.stringify({ extracted_html: extractedHtml, remaining_html: remainingHtml, original_title: originalTitle, new_title: newTitle }),
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
