import { useState, useEffect, useRef } from 'react'
import { ArrowLeft, Music, Send, Pause, Play, XCircle } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { formatTimeAgo, formatDuration } from '@/lib/utils'

const apiKey = () => localStorage.getItem('cc_api_key') || ''
const headers = () => ({ 'Content-Type': 'application/json', 'X-API-Key': apiKey() })

export function OrchestraDetail({ orchestraId }: { orchestraId: string }) {
  const [score, setScore] = useState<any>(null)
  const [chat, setChat] = useState<any[]>([])
  const [chatInput, setChatInput] = useState('')
  const [loading, setLoading] = useState(true)
  const chatEndRef = useRef<HTMLDivElement>(null)

  // Load score
  useEffect(() => {
    fetch(`/api/v1/orchestras/${orchestraId}/score`, { headers: headers() })
      .then(r => r.json())
      .then(d => { setScore(d.data); setLoading(false) })
      .catch(() => setLoading(false))
  }, [orchestraId])

  // Load chat
  useEffect(() => {
    fetch(`/api/v1/orchestras/${orchestraId}/chat`, { headers: headers() })
      .then(r => r.json())
      .then(d => setChat(d.data || []))
      .catch(() => {})
  }, [orchestraId])

  // SSE stream for real-time updates
  useEffect(() => {
    const es = new EventSource(`/api/v1/orchestras/${orchestraId}/stream`)
    es.addEventListener('chat', (e) => {
      const msg = JSON.parse(e.data)
      setChat(prev => [...prev, msg])
    })
    es.addEventListener('state', (e) => {
      const data = JSON.parse(e.data)
      setScore((prev: any) => prev ? { ...prev, orchestra: { ...prev.orchestra, state: data.state, movement_count: data.movement_count } } : prev)
    })
    return () => es.close()
  }, [orchestraId])

  // Auto-scroll chat
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chat])

  const sendChat = async () => {
    if (!chatInput.trim()) return
    await fetch(`/api/v1/orchestras/${orchestraId}/chat`, {
      method: 'POST', headers: headers(),
      body: JSON.stringify({ content: chatInput, sender_type: 'human' }),
    })
    setChatInput('')
  }

  const handleAction = async (action: string) => {
    await fetch(`/api/v1/orchestras/${orchestraId}/${action}`, { method: 'POST', headers: headers() })
    // Reload score
    const res = await fetch(`/api/v1/orchestras/${orchestraId}/score`, { headers: headers() })
    const d = await res.json()
    setScore(d.data)
  }

  const handleChoose = async (runId: string, index: number) => {
    await fetch(`/api/v1/runs/${runId}/choose`, {
      method: 'POST', headers: headers(),
      body: JSON.stringify({ choice_index: index }),
    })
    // Reload
    const res = await fetch(`/api/v1/orchestras/${orchestraId}/score`, { headers: headers() })
    setScore((await res.json()).data)
  }

  const goBack = () => {
    window.history.pushState(null, '', '/orchestras')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  if (loading) return <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
  if (!score) return <div className="p-8 text-center text-sm text-muted-foreground">Orchestra not found</div>

  const orch = score.orchestra
  const movements = score.movements || []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <button onClick={goBack} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
            <ArrowLeft size={12} /> Back to orchestras
          </button>
          <div className="flex items-center gap-3">
            <Music size={20} className="text-indigo-400" />
            <h1 className="text-xl font-semibold tracking-tight">{orch.name}</h1>
            <StateBadge state={orch.state} />
          </div>
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <span>{orch.director_type === 'ai' ? 'AI Director' : orch.director_type === 'process' ? 'Code Director' : 'No Director'}</span>
            <span>·</span>
            <span>{orch.movement_count} movements</span>
            <span>·</span>
            <span>{formatTimeAgo(orch.created_at)}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {orch.state === 'active' && (
            <button type="button" onClick={() => handleAction('pause')}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-sm hover:bg-amber-500/10 text-amber-400 transition-colors">
              <Pause size={13} /> Pause
            </button>
          )}
          {orch.state === 'paused' && (
            <button type="button" onClick={() => handleAction('resume')}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-sm hover:bg-emerald-500/10 text-emerald-400 transition-colors">
              <Play size={13} /> Resume
            </button>
          )}
          {(orch.state === 'active' || orch.state === 'paused') && (
            <button type="button" onClick={() => { if (confirm('Cancel this orchestra?')) handleAction('cancel') }}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-red-500/20 text-sm hover:bg-red-500/10 text-red-400 transition-colors">
              <XCircle size={13} /> Cancel
            </button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Score Timeline (2/3 width) */}
        <div className="lg:col-span-2 space-y-4">
          <h2 className="text-sm font-semibold">Score</h2>

          {movements.length === 0 ? (
            <div className="rounded-lg border border-border bg-card p-6 text-center text-sm text-muted-foreground">
              No movements yet
            </div>
          ) : (
            <div className="space-y-3">
              {movements.map((mov: any, i: number) => {
                let choiceConfig: any = null
                if (mov.choice_config) {
                  try { choiceConfig = JSON.parse(typeof mov.choice_config === 'string' ? mov.choice_config : JSON.stringify(mov.choice_config)) } catch {}
                }

                return (
                  <div key={mov.id} className="flex gap-4">
                    <div className="flex flex-col items-center">
                      <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-mono flex-shrink-0 ${
                        mov.state === 'completed' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' :
                        mov.state === 'failed' ? 'bg-red-500/10 text-red-400 border border-red-500/30' :
                        mov.state === 'waiting_for_choice' ? 'bg-amber-500/10 text-amber-400 border border-amber-500/30' :
                        mov.state === 'running' ? 'bg-indigo-500/10 text-indigo-400 border border-indigo-500/30 animate-pulse' :
                        'bg-muted text-muted-foreground border border-border'
                      }`}>
                        {mov.orchestra_step || i + 1}
                      </div>
                      {i < movements.length - 1 && <div className="w-px flex-1 bg-border mt-1"></div>}
                    </div>
                    <div className="flex-1 pb-4">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">{mov.process_id?.slice(0, 20)}</span>
                        <StateBadge state={mov.state} />
                        {mov.duration_ms && <span className="text-xs text-muted-foreground">{formatDuration(mov.duration_ms)}</span>}
                      </div>
                      {mov.result && (
                        <pre className="mt-1 text-xs text-muted-foreground font-mono bg-[#0a0a0c] p-2 rounded max-h-20 overflow-auto">
                          {JSON.stringify(typeof mov.result === 'string' ? JSON.parse(mov.result) : mov.result, null, 2).slice(0, 200)}
                        </pre>
                      )}
                      {mov.state === 'waiting_for_choice' && choiceConfig && (
                        <div className="mt-2 p-3 rounded-lg border border-amber-500/20 bg-amber-500/5">
                          <p className="text-xs text-amber-400 mb-2">{choiceConfig.message}</p>
                          <div className="flex flex-wrap gap-2">
                            {(choiceConfig.choices || []).map((c: any, ci: number) => (
                              <button key={ci} type="button" onClick={() => handleChoose(mov.id, ci)}
                                className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                                  c.style === 'danger' ? 'bg-red-500/10 text-red-400 hover:bg-red-500/20 border border-red-500/20' :
                                  c.style === 'primary' ? 'bg-indigo-500 text-white hover:bg-indigo-400' :
                                  'border border-border text-muted-foreground hover:text-foreground hover:bg-muted/50'
                                }`}>
                                {c.label}
                              </button>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* Chat Panel (1/3 width) */}
        <div className="space-y-3">
          <h2 className="text-sm font-semibold">Chat</h2>
          <div className="rounded-lg border border-border bg-card flex flex-col h-[500px]">
            <div className="flex-1 overflow-y-auto p-3 space-y-2">
              {chat.length === 0 ? (
                <p className="text-xs text-muted-foreground text-center py-8">No messages yet</p>
              ) : chat.map((msg: any) => (
                <div key={msg.id} className={`text-xs ${
                  msg.sender_type === 'system' ? 'text-muted-foreground italic' :
                  msg.sender_type === 'director' ? 'text-indigo-400' :
                  msg.sender_type === 'human' ? 'text-foreground' :
                  'text-muted-foreground'
                }`}>
                  <span className={`font-mono text-[10px] ${
                    msg.message_type === 'warning' ? 'text-amber-400' :
                    msg.message_type === 'status' ? 'text-muted-foreground' :
                    ''
                  }`}>
                    [{msg.sender_type}]
                  </span>{' '}
                  {msg.content}
                </div>
              ))}
              <div ref={chatEndRef} />
            </div>
            <div className="border-t border-border p-2 flex gap-2">
              <input
                value={chatInput}
                onChange={e => setChatInput(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && sendChat()}
                placeholder="Type a message..."
                className="flex-1 px-2 py-1.5 rounded text-xs bg-background border border-border focus:outline-none focus:border-indigo-500/40"
              />
              <button type="button" onClick={sendChat}
                className="p-1.5 rounded text-indigo-400 hover:bg-indigo-500/10 transition-colors">
                <Send size={14} />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
