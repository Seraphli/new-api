import { describe, expect, test } from 'bun:test';
import { loadRequestFieldMaps, saveRequestFieldMaps } from './requestFieldMaps.js';

describe('classic requestFieldMaps serializer', () => {
  test('F1 empty list deletes key', () => {
    const settings = { request_field_maps: [{ from: 'x', to: 'y' }] };
    saveRequestFieldMaps(settings, []);
    expect(settings.request_field_maps).toBeUndefined();
  });

  test('F2 two rows normalize when', () => {
    const settings = { advanced_custom: { advanced_routes: [] }, __sentinel_unknown__: 'keep' };
    saveRequestFieldMaps(settings, [
      { from: 'output_config.effort', to: 'reasoning_effort' },
      { when: 'claude_to_openai', from: 'service_tier', to: 'service_tier' },
    ]);
    expect(settings.request_field_maps).toEqual([
      { when: 'claude_to_openai', from: 'output_config.effort', to: 'reasoning_effort' },
      { when: 'claude_to_openai', from: 'service_tier', to: 'service_tier' },
    ]);
    expect(settings.__sentinel_unknown__).toBe('keep');
    expect(settings.advanced_custom).toEqual({ advanced_routes: [] });
  });

  test('F3 load CPA row', () => {
    const rows = loadRequestFieldMaps({
      request_field_maps: [
        { from: 'output_config.effort', to: 'reasoning_effort' },
      ],
    });
    expect(rows).toEqual([
      {
        when: 'claude_to_openai',
        from: 'output_config.effort',
        to: 'reasoning_effort',
      },
    ]);
  });
});
