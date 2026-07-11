export function loadRequestFieldMaps(parsedSettings) {
  const maps = parsedSettings?.request_field_maps;
  if (!Array.isArray(maps)) return [];
  return maps
    .filter((m) => m && typeof m.from === 'string' && typeof m.to === 'string')
    .map((m) => ({
      when:
        !m.when || String(m.when).trim() === ''
          ? 'claude_to_openai'
          : String(m.when).trim(),
      from: String(m.from).trim(),
      to: String(m.to).trim(),
    }));
}

export function saveRequestFieldMaps(settings, rows) {
  const list = Array.isArray(rows) ? rows : [];
  const cleaned = list
    .filter(
      (m) =>
        m &&
        typeof m.from === 'string' &&
        typeof m.to === 'string' &&
        m.from.trim() !== '' &&
        m.to.trim() !== '',
    )
    .map((m) => ({
      when:
        !m.when || String(m.when).trim() === ''
          ? 'claude_to_openai'
          : String(m.when).trim(),
      from: String(m.from).trim(),
      to: String(m.to).trim(),
    }));
  if (cleaned.length > 0) {
    settings.request_field_maps = cleaned;
  } else if ('request_field_maps' in settings) {
    delete settings.request_field_maps;
  }
  return settings;
}
