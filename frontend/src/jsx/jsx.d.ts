// Global JSX namespace for the project-owned classic-mode JSX runtime.
//
// With tsconfig classic mode (jsx: "react", jsxFactory: "h",
// jsxFragmentFactory: "Fragment"), TypeScript resolves the JSX namespace from
// this ambient declaration. Elements are real DOM Nodes, intrinsic attributes
// follow the browser's HTML and SVG vocabulary, and className values are tied
// to the generated stylesheet registry.

import type { ClassName } from "./classes.ts";
import type { ClassNames } from "./jsx-runtime.ts";

declare global {
  namespace JSX {
    type Element = globalThis.Node;
    type Node = globalThis.Node | string | number | boolean | null | undefined | Node[];

    interface ElementChildrenAttribute {
      children: Node;
    }

    interface IntrinsicAttributes {}

    interface RWAriaAttributes {
      [attribute: `aria-${string}`]: string | boolean | undefined;
    }

    interface RWDataAttributes {
      [attribute: `data-${string}`]: string | number | boolean | undefined;
    }

    interface RWGlobalAttributes extends RWAriaAttributes, RWDataAttributes {
      accessKey?: string;
      className?: ClassName | ClassNames;
      contentEditable?: boolean | "true" | "false" | "plaintext-only";
      dir?: "auto" | "ltr" | "rtl";
      draggable?: boolean;
      hidden?: boolean;
      id?: string;
      inert?: boolean;
      lang?: string;
      role?: string;
      slot?: string;
      spellCheck?: boolean;
      style?: Partial<CSSStyleDeclaration> | string;
      tabIndex?: number;
      title?: string;
      translate?: "yes" | "no";
      onChange?: (event: Event) => void;
      onClick?: (event: MouseEvent) => void;
      onDoubleClick?: (event: MouseEvent) => void;
      onInput?: (event: InputEvent) => void;
      onKeyDown?: (event: KeyboardEvent) => void;
      onKeyUp?: (event: KeyboardEvent) => void;
      onSubmit?: (event: SubmitEvent) => void;
    }

    interface RWHTMLAttributes extends RWGlobalAttributes {
      children?: Node;
    }

    interface RWAnchorAttributes extends RWHTMLAttributes {
      download?: boolean | string;
      href?: string;
      rel?: string;
      target?: "_blank" | "_parent" | "_self" | "_top" | string;
    }

    interface RWButtonAttributes extends RWHTMLAttributes {
      disabled?: boolean;
      form?: string;
      name?: string;
      type?: "button" | "reset" | "submit";
      value?: string | number;
    }

    type RWInputType = "button" | "checkbox" | "color" | "date" | "datetime-local" | "email" | "file" | "hidden" | "image" | "month" | "number" | "password" | "radio" | "range" | "reset" | "search" | "submit" | "tel" | "text" | "time" | "url" | "week";

    interface RWInputAttributes extends RWGlobalAttributes {
      accept?: string;
      autoComplete?: string;
      checked?: boolean;
      disabled?: boolean;
      form?: string;
      list?: string;
      max?: string | number;
      maxLength?: number;
      min?: string | number;
      minLength?: number;
      multiple?: boolean;
      name?: string;
      pattern?: string;
      placeholder?: string;
      readOnly?: boolean;
      required?: boolean;
      size?: number;
      step?: string | number;
      type?: RWInputType;
      value?: string | number;
    }

    interface RWSelectAttributes extends RWHTMLAttributes {
      disabled?: boolean;
      form?: string;
      multiple?: boolean;
      name?: string;
      required?: boolean;
      size?: number;
      value?: string | number;
    }

    interface RWOptionAttributes extends RWHTMLAttributes {
      disabled?: boolean;
      label?: string;
      selected?: boolean;
      value?: string | number;
    }

    interface RWLabelAttributes extends RWHTMLAttributes {
      htmlFor?: string;
    }

    interface RWFormAttributes extends RWHTMLAttributes {
      action?: string;
      autoComplete?: "off" | "on";
      method?: "dialog" | "get" | "post";
      name?: string;
      noValidate?: boolean;
    }

    interface RWCellAttributes extends RWHTMLAttributes {
      colSpan?: number;
      rowSpan?: number;
      scope?: "col" | "colgroup" | "row" | "rowgroup";
    }

    interface RWImageAttributes extends RWGlobalAttributes {
      alt: string;
      height?: number | string;
      loading?: "eager" | "lazy";
      src: string;
      width?: number | string;
    }

    interface RWDetailsAttributes extends RWHTMLAttributes {
      open?: boolean;
    }

    interface RWTextareaAttributes extends RWHTMLAttributes {
      cols?: number;
      disabled?: boolean;
      form?: string;
      maxLength?: number;
      minLength?: number;
      name?: string;
      placeholder?: string;
      readOnly?: boolean;
      required?: boolean;
      rows?: number;
      value?: string;
    }

    interface RWCanvasAttributes extends RWHTMLAttributes {
      height?: number;
      width?: number;
    }

    interface RWTimeAttributes extends RWHTMLAttributes {
      dateTime?: string;
    }

    interface RWDialogAttributes extends RWHTMLAttributes {
      open?: boolean;
    }

    interface RWFieldsetAttributes extends RWHTMLAttributes {
      disabled?: boolean;
      form?: string;
      name?: string;
    }

    interface RWOrderedListAttributes extends RWHTMLAttributes {
      reversed?: boolean;
      start?: number;
      type?: "1" | "A" | "a" | "I" | "i";
    }

    interface RWListItemAttributes extends RWHTMLAttributes {
      value?: number;
    }

    interface RWVoidAttributes extends RWGlobalAttributes {}

    type RWHTMLElementAttributes<K extends keyof HTMLElementTagNameMap> =
      K extends "a" ? RWAnchorAttributes :
      K extends "button" ? RWButtonAttributes :
      K extends "input" ? RWInputAttributes :
      K extends "select" ? RWSelectAttributes :
      K extends "option" ? RWOptionAttributes :
      K extends "label" ? RWLabelAttributes :
      K extends "form" ? RWFormAttributes :
      K extends "td" | "th" ? RWCellAttributes :
      K extends "img" ? RWImageAttributes :
      K extends "details" ? RWDetailsAttributes :
      K extends "textarea" ? RWTextareaAttributes :
      K extends "canvas" ? RWCanvasAttributes :
      K extends "time" ? RWTimeAttributes :
      K extends "dialog" ? RWDialogAttributes :
      K extends "fieldset" ? RWFieldsetAttributes :
      K extends "ol" ? RWOrderedListAttributes :
      K extends "li" ? RWListItemAttributes :
      K extends "area" | "base" | "br" | "col" | "embed" | "hr" | "link" | "meta" | "param" | "source" | "track" | "wbr" ? RWVoidAttributes :
      RWHTMLAttributes;

    interface RWSVGAttributes extends RWAriaAttributes, RWDataAttributes {
      children?: Node;
      className?: ClassName | ClassNames;
      cx?: number | string;
      cy?: number | string;
      d?: string;
      fill?: string;
      height?: number | string;
      id?: string;
      r?: number | string;
      role?: string;
      stroke?: string;
      strokeWidth?: number | string;
      style?: Partial<CSSStyleDeclaration> | string;
      tabIndex?: number;
      transform?: string;
      viewBox?: string;
      width?: number | string;
      x?: number | string;
      y?: number | string;
    }

    type SVGIntrinsicElements = {
      [K in Exclude<keyof SVGElementTagNameMap, keyof HTMLElementTagNameMap>]: RWSVGAttributes;
    };

    type IntrinsicElements = {
      [K in keyof HTMLElementTagNameMap]: RWHTMLElementAttributes<K>;
    } & SVGIntrinsicElements;
  }
}

export {};
