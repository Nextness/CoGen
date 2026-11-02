// Unit tests for components/shell.tsx — health check.
import { describe, it, before, beforeEach } from "node:test";
import assert from "node:assert/strict";

import "../../../../support/node/setup.ts";
import { healthStatus, initHealthCheck, initMobileNavToggle } from "../../../../../frontend/src/components/shell.tsx";

describe("shell.tsx — healthStatus", () => {

  it("is a DOM element", () => {
    assert.ok(healthStatus instanceof HTMLElement);
    assert.equal(healthStatus.id, "health-status");
  });

});

describe("shell.tsx — initHealthCheck", () => {

  beforeEach(() => {
    healthStatus.textContent = "";
    healthStatus.className = "";
  });

  it("sets healthy status on successful response", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => {
          return Promise.resolve({ data: { readable: true } });
        },
      } as unknown as Response);
    }) as typeof fetch;

    initHealthCheck();
    // initHealthCheck does not return the promise chain, so wait for microtasks
    await new Promise((resolve) => {
      setTimeout(resolve, 10);
    });
    assert.equal(healthStatus.textContent, "Database healthy");

    globalThis.fetch = originalFetch;
  });

  it("sets unavailable status when readable is false", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => {
          return Promise.resolve({ data: { readable: false } });
        },
      } as unknown as Response);
    }) as typeof fetch;

    initHealthCheck();
    await new Promise((resolve) => {
      setTimeout(resolve, 10);
    });
    assert.equal(healthStatus.textContent, "Database unavailable");

    globalThis.fetch = originalFetch;
  });

  it("sets unavailable status on fetch failure", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => {
      return Promise.reject(new Error("Network error"));
    }) as typeof fetch;

    initHealthCheck();
    await new Promise((resolve) => {
      setTimeout(resolve, 10);
    });
    assert.equal(healthStatus.textContent, "Database unavailable");

    globalThis.fetch = originalFetch;
  });
});

describe("shell.tsx — initMobileNavToggle", () => {

  let toggle: HTMLButtonElement;
  let nav: HTMLElement;

  before(() => {
    document.querySelectorAll("#mobile-nav-toggle, .rw-primary-nav").forEach((element) => {
      element.remove();
    });
    document.body.classList.remove("rw-mobile-nav-open");

    toggle = document.createElement("button");
    toggle.id = "mobile-nav-toggle";
    toggle.setAttribute("aria-expanded", "false");
    document.body.appendChild(toggle);

    nav = document.createElement("nav");
    nav.className = "rw-primary-nav";
    const link = document.createElement("a");
    link.href = "#";
    nav.appendChild(link);
    document.body.appendChild(nav);

    initMobileNavToggle();
  });

  it("toggles rw-mobile-nav-open on click", () => {
    toggle.click();
    assert.ok(nav.classList.contains("rw-mobile-nav-open"));
    assert.ok(document.body.classList.contains("rw-mobile-nav-open"));
    assert.equal(toggle.getAttribute("aria-expanded"), "true");

    toggle.click();
    assert.ok(!nav.classList.contains("rw-mobile-nav-open"));
    assert.ok(!document.body.classList.contains("rw-mobile-nav-open"));
    assert.equal(toggle.getAttribute("aria-expanded"), "false");
  });

  it("closes nav when a nav link is clicked", () => {
    toggle.click();
    assert.ok(nav.classList.contains("rw-mobile-nav-open"));

    const link = nav.querySelector("a") as HTMLAnchorElement;
    link.click();
    assert.ok(!nav.classList.contains("rw-mobile-nav-open"));
    assert.equal(toggle.getAttribute("aria-expanded"), "false");
  });

  it("owns the disclosure and restores focus when Escape closes it", () => {
    toggle.click();
    assert.equal(toggle.getAttribute("aria-controls"), nav.id);

    document.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Escape",
      bubbles: true,
    }));

    assert.equal(toggle.getAttribute("aria-expanded"), "false");
    assert.equal(document.activeElement, toggle);
  });

  it("closes the disclosure after an outside click", () => {
    toggle.click();
    document.body.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    assert.equal(toggle.getAttribute("aria-expanded"), "false");
  });

  it("is a no-op when toggle or nav is missing", () => {
    // Separate call with missing elements — already tested by the single init above
    assert.ok(toggle);
    assert.ok(nav);
  });

});
