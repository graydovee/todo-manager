import type { Category } from '../types';

const CATEGORY_PREFIX: Record<Category, string> = {
  bug: 'B',
  feature: 'F',
  task: 'T',
};

export function formatDisplayCode(category: Category, code: string | null | undefined): string {
  const prefix = CATEGORY_PREFIX[category] || '?';
  if (!code || !code.trim()) {
    return prefix;
  }
  return `${prefix}-${code}`;
}
