import { integrationRoutes } from '../../config/modelCatalog'

interface IntegrationGuideProps {
  proxyUrl: string
  apiKey: string
}

const buildCurlExample = (route: string, proxyUrl: string, apiKey: string) => {
  const base = proxyUrl.trim() || 'http://localhost:8000'
  const token = apiKey.trim() || '<proxy_api_key>'

  if (route.includes('/v1/messages')) {
    return `curl ${base}/v1/messages \\
  -H "x-api-key: ${token}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4-5-20250929","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}'`
  }

  if (route.includes('/v1beta/models/{model}:generateContent')) {
    return `curl "${base}/v1beta/models/gemini-2.5-flash:generateContent" \\
  -H "x-goog-api-key: ${token}" \\
  -H "Content-Type: application/json" \\
  -d '{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}'`
  }

  return `curl ${base}/v1/responses \\
  -H "Authorization: Bearer ${token}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-5.1-codex-mini","input":"hi"}'`
}

export default function IntegrationGuide({ proxyUrl, apiKey }: IntegrationGuideProps) {
  return (
    <div className="integration-guide-grid">
      {integrationRoutes.map((item) => (
        <section key={item.title} className="integration-guide-card">
          <div className="section-head compact">
            <div>
              <div className="eyebrow">快速对接</div>
              <h3 className="section-title">{item.title}</h3>
            </div>
            <span className="badge ghost">{item.protocol}</span>
          </div>
          <div className="integration-route">{item.route}</div>
          <p className="integration-note">{item.note}</p>
          <div className="integration-hint">{item.bodyHint}</div>
          <pre className="integration-code">{buildCurlExample(item.route, proxyUrl, apiKey)}</pre>
        </section>
      ))}
    </div>
  )
}
