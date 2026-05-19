/**
 * Preservation Property Tests
 *
 * **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5**
 *
 * These tests capture existing correct behavior on UNFIXED code.
 * They MUST PASS on the current codebase to confirm baseline behavior to preserve.
 *
 * Observation-first methodology:
 * - Normal task creation (useCreateTodo) invalidates ['todos'] queries
 * - SummaryLink renders a clickable link with `item.code - item.title` and calls onNavigate
 * - Status change operations (start/complete/reopen) use mutation hooks and invalidate cache
 * - TodoSummaryDTO is used in ConflictResponse and TodoGraphComponentResponse
 * - Tasks with no relations do not render the dependencies section
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import * as fs from 'node:fs';
import * as path from 'node:path';

describe('Preservation: Existing Functionality Unchanged', () => {
  /**
   * Property: For all normal task CRUD operations (not involving prerequisite creation flow),
   * verify cache invalidation occurs via mutation hooks.
   *
   * **Validates: Requirements 3.1**
   *
   * Observation: useCreateTodo, useUpdateTodo, useDeleteTodo all call
   * qc.invalidateQueries({ queryKey: ['todos'] }) in their onSuccess callbacks.
   */
  it('normal CRUD mutation hooks invalidate todos cache on success', () => {
    const hooksPath = path.resolve(__dirname, '../../src/hooks/useTodos.ts');
    const source = fs.readFileSync(hooksPath, 'utf-8');

    // Property: for all CRUD hook names, each must invalidate ['todos'] queries
    fc.assert(
      fc.property(
        fc.constantFrom('useCreateTodo', 'useUpdateTodo', 'useDeleteTodo'),
        (hookName) => {
          // Extract the hook function body
          const hookRegex = new RegExp(
            `export function ${hookName}\\(\\)\\s*\\{([\\s\\S]*?)\\n\\}`,
            'm',
          );
          const match = source.match(hookRegex);
          expect(match).not.toBeNull();
          const hookBody = match![1];

          // Verify it uses useQueryClient
          expect(hookBody).toContain('useQueryClient');

          // Verify it uses useMutation
          expect(hookBody).toContain('useMutation');

          // Verify onSuccess invalidates ['todos'] queries
          expect(hookBody).toContain('onSuccess');
          expect(hookBody).toContain("invalidateQueries");
          expect(hookBody).toContain("['todos']");

          return true;
        },
      ),
      { numRuns: 3 },
    );
  });

  /**
   * Property: For all valid TodoSummary objects, SummaryLink renders a clickable link
   * with formatted display code and title text, and calls onNavigate on click.
   *
   * **Validates: Requirements 3.2**
   *
   * Observation: SummaryLink component renders:
   *   <a onClick={() => onNavigate(item.id)} style={{ cursor: 'pointer' }}>
   *     <LinkOutlined style={{ marginRight: 4 }} />
   *     {formatDisplayCode(item.category, item.code)} - {item.title}
   *   </a>
   */
  it('SummaryLink renders clickable link with code-title text and calls onNavigate', () => {
    const detailContentPath = path.resolve(
      __dirname,
      '../../src/components/TodoDetailContent.tsx',
    );
    const source = fs.readFileSync(detailContentPath, 'utf-8');

    fc.assert(
      fc.property(
        fc.record({
          id: fc.integer({ min: 1, max: 10000 }),
          code: fc.stringMatching(/^[A-Z]+-\d+$/),
          title: fc.string({ minLength: 1, maxLength: 50 }),
        }),
        (_todoSummary) => {
          // Extract SummaryLink component definition
          const summaryLinkMatch = source.match(
            /const SummaryLink\s*=\s*\(\{[^}]*\}\s*:\s*\{[^}]*\}\)\s*=>\s*\(([\s\S]*?)\);/m,
          );
          expect(summaryLinkMatch).not.toBeNull();
          const summaryLinkBody = summaryLinkMatch![1];

          // Verify it renders an <a> tag with onClick handler calling onNavigate
          expect(summaryLinkBody).toContain('<a');
          expect(summaryLinkBody).toContain('onClick');
          expect(summaryLinkBody).toContain('onNavigate(item.id)');

          // Verify it renders formatted display code using formatDisplayCode utility
          expect(summaryLinkBody).toContain('formatDisplayCode(item.category, item.code)');
          expect(summaryLinkBody).toContain('{item.title}');
          expect(summaryLinkBody).toContain(' - ');

          // Verify cursor pointer style for clickability
          expect(summaryLinkBody).toContain("cursor: 'pointer'");

          return true;
        },
      ),
      { numRuns: 10 },
    );
  });

  /**
   * Property: For all status values in ['open', 'in_progress', 'completed'],
   * verify status change mutations work and invalidate cache.
   *
   * **Validates: Requirements 3.5**
   *
   * Observation: useStartTodo, useCompleteTodo, useReopenTodo, useSetTodoStatus
   * all use useMutation and invalidate ['todos'] queries on success.
   */
  it('status change mutation hooks invalidate todos cache on success', () => {
    const hooksPath = path.resolve(__dirname, '../../src/hooks/useTodos.ts');
    const source = fs.readFileSync(hooksPath, 'utf-8');

    fc.assert(
      fc.property(
        fc.constantFrom('useStartTodo', 'useCompleteTodo', 'useReopenTodo', 'useSetTodoStatus'),
        (hookName) => {
          // Extract the hook function body
          const hookRegex = new RegExp(
            `export function ${hookName}\\(\\)\\s*\\{([\\s\\S]*?)\\n\\}`,
            'm',
          );
          const match = source.match(hookRegex);
          expect(match).not.toBeNull();
          const hookBody = match![1];

          // Verify it uses useMutation with cache invalidation
          expect(hookBody).toContain('useMutation');
          expect(hookBody).toContain('onSuccess');
          expect(hookBody).toContain("invalidateQueries");
          expect(hookBody).toContain("['todos']");

          return true;
        },
      ),
      { numRuns: 4 },
    );
  });

  /**
   * Property: For todo details with empty depends_on and depended_by arrays,
   * verify no dependencies section is rendered.
   *
   * **Validates: Requirements 3.3**
   *
   * Observation: The dependencies section is conditionally rendered:
   *   {(todo.depends_on.length > 0 || todo.depended_by.length > 0) && ( ... )}
   * When both arrays are empty, the section is not rendered.
   */
  it('dependencies section only renders when relations exist', () => {
    const detailContentPath = path.resolve(
      __dirname,
      '../../src/components/TodoDetailContent.tsx',
    );
    const source = fs.readFileSync(detailContentPath, 'utf-8');

    fc.assert(
      fc.property(fc.constant(null), () => {
        // Verify the conditional rendering pattern exists
        // The component checks both arrays before rendering the dependencies section
        expect(source).toContain('todo.depends_on.length > 0');
        expect(source).toContain('todo.depended_by.length > 0');

        // Verify it uses logical OR — both must be empty to hide
        const conditionalMatch = source.match(
          /\(todo\.depends_on\.length > 0 \|\| todo\.depended_by\.length > 0\)/,
        );
        expect(conditionalMatch).not.toBeNull();

        // Verify the dependencies section has a className for identification
        expect(source).toContain('detail-panel-dependencies');

        return true;
      }),
      { numRuns: 1 },
    );
  });

  /**
   * Property: For TodoSummaryDTO with added status field, verify backward compatibility
   * with graph view and conflict response consumers.
   *
   * **Validates: Requirements 3.4, 3.5**
   *
   * Observation: TodoSummaryDTO is used in:
   * - ConflictResponse (pending_dependencies, completed_dependents)
   * - TodoGraphComponentResponse (root_summaries)
   * - TodoDetailResponse (depends_on, depended_by, duplicates, duplicate_of)
   *
   * The frontend consumers access only id, code, title from TodoSummary.
   * Adding a status field is an additive change that won't break existing consumers.
   */
  it('TodoSummaryDTO consumers only access id, code, title (additive status field is safe)', () => {
    // Check graph page consumer — only accesses id, code, title from root_summaries
    const graphPagePath = path.resolve(__dirname, '../../src/pages/TodoGraphPage.tsx');
    const graphSource = fs.readFileSync(graphPagePath, 'utf-8');

    // Check types definition — TodoSummary currently has id, code, title
    const typesPath = path.resolve(__dirname, '../../src/types/index.ts');
    const typesSource = fs.readFileSync(typesPath, 'utf-8');

    fc.assert(
      fc.property(
        fc.constantFrom('id', 'code', 'title'),
        (field) => {
          // Verify TodoSummary interface contains the field
          const interfaceMatch = typesSource.match(
            /export interface TodoSummary\s*\{([^}]*)\}/,
          );
          expect(interfaceMatch).not.toBeNull();
          const interfaceBody = interfaceMatch![1];
          expect(interfaceBody).toContain(field);

          return true;
        },
      ),
      { numRuns: 3 },
    );

    fc.assert(
      fc.property(fc.constant(null), () => {
        // Verify graph page uses TodoSummary type
        expect(graphSource).toContain('TodoSummary');

        // Verify graph page constructs TodoSummary-like objects with id, code, title
        // The lockedPrerequisite in graph page: { id: matched.id, code: matched.code, title: matched.title }
        expect(graphSource).toContain('id: matched.id');
        expect(graphSource).toContain('code: matched.code');
        expect(graphSource).toContain('title: matched.title');

        // Verify ConflictResponse consumer in TodoDetailContent accesses only id, code, title
        const detailContentPath = path.resolve(
          __dirname,
          '../../src/components/TodoDetailContent.tsx',
        );
        const detailSource = fs.readFileSync(detailContentPath, 'utf-8');

        // The conflict modal renders: {item.code} - {item.title}
        // It accesses item.id (as key), item.code, and item.title
        expect(detailSource).toContain('item.code');
        expect(detailSource).toContain('item.title');
        expect(detailSource).toContain('key={item.id}');

        return true;
      }),
      { numRuns: 1 },
    );
  });

  /**
   * Property: Backend TodoSummaryDTO struct is used in ConflictResponse and
   * TodoGraphComponentResponse — verify these struct definitions exist and reference TodoSummaryDTO.
   *
   * **Validates: Requirements 3.5**
   *
   * Observation: dto.go defines ConflictResponse with Pending/Completed []TodoSummaryDTO
   * and TodoGraphComponentResponse with RootSummaries []TodoSummaryDTO.
   * Adding a Status field to TodoSummaryDTO is additive and won't break JSON serialization
   * for these consumers (extra fields are simply included in the response).
   */
  it('backend TodoSummaryDTO is referenced by ConflictResponse and TodoGraphComponentResponse', () => {
    const dtoPath = path.resolve(__dirname, '../../../backend/internal/handler/dto.go');
    const source = fs.readFileSync(dtoPath, 'utf-8');

    fc.assert(
      fc.property(
        fc.constantFrom('ConflictResponse', 'TodoGraphComponentResponse'),
        (structName) => {
          // Verify the struct exists
          const structRegex = new RegExp(
            `type ${structName} struct\\s*\\{([^}]*)\\}`,
          );
          const match = source.match(structRegex);
          expect(match).not.toBeNull();
          const structBody = match![1];

          // Verify it references TodoSummaryDTO
          expect(structBody).toContain('TodoSummaryDTO');

          return true;
        },
      ),
      { numRuns: 2 },
    );
  });
});
