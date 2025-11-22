import type { ReactNode } from 'react'
import './Card.css'

interface CardProps {
  title?: string
  extra?: ReactNode
  children: ReactNode
  className?: string
}

export default function Card({ title, extra, children, className }: CardProps) {
  return (
    <section className={`card${className ? ' ' + className : ''}`}>
      {(title || extra) && (
        <div className="card-head">
          {title && <h3 className="card-title">{title}</h3>}
          {extra && <div className="card-extra">{extra}</div>}
        </div>
      )}
      {children}
    </section>
  )
}
