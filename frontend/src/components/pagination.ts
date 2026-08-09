// Shared pagination rendering for server-backed and in-memory result sets.
import { esc } from '../state.ts';

/** Returns the bounded sequence of page numbers surrounding the current page. */
export function paginationPages(currentPage: any, totalPages: any, visibleCount: any): number[] {
  const current = Math.max(1, Number(currentPage) || 1);
  const total = Math.max(1, Number(totalPages) || 1);
  const count = Math.max(3, Number(visibleCount) || 5);
  const half = Math.floor(count / 2);
  var start = Math.max(1, current - half);
  var end = Math.min(total, start + count - 1);
  start = Math.max(1, end - count + 1);

  const pages = [];
  for (var page = start; page <= end; page += 1) {
    pages.push(page);
  }
  return pages;
}

/** One pagination option set. */
export interface PaginationOptions {
  page?: any;
  perPage?: any;
  itemLabel?: string;
  pageAttribute?: string;
  pageClass?: string;
  visibleCount?: any;
  secondary?: string;
}

/** Returns accessible pagination markup for server-backed or in-memory results. */
export function pagination(result: any, options?: PaginationOptions): string {
  if (!options) {
    options = {};
  }

  const current = Math.max(1, Number(result?.page || options.page) || 1);
  const totalRows = Math.max(0, Number(result?.total_rows) || 0);
  const perPage = Math.max(1, Number(result?.per_page || options.perPage) || 50);
  const totalPages = Math.max(1, Number(result?.total_pages) || Math.ceil(totalRows / perPage) || 1);
  const safeCurrent = Math.min(current, totalPages);
  const start = totalRows === 0 ? 0 : (safeCurrent - 1) * perPage + 1;
  const end = totalRows === 0 ? 0 : Math.min(totalRows, safeCurrent * perPage);
  const itemLabel = options.itemLabel || 'records';
  const pageAttribute = options.pageAttribute || 'data-page';
  const pageClass = options.pageClass || '';

  const numbered = paginationPages(safeCurrent, totalPages, options.visibleCount).map(function(page) {
    const active = page === safeCurrent ? ' active' : '';
    const currentAttribute = page === safeCurrent ? ' aria-current="page"' : '';
    return '<button type="button" class="item page-number' + active + pageClass + '" '
      + pageAttribute + '="' + page + '"' + currentAttribute + '>' + page + '</button>';
  }).join('');

  /** Returns one pagination navigation control. */
  function control(label: string, target: number, disabled: boolean, relation: string): string {
    const disabledAttribute = disabled ? ' disabled' : '';
    var relationAttribute = '';
    if (relation) {
      relationAttribute = ' aria-label="' + esc(relation) + '"';
    }
    return '<button type="button" class="item' + pageClass + '" ' + pageAttribute + '="' + target + '"'
      + relationAttribute + disabledAttribute + '>' + esc(label) + '</button>';
  }

  var secondary = '';
  if (options.secondary) {
    secondary = '<span class="rw-pagination__secondary">' + esc(options.secondary) + '</span>';
  }

  return '<nav class="ui pagination menu" aria-label="Result pages">'
    + '<div class="rw-pagination__summary">'
    + '<strong>' + start.toLocaleString() + '\u2013' + end.toLocaleString() + '</strong> of '
    + totalRows.toLocaleString() + ' ' + esc(itemLabel)
    + '<span>Page ' + safeCurrent + ' of ' + totalPages + '</span>'
    + secondary
    + '</div>'
    + '<div class="pagination-actions">'
    + control('First', 1, safeCurrent === 1, 'First page')
    + control('Previous', Math.max(1, safeCurrent - 1), safeCurrent === 1, 'Previous page')
    + '<span class="pagination-pages">' + numbered + '</span>'
    + control('Next', Math.min(totalPages, safeCurrent + 1), safeCurrent === totalPages, 'Next page')
    + control('Last', totalPages, safeCurrent === totalPages, 'Last page')
    + '</div></nav>';
}