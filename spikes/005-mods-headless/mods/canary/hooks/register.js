// Spike 005 (desechable). Solo observa: nunca cambia ni bloquea nada.
// Escribe una línea JSON por evento en $HILVAN_SPIKE005_LOG (ruta absoluta).

async function append($, entry) {
  const path = $.env.get('HILVAN_SPIKE005_LOG')
  if (!path) return
  const prev = (await $.fs.exists(path)) ? await $.fs.read(path) : ''
  const line = JSON.stringify({ t: new Date().toISOString(), mod: 'hv-canary', ...entry })
  await $.fs.write(path, prev + line + '\n')
}

export function register(on) {
  on('session.start', async ($, e, next) => {
    await append($, { event: 'session.start', cwd: $.session.cwd() })
    return next(e)
  })

  on('turn.start', async ($, e, next) => {
    await append($, { event: 'turn.start' })
    return next(e)
  })

  on('tool.call', async ($, e, next) => {
    await append($, { event: 'tool.call', tool: e.tool })
    return next(e)
  })

  on('tool.check', async ($, e, next) => {
    const result = await next(e)
    await append($, { event: 'tool.check', tool: e.tool, decision: result && result.decision })
    return result
  })
}
