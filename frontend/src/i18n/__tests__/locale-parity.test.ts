/**
 * Property 10: Locale structural parity
 *
 * **Validates: Requirements 9.3**
 *
 * For any translation key present in the Chinese (zh) locale object,
 * the same key path SHALL exist in the English (en) locale object, and vice versa.
 */
import { describe, it, expect } from 'vitest'
import zh from '../zh'
import en from '../en'

/**
 * Recursively extracts all dot-notation key paths from a nested object.
 * e.g., { a: { b: 'x', c: 'y' } } => ['a.b', 'a.c']
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

describe('Feature: ai-summary-followup, Property 10: Locale structural parity', () => {
  const zhKeys = extractKeys(zh as unknown as Record<string, unknown>)
  const enKeys = extractKeys(en as unknown as Record<string, unknown>)

  it('every key in zh exists in en', () => {
    const enKeySet = new Set(enKeys)
    const missingInEn = zhKeys.filter((key) => !enKeySet.has(key))
    expect(
      missingInEn,
      `Keys present in zh but missing from en:\n  ${missingInEn.join('\n  ')}`
    ).toEqual([])
  })

  it('every key in en exists in zh', () => {
    const zhKeySet = new Set(zhKeys)
    const missingInZh = enKeys.filter((key) => !zhKeySet.has(key))
    expect(
      missingInZh,
      `Keys present in en but missing from zh:\n  ${missingInZh.join('\n  ')}`
    ).toEqual([])
  })
})
