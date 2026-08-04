// Trash: trashed runs, restore history.
import { app, pageHeader, detailTable, timeline, panel, list } from '../state.js';
import { api } from '../api.js';

/** Asynchronously implements trash view for the viewer. */
export async function trashView() {
  const [data, history] = await Promise.all([
    api('/api/trash'),
    api('/api/audit', { action: 'run_restored', limit: 100 })
  ]);

  app.innerHTML = pageHeader('Inspection only', 'Trash', 'Trashed run attempts and restore history are visible here. Restore and purge are not available in this read-only viewer.')
    + '<div class="ui grid dashboard-grid">'
    + detailTable('Trashed run attempts', list(data, ['runs']))
    + panel('Restore history', 'Historical audit evidence only.', timeline(list(history, ['events'])))
    + '</div>';
}
