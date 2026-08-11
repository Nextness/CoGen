// Shared pagination rendering for server-backed and in-memory result sets.
import { esc } from "../state.tsx";
import { h, renderToString } from "../jsx/jsx-runtime.ts";

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

/** Renders accessible pagination markup for server-backed or in-memory results. */
export function Pagination(props: { result: any; options?: PaginationOptions }): JSX.Element {
  const options = props.options || {};
  const result = props.result;

  const current = Math.max(1, Number(result?.page || options.page) || 1);
  const totalRows = Math.max(0, Number(result?.total_rows) || 0);
  const perPage = Math.max(1, Number(result?.per_page || options.perPage) || 50);
  const totalPages = Math.max(1, Number(result?.total_pages) || Math.ceil(totalRows / perPage) || 1);
  const safeCurrent = Math.min(current, totalPages);
  const start = totalRows === 0 ? 0 : (safeCurrent - 1) * perPage + 1;
  const end = totalRows === 0 ? 0 : Math.min(totalRows, safeCurrent * perPage);
  const itemLabel = options.itemLabel || "records";
  const pageAttribute = options.pageAttribute || "data-page";
  const pageClass = options.pageClass || "";

  const numbered = paginationPages(safeCurrent, totalPages, options.visibleCount).map(function(page) {
    const active = page === safeCurrent ? " active" : "";
    const attrs: Record<string, unknown> = { type: "button", className: "item page-number" + active + pageClass, [pageAttribute]: page };
    if (page === safeCurrent) {
      attrs["aria-current"] = "page";
    }
    return h("button", attrs, String(page));
  });

  /** Returns one pagination navigation control. */
  function control(label: string, target: number, disabled: boolean, relation: string): JSX.Element {
    const attrs: Record<string, unknown> = { type: "button", className: "item" + pageClass, [pageAttribute]: target };
    if (relation) {
      attrs["aria-label"] = relation;
    }
    if (disabled) {
      attrs.disabled = true;
    }
    return h("button", attrs, label);
  }

  const secondary = options.secondary ? <span className="rw-pagination__secondary">{options.secondary}</span> : null;

  return (
    <nav className="ui pagination menu" aria-label="Result pages">
      <div className="rw-pagination__summary">
        <strong>{start.toLocaleString()}{"\u2013"}{end.toLocaleString()}</strong> of {totalRows.toLocaleString()} {itemLabel}
        <span>Page {safeCurrent} of {totalPages}</span>
        {secondary}
      </div>
      <div className="pagination-actions">
        {control("First", 1, safeCurrent === 1, "First page")}
        {control("Previous", Math.max(1, safeCurrent - 1), safeCurrent === 1, "Previous page")}
        <span className="pagination-pages">{numbered}</span>
        {control("Next", Math.min(totalPages, safeCurrent + 1), safeCurrent === totalPages, "Next page")}
        {control("Last", totalPages, safeCurrent === totalPages, "Last page")}
      </div>
    </nav>
  );
}

/** Returns accessible pagination markup for server-backed or in-memory results. */
export function pagination(result: any, options?: PaginationOptions): string {
  return renderToString(<Pagination result={result} options={options} />);
}