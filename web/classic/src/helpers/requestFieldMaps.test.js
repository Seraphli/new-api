import { describe, expect, test } from 'bun:test';
import {
  loadRequestFieldMapsState,
  saveRequestFieldMapsState,
} from './requestFieldMaps.js';

describe('classic requestFieldMaps serializer r4', () => {
  test('F1 enabled false keeps maps and writes false key', () => {
    const settings = { __sentinel_unknown__: 'keep' };
    saveRequestFieldMapsState(
      settings,
      false,
      JSON.stringify([
        {
          when: 'claude_to_openai',
          from: 'output_config.effort',
          to: 'reasoning_effort',
        },
      ]),
    );
    expect(settings.request_field_maps_enabled).toBe(false);
    expect(settings.request_field_maps[0].to).toBe('reasoning_effort');
    expect(settings.__sentinel_unknown__).toBe('keep');
  });

  test('F3 load CPA absent enabled => on + json', () => {
    const state = loadRequestFieldMapsState({
      request_field_maps: [
        { from: 'output_config.effort', to: 'reasoning_effort' },
      ],
    });
    expect(state.request_field_maps_enabled).toBe(true);
    expect(state.request_field_maps_json).toContain('reasoning_effort');
  });

  test('F7 explicit false load', () => {
    const state = loadRequestFieldMapsState({
      request_field_maps_enabled: false,
      request_field_maps: [
        {
          when: 'claude_to_openai',
          from: 'output_config.effort',
          to: 'reasoning_effort',
        },
      ],
    });
    expect(state.request_field_maps_enabled).toBe(false);
    expect(state.request_field_maps_json).toContain('output_config.effort');
  });
});
