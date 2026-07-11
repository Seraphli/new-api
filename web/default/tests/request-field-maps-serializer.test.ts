import { describe, expect, test } from 'bun:test'
import {
  buildSettingsJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  type ChannelFormValues,
} from '../src/features/channels/lib/channel-form'
import type { Channel } from '../src/features/channels/types'

function baseForm(
  patch: Partial<ChannelFormValues> = {}
): ChannelFormValues {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 't',
    type: 1,
    ...patch,
  }
}

describe('request_field_maps serializer', () => {
  test('F1 empty list omits request_field_maps', () => {
    const json = buildSettingsJSON(
      baseForm({
        settings: JSON.stringify({ allow_service_tier: false }),
        request_field_maps: [],
      })
    )
    const obj = JSON.parse(json)
    expect(obj.request_field_maps).toBeUndefined()
  })

  test('F2 two rows P1+P2 with when normalize', () => {
    const json = buildSettingsJSON(
      baseForm({
        settings: '{}',
        request_field_maps: [
          { from: 'output_config.effort', to: 'reasoning_effort' },
          {
            when: 'claude_to_openai',
            from: 'service_tier',
            to: 'service_tier',
          },
        ],
      })
    )
    const obj = JSON.parse(json)
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

  test('F3 load CPA single row', () => {
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
    expect(form.request_field_maps).toEqual([
      {
        when: 'claude_to_openai',
        from: 'output_config.effort',
        to: 'reasoning_effort',
      },
    ])
  })

  test('F4 preserves unknown sentinel and maps', () => {
    // advanced_custom is type-gated and stripped for non-advanced channels;
    // merge-safe contract is unknown keys + maps on OtherSettings.
    const json = buildSettingsJSON(
      baseForm({
        settings: JSON.stringify({
          __sentinel_unknown__: 'keep-me',
          allow_service_tier: true,
        }),
        allow_service_tier: true,
        request_field_maps: [
          {
            when: 'claude_to_openai',
            from: 'output_config.effort',
            to: 'reasoning_effort',
          },
        ],
      })
    )
    const obj = JSON.parse(json)
    expect(obj.__sentinel_unknown__).toBe('keep-me')
    expect(obj.request_field_maps[0].to).toBe('reasoning_effort')
  })

  test('F5 schema has no map_effort field', () => {
    expect('map_effort_to_reasoning_effort' in CHANNEL_FORM_DEFAULT_VALUES).toBe(
      false
    )
    expect('request_field_maps' in CHANNEL_FORM_DEFAULT_VALUES).toBe(true)
  })
})
