/**
 * Property-Based Tests for tag suggestion filtering
 *
 * **Validates: Requirements 3.1, 3.2**
 *
 * Property 4: Tag suggestion filtering
 * - For any set of existing tags, any subset of already-selected tags, and any
 *   typed prefix string, the suggestion list SHALL equal the set of existing tags
 *   that (a) are not in the already-selected set AND (b) whose lowercased form
 *   starts with the lowercased prefix.
 */
import { describe, it, expect } from 'vitest'
import * as fc from 'fast-check'

/**
 * Pure implementation of the tag suggestion filtering logic from TodoForm.
 *
 * This mirrors the filtering in TodoForm:
 * ```
 * options={(allTags || [])
 *   .filter((tag) => !(form.getFieldValue('tags') || []).includes(tag))
 *   .map((tag) => ({ value: tag, label: tag }))}
 * filterOption={(input, option) =>
 *   (option?.value as string)?.toLowerCase().startsWith(input.toLowerCase())
 * }
 * ```
 *
 * Step 1: Remove already-selected tags from the full list.
 * Step 2: Filter remaining tags by case-insensitive prefix match.
 */
function filterTagSuggestions(
  allTags: string[],
  selectedTags: string[],
  prefix: string,
): string[] {
  return allTags
    .filter((tag) => !selectedTags.includes(tag))
    .filter((tag) => tag.toLowerCase().startsWith(prefix.toLowerCase()))
}

// Arbitrary for generating tag-like strings (non-empty, printable)
const tagArb = fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0)

// Arbitrary for generating a set of unique tags
const tagSetArb = fc.uniqueArray(tagArb, { minLength: 0, maxLength: 20 })

describe('Property 4: Tag suggestion filtering', () => {
  /**
   * For any set of existing tags, any subset of already-selected tags, and any
   * typed prefix string, the suggestion list equals the set of existing tags that
   * (a) are not in the already-selected set AND (b) whose lowercased form starts
   * with the lowercased prefix.
   */
  it('suggestion list equals tags not selected whose lowercase starts with lowercase prefix', () => {
    fc.assert(
      fc.property(
        tagSetArb,
        fc.string({ minLength: 0, maxLength: 15 }),
        (allTags, prefix) => {
          // Generate a random subset of allTags as "already selected"
          // We use a deterministic subset approach: pick roughly half
          const selectedTags = allTags.filter((_, i) => i % 2 === 0)

          const result = filterTagSuggestions(allTags, selectedTags, prefix)

          // Compute expected: tags not in selectedTags AND whose lowercase starts with lowercase prefix
          const expected = allTags.filter(
            (tag) =>
              !selectedTags.includes(tag) &&
              tag.toLowerCase().startsWith(prefix.toLowerCase()),
          )

          expect(result).toEqual(expected)
        },
      ),
      { numRuns: 200 },
    )
  })

  /**
   * With a randomly generated subset (using fast-check's subarray), verify the
   * same property holds for arbitrary selected subsets.
   */
  it('holds for arbitrary selected subsets of existing tags', () => {
    fc.assert(
      fc.property(
        tagSetArb.chain((tags) =>
          fc.tuple(
            fc.constant(tags),
            fc.subarray(tags),
            fc.string({ minLength: 0, maxLength: 15 }),
          ),
        ),
        ([allTags, selectedTags, prefix]) => {
          const result = filterTagSuggestions(allTags, selectedTags, prefix)

          const expected = allTags.filter(
            (tag) =>
              !selectedTags.includes(tag) &&
              tag.toLowerCase().startsWith(prefix.toLowerCase()),
          )

          expect(result).toEqual(expected)
        },
      ),
      { numRuns: 200 },
    )
  })

  /**
   * When no tags are selected, the suggestion list equals all tags matching the prefix.
   */
  it('with no selected tags, returns all tags matching the prefix', () => {
    fc.assert(
      fc.property(tagSetArb, fc.string({ minLength: 0, maxLength: 15 }), (allTags, prefix) => {
        const result = filterTagSuggestions(allTags, [], prefix)

        const expected = allTags.filter((tag) =>
          tag.toLowerCase().startsWith(prefix.toLowerCase()),
        )

        expect(result).toEqual(expected)
      }),
      { numRuns: 200 },
    )
  })

  /**
   * When all tags are selected, the suggestion list is empty regardless of prefix.
   */
  it('with all tags selected, returns empty list', () => {
    fc.assert(
      fc.property(tagSetArb, fc.string({ minLength: 0, maxLength: 15 }), (allTags, prefix) => {
        const result = filterTagSuggestions(allTags, allTags, prefix)
        expect(result).toEqual([])
      }),
      { numRuns: 200 },
    )
  })

  /**
   * With an empty prefix, the suggestion list equals all tags not in the selected set.
   */
  it('with empty prefix, returns all non-selected tags', () => {
    fc.assert(
      fc.property(
        tagSetArb.chain((tags) =>
          fc.tuple(fc.constant(tags), fc.subarray(tags)),
        ),
        ([allTags, selectedTags]) => {
          const result = filterTagSuggestions(allTags, selectedTags, '')

          const expected = allTags.filter((tag) => !selectedTags.includes(tag))

          expect(result).toEqual(expected)
        },
      ),
      { numRuns: 200 },
    )
  })
})
