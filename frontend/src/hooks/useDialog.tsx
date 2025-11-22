import { useCallback, useRef } from 'react'
import { createRoot } from 'react-dom/client'
import Dialog, { type DialogOptions } from '../components/Dialog'

export default function useDialog() {
  const busyRef = useRef(false)

  const confirm = useCallback(
    (options: DialogOptions) =>
      new Promise<boolean>((resolve) => {
        if (busyRef.current) {
          resolve(false)
          return
        }
        busyRef.current = true
        const container = document.createElement('div')
        document.body.appendChild(container)
        const root = createRoot(container)

        const cleanup = (result: boolean) => {
          busyRef.current = false
          root.unmount()
          container.remove()
          resolve(result)
        }

        root.render(
          <Dialog
            open
            {...options}
            onConfirm={() => cleanup(true)}
            onCancel={() => cleanup(false)}
          />
        )
      }),
    []
  )

  return { confirm }
}
