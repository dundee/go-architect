import { useState } from 'react'
import { AlertCircle, FolderOpen, Loader2, X } from 'lucide-react'
import { switchProjectPath } from '../api/client'
import type { ProjectContextResponse } from '../types/api'

interface ProjectSwitcherModalProps {
  currentPath: string
  currentOrgPackages: string[]
  onClose: () => void
  onSwitched: (ctx: ProjectContextResponse) => void
}

export function ProjectSwitcherModal({
  currentPath,
  currentOrgPackages,
  onClose,
  onSwitched,
}: ProjectSwitcherModalProps) {
  const [pathInput, setPathInput] = useState(currentPath)
  const [orgInput, setOrgInput] = useState(currentOrgPackages.join('\n'))
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = pathInput.trim()
    if (!trimmed) {
      setError('Please enter a path to a Go Project directory.')
      return
    }
    const parsedOrgPackages = orgInput
      .split(/[\n,]+/)
      .map((item) => item.trim())
      .filter(Boolean)

    setSubmitting(true)
    setError(null)
    try {
      const ctx = await switchProjectPath(trimmed, parsedOrgPackages)
      onSwitched(ctx)
      onClose()
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : 'Failed to load Go Project',
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 p-4 backdrop-blur-xs"
      role="dialog"
      aria-modal="true"
      aria-labelledby="switch-project-title"
    >
      <div className="w-full max-w-lg rounded-2xl border border-slate-200 bg-white p-6 shadow-2xl">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600">
              <FolderOpen className="h-5 w-5" />
            </div>
            <div>
              <h2
                id="switch-project-title"
                className="text-base font-semibold text-slate-900"
              >
                Project Settings
              </h2>
              <p className="text-xs text-slate-500">
                Configure target local Go Project directory and organization
                package prefixes.
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600"
            aria-label="Close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-5 flex flex-col gap-4">
          <div>
            <label
              htmlFor="project-path-input"
              className="mb-1.5 block text-xs font-medium text-slate-700"
            >
              Local Directory Path
            </label>
            <input
              id="project-path-input"
              type="text"
              value={pathInput}
              onChange={(e) => setPathInput(e.target.value)}
              placeholder="/path/to/your/go-project"
              autoFocus
              className="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-sm text-slate-900 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 focus:outline-none"
            />
          </div>

          <div>
            <label
              htmlFor="org-packages-input"
              className="mb-1.5 block text-xs font-medium text-slate-700"
            >
              Organization Package Prefixes{' '}
              <span className="font-normal text-slate-400">
                (optional, one per line or comma-separated)
              </span>
            </label>
            <textarea
              id="org-packages-input"
              rows={3}
              value={orgInput}
              onChange={(e) => setOrgInput(e.target.value)}
              placeholder="github.com/my-org/"
              className="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs text-slate-900 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 focus:outline-none"
            />
            <p className="mt-1 text-[11px] text-slate-500">
              Used by Dependency Graph and DSM to distinguish{' '}
              <code className="font-mono">organization</code> packages from
              third-party <code className="font-mono">external</code> ones.
            </p>
          </div>

          {error && (
            <div className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-800">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
              <span>{error}</span>
            </div>
          )}

          <div className="flex items-center justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg px-3.5 py-2 text-xs font-medium text-slate-600 transition hover:bg-slate-100"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition hover:bg-indigo-700 disabled:opacity-60"
            >
              {submitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              Apply Project
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
