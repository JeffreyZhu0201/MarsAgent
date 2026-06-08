export interface EchoResponse { task_id: string }

export async function postEcho(msg: string): Promise<EchoResponse> {
  const r = await fetch('/api/echo', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ msg }),
  })
  if (!r.ok) {
    const text = await r.text()
    throw new Error(`POST /api/echo failed: ${r.status} ${text}`)
  }
  return r.json()
}
