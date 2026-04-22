export interface SectionVersion {
  version: number
  content: string
}

export interface Section {
  id: string
  parent_id?: string | null
  current_version: number
  deleted?: boolean
  versions: SectionVersion[]
}

export interface Explanation {
  id: string
  title: string
  topic: string
  tags: string[]
  created_at: string
  updated_at: string
  sections?: Section[]
}
