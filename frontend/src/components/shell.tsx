// Shell: header health status and mobile navigation toggle.
import { api } from "../api.tsx";

/** Health status dot element updated by the health check. */
export const healthStatus = document.querySelector<HTMLElement>("#health-status")!;

/** Initializes health check. */
export function initHealthCheck(): void {
  api("/api/health", {}, {
    method: "GET",
    headers: { Accept: "application/json" },
  }).then((health) => {
    const unavailable = health?.readable === false;
    if (unavailable) {
      healthStatus.textContent = "Database unavailable";
    } else {
      healthStatus.textContent = "Database healthy";
    }
    healthStatus.classList.toggle("rw-status-dot--unavailable", unavailable);
    healthStatus.classList.toggle("unavailable", unavailable);
  }).catch(() => {
    healthStatus.textContent = "Database unavailable";
    healthStatus.classList.add("rw-status-dot--unavailable");
    healthStatus.classList.add("unavailable");
  });
}

/**
 * Initialize mobile nav toggle.
 * Shows/hides the primary navigation on small screens.
 */
export function initMobileNavToggle(): void {
  const toggle = document.querySelector<HTMLElement>("#mobile-nav-toggle");
  const nav = document.querySelector<HTMLElement>(".primary-nav");
  if (!toggle || !nav) return;

  /** Handles toggle. */
  function handleToggle(): void {
    const isOpen = nav!.classList.toggle("rw-mobile-nav-open");
    document.body.classList.toggle("rw-mobile-nav-open", isOpen);
    toggle!.setAttribute("aria-expanded", String(isOpen));
  }

  toggle.addEventListener("click", handleToggle);

  // Close nav when a link is clicked (mobile)
  const navLinks = nav.querySelectorAll("a");
  navLinks.forEach((link) => {
    link.addEventListener("click", () => {
      nav!.classList.remove("rw-mobile-nav-open");
      document.body.classList.remove("rw-mobile-nav-open");
      toggle!.setAttribute("aria-expanded", "false");
    });
  });
}
