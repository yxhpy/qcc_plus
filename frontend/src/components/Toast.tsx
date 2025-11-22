interface ToastProps {
  message?: string
  type?: 'success' | 'error'
}

export default function Toast({ message, type = 'success' }: ToastProps) {
  if (!message) return null
  return (
    <div className={`toast show ${type === 'error' ? 'error' : ''}`}>
      <span>{type === 'error' ? '⚠️' : '✅'}</span>
      <span>{message}</span>
    </div>
  )
}
