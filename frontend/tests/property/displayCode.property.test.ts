/**
 * Property-Based Tests for formatDisplayCode utility
 *
 * **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 7.1, 7.2, 7.3, 7.4**
 *
 * Property 8: Display code formatting
 * - For any valid category and any non-empty numeric code string,
 *   formatDisplayCode(category, code) SHALL return PREFIX + "-" + code
 *   where PREFIX is "B" for bug, "F" for feature, "T" for task.
 * - For empty/null/undefined code, it SHALL return only the PREFIX character.
 */
import { describe, it, expect } from 'vitest'
import * as fc from 'fast-check'
import { formatDisplayCode } from '../../src/utils/displayCode'
import type { Category } from '../../src/types'

const CATEGORY_PREFIX_MAP: Record<Category, string> = {
  bug: 'B',
  feature: 'F',
  task: 'T',
}

const categoryArb = fc.constantFrom<Category>('bug', 'feature', 'task')

// Generate non-empty numeric code strings (simulating real codes like "1", "42", "100")
const nonEmptyCodeArb = fc.nat({ max: 99999 }).map((n) => String(n + 1))

describe('Property 8: Display code formatting', () => {
  /**
   * For any valid category and any non-empty code string,
   * formatDisplayCode SHALL return PREFIX + "-" + code.
   */
  it('returns PREFIX-code for any valid category and non-empty numeric code', () => {
    fc.assert(
      fc.property(categoryArb, nonEmptyCodeArb, (category, code) => {
        const result = formatDisplayCode(category, code)
        const expectedPrefix = CATEGORY_PREFIX_MAP[category]
        expect(result).toBe(`${expectedPrefix}-${code}`)
      }),
      { numRuns: 100 },
    )
  })

  /**
   * For any valid category and any non-empty arbitrary string code (not just numeric),
   * formatDisplayCode SHALL return PREFIX + "-" + code.
   */
  it('returns PREFIX-code for any valid category and non-empty arbitrary code string', () => {
    // Generate non-empty, non-whitespace-only strings
    const nonEmptyStringArb = fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0)

    fc.assert(
      fc.property(categoryArb, nonEmptyStringArb, (category, code) => {
        const result = formatDisplayCode(category, code)
        const expectedPrefix = CATEGORY_PREFIX_MAP[category]
        expect(result).toBe(`${expectedPrefix}-${code}`)
      }),
      { numRuns: 100 },
    )
  })

  /**
   * For any valid category with null code, formatDisplayCode SHALL return only the PREFIX.
   */
  it('returns only PREFIX for null code', () => {
    fc.assert(
      fc.property(categoryArb, (category) => {
        const result = formatDisplayCode(category, null)
        const expectedPrefix = CATEGORY_PREFIX_MAP[category]
        expect(result).toBe(expectedPrefix)
      }),
      { numRuns: 100 },
    )
  })

  /**
   * For any valid category with undefined code, formatDisplayCode SHALL return only the PREFIX.
   */
  it('returns only PREFIX for undefined code', () => {
    fc.assert(
      fc.property(categoryArb, (category) => {
        const result = formatDisplayCode(category, undefined)
        const expectedPrefix = CATEGORY_PREFIX_MAP[category]
        expect(result).toBe(expectedPrefix)
      }),
      { numRuns: 100 },
    )
  })

  /**
   * For any valid category with empty string code, formatDisplayCode SHALL return only the PREFIX.
   */
  it('returns only PREFIX for empty string code', () => {
    fc.assert(
      fc.property(categoryArb, (category) => {
        const result = formatDisplayCode(category, '')
        const expectedPrefix = CATEGORY_PREFIX_MAP[category]
        expect(result).toBe(expectedPrefix)
      }),
      { numRuns: 100 },
    )
  })

  /**
   * For any valid category with whitespace-only code, formatDisplayCode SHALL return only the PREFIX.
   */
  it('returns only PREFIX for whitespace-only code', () => {
    const whitespaceArb = fc.array(fc.constantFrom(' ', '\t', '\n'), { minLength: 1, maxLength: 10 })
      .map((chars) => chars.join(''))

    fc.assert(
      fc.property(categoryArb, whitespaceArb, (category, code) => {
        const result = formatDisplayCode(category, code)
        const expectedPrefix = CATEGORY_PREFIX_MAP[category]
        expect(result).toBe(expectedPrefix)
      }),
      { numRuns: 100 },
    )
  })

  /**
   * The prefix mapping is exactly: bug → "B", feature → "F", task → "T".
   * Verify the mapping is correct for all categories.
   */
  it('maps categories to correct prefixes: bug→B, feature→F, task→T', () => {
    fc.assert(
      fc.property(nonEmptyCodeArb, (code) => {
        expect(formatDisplayCode('bug', code)).toMatch(/^B-/)
        expect(formatDisplayCode('feature', code)).toMatch(/^F-/)
        expect(formatDisplayCode('task', code)).toMatch(/^T-/)
      }),
      { numRuns: 100 },
    )
  })
})
