import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const LOCALES = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi'] as const

const REQUIRED_KEYS = [
  'Enable request field maps',
  'When on, apply the JSON maps below on Claude→OpenAI conversion. When off, maps are kept but not applied.',
  'Request field maps JSON',
  'JSON array of {when,from,to}. Only allowlisted pairs are accepted by the server. service_tier also needs allow service_tier on this channel.',
  'Fill effort example',
  'Fill effort + service_tier example',
] as const

describe('request field maps i18n namespace', () => {
  for (const loc of LOCALES) {
    test(`${loc}: keys live under translation, not root`, () => {
      const path = join(
        import.meta.dir,
        `../src/i18n/locales/${loc}.json`
      )
      const data = JSON.parse(readFileSync(path, 'utf8')) as Record<
        string,
        unknown
      >
      expect(data.translation && typeof data.translation === 'object').toBe(
        true
      )
      const tr = data.translation as Record<string, string>
      for (const key of REQUIRED_KEYS) {
        expect(Object.prototype.hasOwnProperty.call(data, key)).toBe(false)
        expect(typeof tr[key]).toBe('string')
        expect(tr[key].length).toBeGreaterThan(0)
      }
      if (loc === 'zh') {
        expect(tr['Enable request field maps']).toBe('启用请求字段映射')
        expect(tr['Enable request field maps']).not.toBe(
          'Enable request field maps'
        )
      }
    })
  }
})
