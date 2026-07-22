import type { TFunction } from "i18next";
import type { Category, Priority, Status } from "../types";

/** Shared enum → option/label/color mappings (used by forms, list, filters). */

export const PRIORITY_COLORS: Record<Priority, string> = {
  p0: "var(--p0)",
  p1: "var(--p1)",
  p2: "var(--p2)",
  p3: "var(--p3)",
};

export const CATEGORY_COLORS: Record<Category, string> = {
  bug: "var(--cat-bug)",
  feature: "var(--cat-feature)",
  task: "var(--cat-task)",
};

export const CATEGORIES: Category[] = ["bug", "feature", "task"];
export const PRIORITIES: Priority[] = ["p0", "p1", "p2", "p3"];
export const STATUSES: Status[] = ["open", "in_progress", "completed", "duplicate"];

export function categoryOptions(t: TFunction) {
  return CATEGORIES.map((c) => ({
    value: c as string,
    label: t(`category.${c}`),
    color: CATEGORY_COLORS[c],
  }));
}

export function priorityOptions(t: TFunction) {
  return PRIORITIES.map((p) => ({
    value: p as string,
    label: t(`priority.${p}`),
    color: PRIORITY_COLORS[p],
  }));
}

const STATUS_KEYS: Record<Status, string> = {
  open: "status.open",
  in_progress: "status.inProgress",
  completed: "status.completed",
  duplicate: "status.duplicate",
};

export function statusLabel(t: TFunction, s: Status): string {
  return t(STATUS_KEYS[s] ?? s);
}
