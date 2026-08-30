// Unit tests for components/pdf-fullscreen.tsx — fullscreen reading mode and review drawer.
import { describe, it } from "node:test";
import assert from "node:assert/strict";

import "../setup.ts";
import { mountPDFFullscreen } from "../../../src/components/pdf-fullscreen.tsx";

/** Builds one reading workspace with a fullscreen button and review host. */
function buildWorkspace(): { workspace: HTMLElement; reviewHost: HTMLElement; button: HTMLButtonElement } {
  const workspace = document.createElement("div");
  workspace.className = "rw-reading-workspace";
  const button = document.createElement("button");
  button.dataset.pdfFullscreen = "";
  button.textContent = "Fullscreen";
  button.setAttribute("aria-pressed", "false");
  workspace.append(button);
  const reviewHost = document.createElement("div");
  reviewHost.dataset.reviewHost = "";
  workspace.append(reviewHost);
  document.body.append(workspace);
  return { workspace: workspace, reviewHost: reviewHost, button: button };
}

describe("pdf-fullscreen.tsx", function() {

  it("falls back to the expanded class when the Fullscreen API is absent and exits on toggle", function() {
    const { workspace, button } = buildWorkspace();
    const controller = mountPDFFullscreen({ workspace: workspace, reviewHost: workspace.querySelector("[data-review-host]") as HTMLElement });

    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), false);
    assert.equal(button.textContent, "Fullscreen");
    assert.equal(button.getAttribute("aria-pressed"), "false");

    button.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), true);
    assert.equal(button.textContent, "Exit fullscreen");
    assert.equal(button.getAttribute("aria-pressed"), "true");

    button.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), false);
    assert.equal(button.textContent, "Fullscreen");
    assert.equal(button.getAttribute("aria-pressed"), "false");

    controller.destroy();
    workspace.remove();
  });

  it("starts the drawer expanded and toggles it through the edge control", function() {
    const { workspace } = buildWorkspace();
    const controller = mountPDFFullscreen({ workspace: workspace, reviewHost: workspace.querySelector("[data-review-host]") as HTMLElement });

    buttonClick(workspace.querySelector("[data-pdf-fullscreen]") as HTMLButtonElement);
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), false);

    const edge = workspace.querySelector<HTMLButtonElement>("[data-drawer-edge]")!;
    assert.ok(edge);
    assert.equal(edge.getAttribute("aria-expanded"), "true");

    edge.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), true);
    assert.equal(edge.getAttribute("aria-expanded"), "false");
    assert.equal(edge.getAttribute("aria-label"), "Show review panel");

    edge.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), false);
    assert.equal(edge.getAttribute("aria-expanded"), "true");

    controller.destroy();
    workspace.remove();
  });

  it("expands a collapsed drawer on the rw-pdf-selection event", function() {
    const { workspace } = buildWorkspace();
    const controller = mountPDFFullscreen({ workspace: workspace, reviewHost: workspace.querySelector("[data-review-host]") as HTMLElement });

    buttonClick(workspace.querySelector("[data-pdf-fullscreen]") as HTMLButtonElement);
    const edge = workspace.querySelector<HTMLButtonElement>("[data-drawer-edge]")!;
    edge.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), true);

    workspace.dispatchEvent(new CustomEvent("rw-pdf-selection"));
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), false);
    assert.equal(edge.getAttribute("aria-expanded"), "true");

    controller.destroy();
    workspace.remove();
  });

  it("removes the drawer-collapsed class and edge control when exiting fullscreen", function() {
    const { workspace } = buildWorkspace();
    const controller = mountPDFFullscreen({ workspace: workspace, reviewHost: workspace.querySelector("[data-review-host]") as HTMLElement });

    buttonClick(workspace.querySelector("[data-pdf-fullscreen]") as HTMLButtonElement);
    const edge = workspace.querySelector<HTMLButtonElement>("[data-drawer-edge]")!;
    edge.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), true);

    buttonClick(workspace.querySelector("[data-pdf-fullscreen]") as HTMLButtonElement);
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), false);
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), false);
    assert.equal(workspace.querySelector("[data-drawer-edge]"), null);

    controller.destroy();
    workspace.remove();
  });

  it("closes the fallback expansion on Escape", function() {
    const { workspace } = buildWorkspace();
    const controller = mountPDFFullscreen({ workspace: workspace, reviewHost: workspace.querySelector("[data-review-host]") as HTMLElement });

    buttonClick(workspace.querySelector("[data-pdf-fullscreen]") as HTMLButtonElement);
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), true);

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), false);

    controller.destroy();
    workspace.remove();
  });

  it("destroy removes listeners, the fallback class, and the edge control", function() {
    const { workspace, button } = buildWorkspace();
    const controller = mountPDFFullscreen({ workspace: workspace, reviewHost: workspace.querySelector("[data-review-host]") as HTMLElement });

    buttonClick(button);
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), true);
    assert.ok(workspace.querySelector("[data-drawer-edge]"));

    controller.destroy();
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), false);
    assert.equal(workspace.classList.contains("rw-reading-workspace--drawer-collapsed"), false);
    assert.equal(workspace.querySelector("[data-drawer-edge]"), null);

    button.click();
    assert.equal(workspace.classList.contains("rw-reading-workspace--expanded"), false);
    assert.equal(button.textContent, "Fullscreen");

    workspace.remove();
  });
});

/** Clicks one button synchronously. */
function buttonClick(button: HTMLButtonElement): void {
  button.click();
}