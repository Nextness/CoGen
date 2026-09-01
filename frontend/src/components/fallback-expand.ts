// Shared CSS-fallback expansion for fullscreen-style toggles. When the
// Fullscreen API is unavailable, an element is expanded by adding a class and
// locking document scroll. This module owns the overflow save/restore so the
// graph and PDF viewers do not duplicate it.

import { classAdd, classRemove, classHas } from "../jsx/jsx-runtime.ts";
import type { ClassName } from "../jsx/classes.ts";

/** One fallback-expansion controller for a single element. */
export interface FallbackExpandController {
  /** Returns whether the element currently carries the expanded class. */
  isExpanded(): boolean;
  /** Toggles the expanded class, locking or restoring document scroll. */
  toggle(): void;
  /** Removes the expanded class and restores document scroll. */
  close(): void;
}

/** Creates a fallback-expansion controller for one element and class token. */
export function createFallbackExpand(element: HTMLElement, expandedClass: ClassName): FallbackExpandController {
  let priorOverflow = "";
  return {
    isExpanded(): boolean {
      return classHas(element, expandedClass);
    },
    toggle(): void {
      if (classHas(element, expandedClass)) {
        classRemove(element, expandedClass);
        document.body.style.overflow = priorOverflow;
      } else {
        priorOverflow = document.body.style.overflow;
        document.body.style.overflow = "hidden";
        classAdd(element, [expandedClass]);
      }
    },
    close(): void {
      if (!classHas(element, expandedClass)) return;
      classRemove(element, expandedClass);
      document.body.style.overflow = priorOverflow;
    },
  };
}
