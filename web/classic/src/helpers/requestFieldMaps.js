export function loadRequestFieldMapsState(parsedSettings) {
  const maps = parsedSettings?.request_field_maps;
  const hasMaps = Array.isArray(maps) && maps.length > 0;
  let enabled = false;
  if (
    parsedSettings &&
    Object.prototype.hasOwnProperty.call(
      parsedSettings,
      'request_field_maps_enabled',
    )
  ) {
    enabled = parsedSettings.request_field_maps_enabled === true;
  } else {
    enabled = hasMaps;
  }
  return {
    request_field_maps_enabled: enabled,
    request_field_maps_json: hasMaps ? JSON.stringify(maps, null, 2) : '',
  };
}

export function saveRequestFieldMapsState(settings, enabled, jsonText) {
  // Always write enabled so explicit false is preserved.
  settings.request_field_maps_enabled = enabled === true;
  const raw = String(jsonText || '').trim();
  if (raw === '') {
    if ('request_field_maps' in settings) {
      delete settings.request_field_maps;
    }
    return settings;
  }
  const parsed = JSON.parse(raw);
  if (!Array.isArray(parsed)) {
    throw new Error('request_field_maps must be a JSON array');
  }
  const cleaned = parsed
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

// Back-compat names used by older tests
export function loadRequestFieldMaps(parsedSettings) {
  return loadRequestFieldMapsState(parsedSettings);
}

export function saveRequestFieldMaps(settings, rowsOrEnabled, maybeJson) {
  if (Array.isArray(rowsOrEnabled)) {
    // legacy: (settings, rows)
    const enabled = rowsOrEnabled.length > 0;
    return saveRequestFieldMapsState(
      settings,
      enabled,
      rowsOrEnabled.length ? JSON.stringify(rowsOrEnabled, null, 2) : '',
    );
  }
  return saveRequestFieldMapsState(settings, rowsOrEnabled, maybeJson);
}
