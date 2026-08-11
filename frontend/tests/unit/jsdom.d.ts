// Minimal ambient types for the jsdom dependency used by the unit-test setup.
// jsdom ships no bundled declarations and no @types/jsdom is installed, so the
// setup only needs the JSDOM constructor and its window surface.
declare module 'jsdom' {
  export class JSDOM {
    constructor(html: string, options?: Record<string, unknown>);
    window: Window & typeof globalThis;
  }
}
