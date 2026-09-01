// Shell: header health status and mobile navigation toggle.
import { api } from "../api.tsx";
import type { HealthResponse } from "../api/types.ts";
import { classAdd, classHas, classRemove, classToggle } from "../jsx/jsx-runtime.ts";

/** Health status dot element updated by the health check. */
export const healthStatus = document.querySelector<HTMLElement>("#health-status")!;

/** Writes one independently reported viewer capability into the shell disclosure. */
function setCapability(id: string, available: boolean, availableLabel: string, unavailableLabel: string): void {
  const element = document.querySelector<HTMLElement>(id);
  if (!element) return;
  element.textContent = available ? availableLabel : unavailableLabel;
  classToggle(element, "rw-capability-unavailable", !available);
}

/** Marks the health status dot as unavailable. */
function setUnavailable(): void {
  healthStatus.textContent = "Database unavailable";
  classAdd(healthStatus, ["rw-status-dot--unavailable"]);
  classAdd(healthStatus, ["unavailable"]);
}

/** Initializes health check. */
export function initHealthCheck(): void {
  api<HealthResponse>("/api/health", {}, {
    method: "GET",
    headers: { Accept: "application/json" },
  }).then((health) => {
    const unavailable = health?.readable === false;
    if (unavailable) {
      setUnavailable();
    } else {
      healthStatus.textContent = "Database healthy";
      classRemove(healthStatus, "rw-status-dot--unavailable");
      classRemove(healthStatus, "unavailable");
    }
    const metadataReadable = health?.metadata_readable ?? health?.readable;
    setCapability("#metadata-capability", metadataReadable === true, "Readable", "Unavailable");
    setCapability("#review-capability", health?.review_writable === true, "Writable", "Read-only or unavailable");
    var unavailablePDFLabel = "Unavailable";
    if (health?.pdf_store_bound !== true) unavailablePDFLabel = "Not connected";
    setCapability("#pdf-capability", health?.pdf_store_bound === true && health?.pdf_store_readable === true, "Readable, read-only", unavailablePDFLabel);
  }).catch(() => {
    setUnavailable();
    setCapability("#metadata-capability", false, "Readable", "Unavailable");
    setCapability("#review-capability", false, "Writable", "Unavailable");
    setCapability("#pdf-capability", false, "Readable, read-only", "Unknown");
  });
}

/**
 * Initialize mobile nav toggle.
 * Shows/hides the primary navigation on small screens.
 */
export function initMobileNavToggle(): void {
  const toggle = document.querySelector<HTMLElement>("#mobile-nav-toggle");
  const nav = document.querySelector<HTMLElement>(".rw-primary-nav");
  if (!toggle || !nav) return;
  if (!nav.id) nav.id = "primary-navigation";
  toggle.setAttribute("aria-controls", nav.id);

  /** Closes the mobile navigation and optionally restores its opener focus. */
  function closeNavigation(restoreFocus: boolean): void {
    const wasOpen = classHas(nav!, "rw-mobile-nav-open");
    classRemove(nav!, "rw-mobile-nav-open");
    classRemove(document.body, "rw-mobile-nav-open");
    toggle!.setAttribute("aria-expanded", "false");
    if (restoreFocus && wasOpen) toggle!.focus();
  }

  /** Toggles the mobile navigation disclosure. */
  function handleToggle(): void {
    const isOpen = !classHas(nav!, "rw-mobile-nav-open");
    classToggle(nav!, "rw-mobile-nav-open", isOpen);
    classToggle(document.body, "rw-mobile-nav-open", isOpen);
    toggle!.setAttribute("aria-expanded", String(isOpen));
  }

  toggle.addEventListener("click", handleToggle);

  // Close nav when a link is clicked (mobile)
  const navLinks = nav.querySelectorAll("a");
  navLinks.forEach((link) => {
    link.addEventListener("click", () => {
      closeNavigation(false);
    });
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && classHas(nav, "rw-mobile-nav-open")) {
      event.preventDefault();
      closeNavigation(true);
    }
  });
  document.addEventListener("click", (event) => {
    if (!classHas(nav, "rw-mobile-nav-open")) return;
    const target = event.target as Node;
    if (nav.contains(target) || toggle.contains(target)) return;
    closeNavigation(false);
  });
  window.addEventListener("resize", () => {
    if (window.matchMedia("(min-width: 721px)").matches) closeNavigation(false);
  });
}
