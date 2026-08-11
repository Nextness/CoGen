// Global JSX namespace for the project-owned classic-mode JSX runtime.
//
// With tsconfig classic mode (jsx: "react", jsxFactory: "h",
// jsxFragmentFactory: "Fragment"), TypeScript resolves the JSX namespace from
// this ambient declaration. Element is a real DOM Node, and IntrinsicElements
// validates tag names against HTMLElementTagNameMap while keeping attributes
// permissive. SVG and namespaced elements are not supported.

declare global {
  namespace JSX {
    type Element = Node;
    interface ElementChildrenAttribute { children: {}; }
    interface IntrinsicAttributes {}
    type IntrinsicElements = {
      [K in keyof HTMLElementTagNameMap]: { [attr: string]: unknown };
    };
  }
}

export {};
