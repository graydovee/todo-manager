/**
 * Bug Condition Exploration Test
 *
 * **Validates: Requirements 1.1, 1.2, 1.3, 1.4**
 *
 * These tests are EXPECTED TO FAIL on unfixed code.
 * Failure confirms the bugs exist:
 * - Bug 1: handlePrerequisiteCreated calls raw updateTodo() instead of mutation hook's mutateAsync(),
 *   so React Query cache is never invalidated after prerequisite relation is established
 * - Bug 2: TodoSummary interface lacks `status` field, and SummaryLink component does not render a status Tag
 */
import { describe, it, expect } from 'vitest'
import * as fc from 'fast-check'
import * as fs from 'node:fs'
import * as path from 'node:path'

describe('Bug Condition Exploration: Cache Invalidation Failure & Missing Status Field', () => {
  /**
   * Bug 1: handlePrerequisiteCreated uses direct updateTodo() call instead of useUpdateTodo mutation hook.
   *
   * This test reads the source code of TodoListPage.tsx and verifies that handlePrerequisiteCreated
   * uses the mutation hook (mutateAsync) rather than the raw updateTodo import from api/todos.
   * On unfixed code, handlePrerequisiteCreated calls updateTodo() directly, bypassing React Query
   * cache invalidation.
   */
  it('handlePrerequisiteCreated should use useUpdateTodo mutation hook for cache invalidation', () => {
    const todoListPagePath = path.resolve(__dirname, '../../src/pages/TodoListPage.tsx')
    const source = fs.readFileSync(todoListPagePath, 'utf-8')

    fc.assert(
      fc.property(fc.constant(source), (src) => {
        // Extract the handlePrerequisiteCreated function body
        const fnMatch = src.match(/const handlePrerequisiteCreated[\s\S]*?(?=\n\s*(?:const |return |function ))/m)
        expect(fnMatch).not.toBeNull()
        const fnBody = fnMatch![0]

        // The function should NOT use direct updateTodo() call from api/todos
        // It should use mutateAsync from the useUpdateTodo hook
        const usesDirectUpdateTodo = fnBody.includes('await updateTodo(')
        const usesMutateAsync = fnBody.includes('mutateAsync(') || fnBody.includes('.mutateAsync(')

        // Bug condition: direct API call is used instead of mutation hook
        // Expected behavior: mutateAsync is used, direct updateTodo is NOT used
        expect(usesDirectUpdateTodo).toBe(false)
        expect(usesMutateAsync).toBe(true)

        return true
      }),
      { numRuns: 1 },
    )
  })

  /**
   * Bug 1 (supplementary): Verify that updateTodo is NOT imported from api/todos for use in
   * handlePrerequisiteCreated. The import should be removed or only getTodo should remain.
   */
  it('TodoListPage should not import updateTodo from api/todos (or use it only via hook)', () => {
    const todoListPagePath = path.resolve(__dirname, '../../src/pages/TodoListPage.tsx')
    const source = fs.readFileSync(todoListPagePath, 'utf-8')

    fc.assert(
      fc.property(fc.constant(source), (src) => {
        // Check if updateTodo is imported from '../api/todos'
        const importMatch = src.match(/import\s*\{([^}]*)\}\s*from\s*['"]\.\.\/api\/todos['"]/)
        if (importMatch) {
          const imports = importMatch[1].split(',').map((s) => s.trim())
          // updateTodo should NOT be in the imports (it should be accessed via useUpdateTodo hook)
          expect(imports).not.toContain('updateTodo')
        }
        return true
      }),
      { numRuns: 1 },
    )
  })

  /**
   * Bug 2a: TodoSummary interface should include a `status` field.
   *
   * On unfixed code, TodoSummary only has id, code, title — no status field.
   * This means the frontend cannot display status information for related tasks.
   */
  it('TodoSummary interface should include status field', () => {
    const typesPath = path.resolve(__dirname, '../../src/types/index.ts')
    const source = fs.readFileSync(typesPath, 'utf-8')

    fc.assert(
      fc.property(
        fc.constantFrom('open', 'in_progress', 'completed'),
        (_statusValue) => {
          // Extract the TodoSummary interface definition
          const interfaceMatch = source.match(/export interface TodoSummary\s*\{([^}]*)\}/)
          expect(interfaceMatch).not.toBeNull()
          const interfaceBody = interfaceMatch![1]

          // The interface should contain a status field
          const hasStatusField = /status\s*:\s*Status/.test(interfaceBody) ||
            /status\s*:\s*['"]open['"]/.test(interfaceBody) ||
            /status\s*:/.test(interfaceBody)

          expect(hasStatusField).toBe(true)

          return true
        },
      ),
      { numRuns: 3 },
    )
  })

  /**
   * Bug 2b: SummaryLink component should render a Tag element with status className.
   *
   * On unfixed code, SummaryLink only renders a plain text link (<a> tag) without any
   * status Tag component. The expected behavior is to render a Tag with className
   * matching `detail-panel-tag-status-*`.
   */
  it('SummaryLink component should render a Tag with status className', () => {
    const detailContentPath = path.resolve(__dirname, '../../src/components/TodoDetailContent.tsx')
    const source = fs.readFileSync(detailContentPath, 'utf-8')

    fc.assert(
      fc.property(
        fc.constantFrom('open', 'in_progress', 'completed'),
        (_status) => {
          // Extract the SummaryLink component definition
          const summaryLinkMatch = source.match(/const SummaryLink[\s\S]*?(?=\n\s*return \()/m)
          expect(summaryLinkMatch).not.toBeNull()
          const summaryLinkBody = summaryLinkMatch![0]

          // SummaryLink should render a Tag component
          const rendersTag = summaryLinkBody.includes('<Tag') || summaryLinkBody.includes('Tag ')

          // SummaryLink should use the status-based className pattern
          const hasStatusClassName = summaryLinkBody.includes('detail-panel-tag-status-') ||
            summaryLinkBody.includes('`detail-panel-tag-status-${')

          expect(rendersTag).toBe(true)
          expect(hasStatusClassName).toBe(true)

          return true
        },
      ),
      { numRuns: 3 },
    )
  })

  /**
   * Bug 2c: Backend TodoSummaryDTO should include a status field in its JSON serialization.
   *
   * On unfixed code, TodoSummaryDTO only has ID, Code, Title fields — no Status.
   * This means the API response for todo details won't include status in relation summaries.
   */
  it('Backend TodoSummaryDTO should include status field', () => {
    const dtoPath = path.resolve(__dirname, '../../../backend/internal/handler/dto.go')
    const source = fs.readFileSync(dtoPath, 'utf-8')

    fc.assert(
      fc.property(fc.constant(source), (src) => {
        // Extract the TodoSummaryDTO struct definition
        const structMatch = src.match(/type TodoSummaryDTO struct\s*\{([^}]*)\}/)
        expect(structMatch).not.toBeNull()
        const structBody = structMatch![1]

        // The struct should contain a Status field with json:"status" tag
        const hasStatusField = structBody.includes('Status') && structBody.includes('"status"')

        expect(hasStatusField).toBe(true)

        return true
      }),
      { numRuns: 1 },
    )
  })
})
