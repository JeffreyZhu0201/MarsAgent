import { useState } from 'react'
import { MarkdownView } from '@/components/MarkdownView'
import { approveDraft, rejectDraft, updateDraft, type WikiDraft } from '@/lib/api'

interface WikiDraftEditorProps {
  draft: WikiDraft
  onDone: () => void
}

export function WikiDraftEditor({ draft, onDone }: WikiDraftEditorProps) {
  const [title, setTitle] = useState(draft.title)
  const [category, setCategory] = useState(draft.category || 'general')
  const [content, setContent] = useState(draft.content_md)
  const [preview, setPreview] = useState(true)
  const [saving, setSaving] = useState(false)

  async function save() {
    setSaving(true)
    try {
      await updateDraft(draft.id, { title, category, content_md: content })
      onDone()
    } finally {
      setSaving(false)
    }
  }

  async function approve() {
    setSaving(true)
    try {
      await updateDraft(draft.id, { title, category, content_md: content })
      await approveDraft(draft.id)
      onDone()
    } finally {
      setSaving(false)
    }
  }

  async function reject() {
    setSaving(true)
    try {
      await rejectDraft(draft.id)
      onDone()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="space-y-3">
        <div>
          <label className="mb-1 block text-xs uppercase tracking-[0.2em] text-slate-400" htmlFor="draft-title">
            Title
          </label>
          <input
            id="draft-title"
            className="glass-input w-full"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Draft title"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs uppercase tracking-[0.2em] text-slate-400" htmlFor="draft-category">
            Category
          </label>
          <input
            id="draft-category"
            className="glass-input w-full"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            placeholder="general"
          />
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <button className="glass-button min-h-[2.5rem]" onClick={() => setPreview(!preview)}>
          {preview ? '编辑' : '预览'}
        </button>
        <button className="glass-button min-h-[2.5rem]" onClick={save} disabled={saving}>
          保存草稿
        </button>
        <button className="glass-button min-h-[2.5rem]" onClick={approve} disabled={saving}>
          确认发布
        </button>
        <button className="glass-button min-h-[2.5rem]" onClick={reject} disabled={saving}>
          拒绝
        </button>
      </div>

      {preview ? (
        <MarkdownView content={content} />
      ) : (
        <textarea
          className="glass-input min-h-96 w-full font-mono"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          aria-label="Markdown content"
        />
      )}
    </div>
  )
}
