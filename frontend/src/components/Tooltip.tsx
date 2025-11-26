import { useEffect, useRef, useState, type ReactNode } from 'react'
import './Tooltip.css'

type TriggerType = 'hover' | 'click' | 'both'
type TooltipPosition = 'top' | 'bottom' | 'left' | 'right'

export interface TooltipProps {
  content: ReactNode
  children: ReactNode
  trigger?: TriggerType
  maxWidth?: string
  position?: TooltipPosition
}

export default function Tooltip({
  content,
  children,
  trigger = 'both',
  maxWidth = '300px',
  position = 'top',
}: TooltipProps) {
  const [visible, setVisible] = useState(false)
  const [pinned, setPinned] = useState(false)
  const wrapperRef = useRef<HTMLDivElement | null>(null)

  const enableHover = trigger === 'hover' || trigger === 'both'
  const enableClick = trigger === 'click' || trigger === 'both'

  const hasContent =
    content !== undefined &&
    content !== null &&
    content !== false &&
    !(typeof content === 'string' && content.trim() === '')
  if (!hasContent) return <>{children}</>

  const handleMouseEnter = () => {
    if (!enableHover) return
    setVisible(true)
  }

  const handleMouseLeave = () => {
    if (!enableHover) return
    if (pinned) return
    setVisible(false)
  }

  const handleClick = () => {
    if (!enableClick) return
    setVisible((prev) => {
      const next = !prev
      setPinned(next)
      return next
    })
  }

  useEffect(() => {
    if (!visible) return

    const handleOutside = (event: MouseEvent | TouchEvent) => {
      if (!wrapperRef.current) return
      if (!wrapperRef.current.contains(event.target as Node)) {
        setVisible(false)
        setPinned(false)
      }
    }

    document.addEventListener('mousedown', handleOutside)
    document.addEventListener('touchstart', handleOutside)
    return () => {
      document.removeEventListener('mousedown', handleOutside)
      document.removeEventListener('touchstart', handleOutside)
    }
  }, [visible])

  const positionClass = `tooltip-${position}`

  return (
    <div
      className="tooltip-wrapper"
      ref={wrapperRef}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      onClick={handleClick}
    >
      {children}
      <div
        className={`tooltip-content ${positionClass} ${visible ? 'show' : ''}`}
        style={{ maxWidth }}
        role="tooltip"
      >
        {content}
        <span className="tooltip-arrow" />
      </div>
    </div>
  )
}
