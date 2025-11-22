import { useCallback } from 'react'
import { createRoot } from 'react-dom/client'
import PromptDialog, {
  type PromptFormOptions,
  type PromptOptions,
} from '../components/PromptDialog'

export interface PromptResult {
  [key: string]: string
}

export default function usePrompt() {
  const openPrompt = useCallback(
    (mode: 'input' | 'form', options: PromptOptions | PromptFormOptions) =>
      new Promise<string | PromptResult | null>((resolve) => {
        const container = document.createElement('div')
        document.body.appendChild(container)
        const root = createRoot(container)

        const cleanup = (value: string | PromptResult | null) => {
          root.unmount()
          container.remove()
          resolve(value)
        }

        root.render(
          <PromptDialog
            open
            mode={mode}
            options={options as PromptOptions}
            onSubmit={(val) => cleanup(val)}
            onCancel={() => cleanup(null)}
          />
        )
      }),
    []
  )

  const input = useCallback(
    (options: PromptOptions) => openPrompt('input', options) as Promise<string | null>,
    [openPrompt]
  )
  const form = useCallback(
    (options: PromptFormOptions) => openPrompt('form', options) as Promise<PromptResult | null>,
    [openPrompt]
  )

  return { input, form }
}
