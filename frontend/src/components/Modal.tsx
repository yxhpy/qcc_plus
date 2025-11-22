import type { ReactNode } from 'react'
import { useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'

interface ModalProps {
  open: boolean
  title?: string
  children: ReactNode
  footer?: ReactNode
  onClose?: () => void
  closeOnMask?: boolean
}

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea, input, select, details,[tabindex]:not([tabindex="-1"])'

export default function Modal({
  open,
  title,
  children,
  footer,
  onClose,
  closeOnMask = true,
}: ModalProps) {
  const overlayRef = useRef<HTMLDivElement | null>(null)
  const modalRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!open) return
    const previousActive = document.activeElement as HTMLElement | null
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    const autoFocusEl = modalRef.current?.querySelector<HTMLElement>('[data-autofocus=\"true\"]')
    if (autoFocusEl) {
      autoFocusEl.focus()
    } else {
      const focusable = modalRef.current?.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
      const first = focusable?.[0]
      if (first) {
        first.focus()
      } else {
        modalRef.current?.focus()
      }
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose?.()
      }
      if (e.key === 'Tab') {
        const list = modalRef.current?.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
        if (!list || list.length === 0) return
        const firstEl = list[0]
        const lastEl = list[list.length - 1]
        if (e.shiftKey && document.activeElement === firstEl) {
          lastEl.focus()
          e.preventDefault()
        } else if (!e.shiftKey && document.activeElement === lastEl) {
          firstEl.focus()
          e.preventDefault()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = prevOverflow
      previousActive?.focus()
    }
  }, [open, onClose])

  if (!open) return null

  const content = (
    <div
      className="modal-overlay"
      ref={overlayRef}
      onMouseDown={(e) => {
        if (closeOnMask && e.target === overlayRef.current) {
          onClose?.()
        }
      }}
    >
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label={title || '对话框'}
        ref={modalRef}
        tabIndex={-1}
      >
        {title && (
          <div className="modal-header">
            <h3>{title}</h3>
            <button className="btn ghost" type="button" onClick={onClose} aria-label="关闭">
              X
            </button>
          </div>
        )}
        <div className="modal-body">{children}</div>
        {footer && <div className="modal-footer">{footer}</div>}
      </div>
    </div>
  )

  return createPortal(content, document.body)
}
