// Shared pagination rendering for server-backed and in-memory result sets.
import { h } from "../jsx/jsx-runtime.ts";

/** Returns the bounded sequence of page numbers surrounding the current page. */
export function paginationPages(currentPage: any, totalPages: any, visibleCount: any): number[] {
  const current = Math.max(1, Number(currentPage) || 1);
  const total = Math.max(1, Number(totalPages) || 1);
  const count = Math.max(3, Number(visibleCount) || 5);
  const half = Math.floor(count / 2);
  var start = Math.max(1, current - half);
  var end = Math.min(total, start + count - 1);
  start = Math.max(1, end - count + 1);

  const pages: number[] = [];
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

  const current = Math.max(1, Number(props.result?.page || options.page) || 1);
  const totalRows = Math.max(0, Number(props.result?.total_rows) || 0);
  const perPage = Math.max(1, Number(props.result?.per_page || options.perPage) || 50);
  const totalPages = Math.max(1, Number(props.result?.total_pages) || Math.ceil(totalRows / perPage) || 1);
  const safeCurrent = Math.min(current, totalPages);
  var start = 0;
  if (totalRows !== 0) start = (safeCurrent - 1) * perPage + 1;
  var end = 0;
  if (totalRows !== 0) end = Math.min(totalRows, safeCurrent * perPage);
  const itemLabel = options.itemLabel || "records";
  const pageAttribute = options.pageAttribute || "data-page";
  const pageClass = options.pageClass || "";

  const pageNumbers = paginationPages(safeCurrent, totalPages, options.visibleCount);
  const numbered = pageNumbers.map((page) => {
    var active = "";
    if (page === safeCurrent) active = " active";
    const pageNumberClass = `item page-number${active}${pageClass}`;
    const attrs: Record<string, unknown> = {
      type: "button",
      className: pageNumberClass,
      [pageAttribute]: page,
    };
    if (page === safeCurrent) attrs["aria-current"] = "page";
    return h("button", attrs, String(page));
  });

  /** Returns one pagination navigation control. */
  function control(label: string, target: number, disabled: boolean, relation: string): JSX.Element {
    const itemClass = `item${pageClass}`;
    const attrs: Record<string, unknown> = {
      type: "button",
      className: itemClass,
      [pageAttribute]: target,
    };
    if (relation) attrs["aria-label"] = relation;
    if (disabled) attrs.disabled = true;
    return h("button", attrs, label);
  }

  var secondary: JSX.Element | null = null;
  if (options.secondary) {
    secondary = <span className="rw-pagination__secondary">{options.secondary}</span>;
  }

  const firstControl = control("First", 1, safeCurrent === 1, "First page");
  const previousControl = control("Previous", Math.max(1, safeCurrent - 1), safeCurrent === 1, "Previous page");
  const nextControl = control("Next", Math.min(totalPages, safeCurrent + 1), safeCurrent === totalPages, "Next page");
  const lastControl = control("Last", totalPages, safeCurrent === totalPages, "Last page");

  return (
    <nav className="ui pagination menu" aria-label="Result pages">
      <div className="rw-pagination__summary">
        <strong>{start.toLocaleString()}{"\u2013"}{end.toLocaleString()}</strong> of {totalRows.toLocaleString()} {itemLabel}
        <span>Page {safeCurrent} of {totalPages}</span>
        {secondary}
      </div>
      <div className="pagination-actions">
        {firstControl}
        {previousControl}
        <span className="pagination-pages">{numbered}</span>
        {nextControl}
        {lastControl}
      </div>
    </nav>
  );
}
