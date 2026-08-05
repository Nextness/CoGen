// Unit tests for content-coordinate projection used by PDF anchors.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { rotateRectangles, unrotateRectangles } from '../../../../src/server/frontend/components/pdf-viewer.js';

describe('pdf-viewer.js', function() {
  it('projects displayed rectangles back through all supported rotations', function() {
    const rectangle = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };
    assert.deepEqual(unrotateRectangles([rectangle], 0)[0], rectangle);
    assert.deepEqual(unrotateRectangles([rectangle], 90)[0], { x: 0.2, y: 0.6, width: 0.4, height: 0.3 });
    assert.deepEqual(unrotateRectangles([rectangle], 180)[0], { x: 0.6, y: 0.4, width: 0.3, height: 0.4 });
    assert.deepEqual(unrotateRectangles([rectangle], 270)[0], { x: 0.4, y: 0.1, width: 0.4, height: 0.3 });
  });

  it('round-trips stored rectangles into each displayed rotation', function() {
    const rectangle = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };
    for (const rotation of [0, 90, 180, 270]) {
      assert.deepEqual(unrotateRectangles(rotateRectangles([rectangle], rotation), rotation)[0], rectangle);
    }
  });
});
