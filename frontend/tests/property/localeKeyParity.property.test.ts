/**
 * Property-Based Test for Locale Key Parity
 *
 * **Validates: Requirements 6.3**
 *
 * Property 8: Locale files maintain structural key parity
 * - For any translation key present in the zh locale file, that same key
 *   SHALL also be present in the en locale file, and vice versa.
 * - The two locale files SHALL have identical key structures.
 */
import { describe, it, expect } from 'vitest'
import zh from '../../src/i18n/zh'
import en from '../../src/i18n/en'

/**
 * Recursively extracts all keys from a nested object, using dot notation
 * for nested keys (e.g., "aiSummary.drawer.title").
 */
function extractKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = []
  for (const key of Object.keys(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key
    const value = obj[key]
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      keys.push(...extractKeys(value as Record<string, unknown>, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

describe('Property 8: Locale files maintain structural key parity', () => {
  const zhKeys = extractKeys(zh as unknown as Record<string, unknown>)
  const enKeys = extractKeys(en as unknown as Record<string, unknown>)

  it('every key in zh.ts exists in en.ts', () => {
    const enKeySet = new Set(enKeys)
    const missingInEn = zhKeys.filter((key) => !enKeySet.has(key))
    expect(missingInEn, `Keys in zh.ts missing from en.ts: ${missingInEn.join(', ')}`).toEqual([])
  })

  it('every key in en.ts exists in zh.ts', () => {
    const zhKeySet = new Set(zhKeys)
    const missingInZh = enKeys.filter((key) => !zhKeySet.has(key))
    expect(missingInZh, `Keys in en.ts missing from zh.ts: ${missingInZh.join(', ')}`).toEqual([])
  })

  it('both locale files have the same total number of keys', () => {
    expect(zhKeys.length).toBe(enKeys.length)
  })
})
