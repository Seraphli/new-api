import { describe, expect, test } from 'bun:test'
import {
  buildSettingsJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  type ChannelFormValues,
} from '../src/features/channels/lib/channel-form'
import type { Channel } from '../src/features/channels/types'

function baseForm(patch: Partial<ChannelFormValues> = {}): ChannelFormValues {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 't',
    type: 1,
    ...patch,
  }
}

describe('request_field_maps serializer r4', () => {
  test('F1 enabled false keeps maps and writes false key', () => {
    const json = buildSettingsJSON(
      baseForm({
        settings: JSON.stringify({ __sentinel_unknown__: 'keep-me' }),
        request_field_maps_enabled: false,
        request_field_maps_json: JSON.stringify([
          {
            when: 'claude_to_openai',
            from: 'output_config.effort',
            to: 'reasoning_effort',
          },
        ]),
      })
    )
    const obj = JSON.parse(json)
    expect(obj.request_field_maps_enabled).toBe(false)
    expect(obj.request_field_maps[0].to).toBe('reasoning_effort')
    expect(obj.__sentinel_unknown__).toBe('keep-me')
  })

  test('F2 two rows when normalize', () => {
    const json = buildSettingsJSON(
      baseForm({
        settings: '{}',
        request_field_maps_enabled: true,
        request_field_maps_json: JSON.stringify([
          { from: 'output_config.effort', to: 'reasoning_effort' },
          {
            when: 'claude_to_openai',
            from: 'service_tier',
            to: 'service_tier',
          },
        ]),
      })
    )
    const obj = JSON.parse(json)
    expect(obj.request_field_maps_enabled).toBe(true)
    expect(obj.request_field_maps).toEqual([
      {
        when: 'claude_to_openai',
        from: 'output_config.effort',
        to: 'reasoning_effort',
      },
      {
        when: 'claude_to_openai',
        from: 'service_tier',
        to: 'service_tier',
      },
    ])
  })

  test('F3 load CPA absent enabled => on + json', () => {
    const channel = {
      id: 6,
      type: 1,
      name: 'CPA',
      settings: JSON.stringify({
        request_field_maps: [
          {
            when: 'claude_to_openai',
            from: 'output_config.effort',
            to: 'reasoning_effort',
          },
        ],
      }),
      channel_info: {},
    } as unknown as Channel
    const form = transformChannelToFormDefaults(channel)
    expect(form.request_field_maps_enabled).toBe(true)
    expect(form.request_field_maps_json).toContain('reasoning_effort')
  })

  test('F5 no map_effort field', () => {
    expect('map_effort_to_reasoning_effort' in CHANNEL_FORM_DEFAULT_VALUES).toBe(
      false
    )
    expect('request_field_maps_enabled' in CHANNEL_FORM_DEFAULT_VALUES).toBe(
      true
    )
  })

  test('F7 explicit false load', () => {
    const channel = {
      id: 1,
      type: 1,
      name: 'x',
      settings: JSON.stringify({
        request_field_maps_enabled: false,
        request_field_maps: [
          {
            when: 'claude_to_openai',
            from: 'output_config.effort',
            to: 'reasoning_effort',
          },
        ],
      }),
      channel_info: {},
    } as unknown as Channel
    const form = transformChannelToFormDefaults(channel)
    expect(form.request_field_maps_enabled).toBe(false)
    expect(form.request_field_maps_json).toContain('output_config.effort')
  })
})
