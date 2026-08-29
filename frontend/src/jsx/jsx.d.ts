// Global JSX namespace for the project-owned classic-mode JSX runtime.
//
// With tsconfig classic mode (jsx: "react", jsxFactory: "h",
// jsxFragmentFactory: "Fragment"), TypeScript resolves the JSX namespace from
// this ambient declaration. Element is a real DOM Node, and IntrinsicElements
// validates tag names plus generated class tokens while keeping other
// attributes permissive. SVG and namespaced elements are not supported.

import type { ClassName } from "./classes.ts";
import type { ClassNames } from "./jsx-runtime.ts";

declare global {
  namespace JSX {
    type Element = Node;
    interface ElementChildrenAttribute { children: {}; }
    interface IntrinsicAttributes {}
    type IntrinsicElements = {
      [K in keyof HTMLElementTagNameMap]: { [attr: string]: unknown; className?: ClassName | ClassNames };
    };
  }
}

export {};
