// Ambient type declarations for the checked-in vendored browser modules.
// The vendored bundles ship no TypeScript declarations, and the PDF.js npm
// package's declarations cover its own layout, not the vendored file path.

/** D3-force simulation with the chained API surface used by graph.ts. */
declare module '*d3-force.js' {
  /** One simulation node with its resolved identifier and display properties. */
  export interface SimulationNode {
    id: string | number;
    type?: string;
    radius: number;
    x: number;
    y: number;
    vx?: number;
    vy?: number;
  }

  /** One simulation link between two node identifiers or resolved nodes. */
  export interface SimulationLink {
    source: string | number | SimulationNode;
    target: string | number | SimulationNode;
    type?: string;
    [key: string]: unknown;
  }

  /** A named force that receives the node and link arrays. */
  export interface SimulationForce {
    /** Binds an identifier accessor to a link force. */
    id(accessor: (node: SimulationNode) => string | number): SimulationForce;
    /** Binds a distance accessor to a link force. */
    distance(accessor: (edge: SimulationLink) => number): SimulationForce;
    /** Binds a strength accessor or value to a force. */
    strength(value: number | ((node: SimulationNode | SimulationLink) => number)): SimulationForce;
    /** Binds a collision radius accessor or value to a collision force. */
    radius(value: number | ((node: SimulationNode) => number)): SimulationForce;
    /** Binds an iteration count to a collision force. */
    iterations(count: number): SimulationForce;
    /** Binds a maximum distance to a many-body force. */
    distanceMax(value: number): SimulationForce;
  }

  /** The force simulation controller used by graph.ts. */
  export interface Simulation {
    /** Registers or replaces one named force. */
    force(name: string, force?: SimulationForce): Simulation;
    /** Sets the target alpha and restarts the simulation. */
    alpha(value?: number): Simulation;
    /** Sets the alpha decay rate. */
    alphaDecay(value?: number): Simulation;
    /** Restarts the simulation timer. */
    restart(): Simulation;
    /** Stops the simulation timer. */
    stop(): Simulation;
    /** Advances the simulation by one tick and returns this simulation. */
    tick(): Simulation;
  }

  /** Creates a new force simulation over the supplied nodes. */
  export function forceSimulation(nodes?: SimulationNode[]): Simulation;

  /** Creates a named force that pulls linked nodes together. */
  export function forceLink(edges?: SimulationLink[]): SimulationForce;

  /** Creates a named force that separates all nodes. */
  export function forceManyBody(): SimulationForce;

  /** Creates a named force that pulls nodes toward a fixed center. */
  export function forceCenter(x?: number, y?: number): SimulationForce;

  /** Creates a named force that prevents node overlap. */
  export function forceCollide(): SimulationForce;
}

/** PDF.js runtime module loaded lazily from the vendored bundle. */
declare module '*pdf.min.mjs' {
  /** Any PDF.js namespace; runtime members are exercised but not statically typed. */
  const pdfjs: any;
  export = pdfjs;
}