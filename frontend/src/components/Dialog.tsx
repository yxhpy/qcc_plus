import type { ReactNode } from 'react'
import Modal from './Modal'

export interface DialogOptions {
  title?: string
  message?: ReactNode
  confirmText?: string
  cancelText?: string
  closeOnMask?: boolean
}

interface DialogProps extends DialogOptions {
  open: boolean
  onConfirm: () => void
  onCancel: () => void
}

export default function Dialog({
  open,
  title = '确认操作',
  message,
  confirmText = '确认',
  cancelText = '取消',
  closeOnMask = true,
  onConfirm,
  onCancel,
}: DialogProps) {
  return (
    <Modal
      open={open}
      title={title}
      closeOnMask={closeOnMask}
      onClose={onCancel}
      footer={
        <div className="dialog-actions">
          <button className="btn ghost" type="button" onClick={onCancel}>
            {cancelText}
          </button>
          <button className="btn primary" type="button" onClick={onConfirm}>
            {confirmText}
          </button>
        </div>
      }
    >
      {typeof message === 'string' ? <p className="dialog-message">{message}</p> : message}
    </Modal>
  )
}
