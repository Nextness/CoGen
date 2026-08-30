// Compile-time guards for the project-owned JSX type surface. This file is
// included by tsc and intentionally is not executed by the Node unit runner.

import { Fragment, h } from "../../src/jsx/jsx-runtime.ts";
import type { ComponentProps } from "../../src/jsx/jsx-runtime.ts";

/** Test component used to verify component prop inference. */
function Greeting(props: { name: string; children?: JSX.Node }): JSX.Element {
  return <span>{props.name}{props.children}</span>;
}

/** Props inferred from the test component. */
const greetingProps = { name: "Ada" } satisfies ComponentProps<typeof Greeting>;

void greetingProps;
void <Greeting name="Ada">Lovelace</Greeting>;
void <div className="ui" data-row-key={1} aria-hidden={false} />;
void <input type="search" readOnly />;
void <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M0 0h16v16z" /></svg>;
void <Fragment><span>one</span><span>two</span></Fragment>;

// @ts-expect-error Plain strings outside the generated registry are rejected.
void <div className="not-a-class" />;

// @ts-expect-error Compound class strings must be built through cx.
void <button className="ui button">Action</button>;

// @ts-expect-error Boolean attributes reject string lookalikes.
void <button disabled="true">Action</button>;

// @ts-expect-error Void elements do not accept children.
void <img src="/fixture.png" alt="Fixture">child</img>;

// @ts-expect-error Unknown intrinsic elements are rejected.
void <unknownTag />;

// @ts-expect-error Required component props are enforced.
void <Greeting />;
