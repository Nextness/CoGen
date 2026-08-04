// Shell: header health status and mobile navigation toggle.
import { api } from '../api.js';

export const healthStatus = document.querySelector('#health-status');

/** Initializes health check. */
export function initHealthCheck() {
  api('/api/health').then(function(health) {
    var unavailable = false;
    if (health?.readable === false) {
      unavailable = true;
    }
    if (unavailable) {
      healthStatus.textContent = 'Database unavailable';
    } else {
      healthStatus.textContent = 'Database healthy';
    }
    healthStatus.classList.toggle('rw-status-dot--unavailable', unavailable);
    healthStatus.classList.toggle('unavailable', unavailable);
  }).catch(function() {
    healthStatus.textContent = 'Database unavailable';
    healthStatus.classList.add('rw-status-dot--unavailable');
    healthStatus.classList.add('unavailable');
  });
}

/**
 * Initialize mobile nav toggle.
 * Shows/hides the primary navigation on small screens.
 */
export function initMobileNavToggle() {
  var toggle = document.querySelector('#mobile-nav-toggle');
  var nav = document.querySelector('.primary-nav');
  if (!toggle || !nav) {
    return;
  }

  /** Handles toggle. */
  function handleToggle() {
    var isOpen = nav.classList.toggle('rw-mobile-nav-open');
    document.body.classList.toggle('rw-mobile-nav-open', isOpen);
    toggle.setAttribute('aria-expanded', String(isOpen));
  }

  toggle.addEventListener('click', handleToggle);

  // Close nav when a link is clicked (mobile)
  nav.querySelectorAll('a').forEach(function(link) {
    link.addEventListener('click', function() {
      nav.classList.remove('rw-mobile-nav-open');
      document.body.classList.remove('rw-mobile-nav-open');
      toggle.setAttribute('aria-expanded', 'false');
    });
  });
}
