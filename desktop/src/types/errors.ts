import type { Category } from './index';

export interface RelationConflictItem {
  id: number;
  code: string;
  title: string;
  category: Category;
}

export interface TodoConflictResponse {
  pending_dependencies?: RelationConflictItem[];
  completed_dependents?: RelationConflictItem[];
}

export interface HttpErrorLike {
  response?: {
    status?: number;
    data?: TodoConflictResponse;
  };
}
