import { useEffect, useState } from 'react'
import Modal from './Modal'

export interface PromptField {
  name: string
  label: string
  type?: string
  defaultValue?: string
  placeholder?: string
  required?: boolean
  validate?: (value: string) => string | null
  options?: { label: string; value: string }[]
}

export interface PromptOptions {
  title?: string
  message?: string
  defaultValue?: string
  placeholder?: string
  type?: string
  required?: boolean
  closeOnMask?: boolean
  validate?: (value: string) => string | null
  size?: 'sm' | 'md' | 'lg'
}

export interface PromptFormOptions {
  title?: string
  message?: string
  fields: PromptField[]
  closeOnMask?: boolean
  size?: 'sm' | 'md' | 'lg'
}

interface PromptDialogProps {
  mode: 'input' | 'form'
  open: boolean
  options: PromptOptions | PromptFormOptions
  onSubmit: (value: string | Record<string, string>) => void
  onCancel: () => void
}

export default function PromptDialog({ mode, open, options, onSubmit, onCancel }: PromptDialogProps) {
  const [value, setValue] = useState(() => (mode === 'input' ? (options as PromptOptions).defaultValue || '' : ''))
  const [formValues, setFormValues] = useState<Record<string, string>>(() => {
    if (mode === 'form') {
      const fields = (options as PromptFormOptions).fields
      return fields.reduce<Record<string, string>>((acc, cur) => {
        const defaultVal = cur.defaultValue ?? (cur.type === 'select' ? cur.options?.[0]?.value ?? '' : '')
        acc[cur.name] = defaultVal
        return acc
      }, {})
    }
    return {}
  })
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setError('')
    if (mode === 'input') {
      setValue((options as PromptOptions).defaultValue || '')
    } else {
      const fields = (options as PromptFormOptions).fields
      setFormValues(
        fields.reduce<Record<string, string>>((acc, cur) => {
          const defaultVal = cur.defaultValue ?? (cur.type === 'select' ? cur.options?.[0]?.value ?? '' : '')
          acc[cur.name] = defaultVal
          return acc
        }, {})
      )
    }
  }, [mode, open, options])

  const title = (options as PromptOptions).title ?? (options as PromptFormOptions).title ?? '请输入'
  const message = (options as PromptOptions).message ?? (options as PromptFormOptions).message
  const closeOnMask = (options as PromptOptions).closeOnMask ?? (options as PromptFormOptions).closeOnMask ?? true
  const size = (options as PromptOptions).size ?? (options as PromptFormOptions).size

  const handleSubmit = () => {
    if (mode === 'input') {
      const opts = options as PromptOptions
      if (opts.required && !value.trim()) {
        setError('请输入内容')
        return
      }
      if (opts.validate) {
        const msg = opts.validate(value)
        if (msg) {
          setError(msg)
          return
        }
      }
      onSubmit(value)
      return
    }
    const opts = options as PromptFormOptions
    for (const field of opts.fields) {
      const val = formValues[field.name] ?? ''
      if (field.required && !val.trim()) {
        setError(`${field.label} 为必填项`)
        return
      }
      if (field.validate) {
        const msg = field.validate(val)
        if (msg) {
          setError(msg)
          return
        }
      }
    }
    onSubmit({ ...formValues })
  }

  const renderBody = () => {
    if (mode === 'input') {
      const opts = options as PromptOptions
      return (
        <form
          className="prompt-form"
          onSubmit={(e) => {
            e.preventDefault()
            handleSubmit()
          }}
        >
          {message && <p className="dialog-message">{message}</p>}
          <input
            autoFocus
            value={value}
            placeholder={opts.placeholder}
            type={opts.type || 'text'}
            data-autofocus="true"
            onChange={(e) => {
              setError('')
              setValue(e.target.value)
            }}
            required={opts.required}
          />
          {error && <p className="dialog-error">{error}</p>}
        </form>
      )
    }
    const opts = options as PromptFormOptions
    return (
      <form
        className="prompt-form"
        onSubmit={(e) => {
          e.preventDefault()
          handleSubmit()
        }}
      >
        {message && <p className="dialog-message">{message}</p>}
        <div className="prompt-grid">
          {opts.fields.map((field, idx) => (
            <label key={field.name} className="prompt-field">
              <span>{field.label}</span>
              {field.type === 'select' && field.options?.length ? (
                <select
                  value={formValues[field.name] ?? ''}
                  required={field.required}
                  data-autofocus={idx === 0 ? 'true' : undefined}
                  onChange={(e) => {
                    setError('')
                    setFormValues((prev) => ({
                      ...prev,
                      [field.name]: e.target.value,
                    }))
                  }}
                >
                  {field.options.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  value={formValues[field.name] ?? ''}
                  type={field.type || 'text'}
                  placeholder={field.placeholder}
                  required={field.required}
                  data-autofocus={idx === 0 ? 'true' : undefined}
                  onChange={(e) => {
                    setError('')
                    setFormValues((prev) => ({
                      ...prev,
                      [field.name]: e.target.value,
                    }))
                  }}
                />
              )}
            </label>
          ))}
        </div>
        {error && <p className="dialog-error">{error}</p>}
      </form>
    )
  }

  return (
    <Modal
      open={open}
      title={title}
      onClose={onCancel}
      closeOnMask={closeOnMask}
      size={size}
      footer={
        <div className="dialog-actions">
          <button className="btn ghost" type="button" onClick={onCancel}>
            取消
          </button>
          <button className="btn primary" type="button" onClick={() => handleSubmit()}>
            确定
          </button>
        </div>
      }
    >
      {renderBody()}
    </Modal>
  )
}
