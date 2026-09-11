import { h, Fragment, cx } from "../jsx/jsx-runtime.ts";
import type { TermMatchSummary, WireRecord } from "../api/types.ts";

/** Typed compound class names used by article row evidence. */
const classNames = {
  uiFadedText: cx("ui", "faded", "text"),
  uiLabel: cx("ui", "label"),
};

// Core columns shown in the articles table; extra fields appear in expandable rows.
export const articlesColumns = ["doi", "title", "year", "journal", "source"];
export const articlesExpandFields = [
  {
    f: "title",
    w: "full" as const,
  },
  {
    f: "authors",
    w: "full" as const,
  },
  {
    f: "journal",
    w: 10,
  },
  {
    f: "publisher",
    w: 10,
  },
  {
    f: "abstract",
    w: "full" as const,
  },
  {
    f: "term_matches",
    w: "full" as const,
    label: "Matched search terms",
    render: termMatchMarkup,
  },
  {
    f: "work_id",
    w: 4,
  },
  {
    f: "year",
    w: 4,
  },
  {
    f: "source",
    w: 4,
  },
  {
    f: "doi",
    w: 4,
  },
  {
    f: "validation_status",
    w: 4,
  },
  {
    f: "citation_count",
    w: 5,
  },
  {
    f: "reference_count",
    w: 5,
  },
  {
    f: "producer_stage",
    w: 5,
  },
  {
    f: "created_at",
    w: 5,
  },
];

/** Renders the stored search-term coverage for one article row. */
function termMatchMarkup(row: WireRecord): JSX.Element {
  if (row.term_matches === null || row.term_matches === undefined) {
    return <span className={classNames.uiFadedText}>No search terms recorded</span>;
  }
  const termMatches = row.term_matches as TermMatchSummary;
  const fields = [
    {
      key: "title",
      label: "Title",
    },
    {
      key: "abstract",
      label: "Abstract",
    },
    {
      key: "keywords",
      label: "Keywords",
    },
    {
      key: "keywords_plus",
      label: "Keywords plus",
    },
  ];
  const fieldElements: JSX.Element[] = fields.map(({ key, label }) => {
    const terms = termMatches[key] as string[] | undefined || [];
    var content: JSX.Element = <span className={classNames.uiFadedText}>No matched terms</span>;
    if (terms.length) {
      const termTags: JSX.Element[] = terms.map((term: string) => {
        return <span className={classNames.uiLabel}>{term}</span>;
      });
      content = <span className="rw-keyword-tags">{termTags}</span>;
    }
    return (
      <div className="rw-term-field">
        <span className="rw-term-field__label">{label}</span>
        {content}
      </div>
    );
  });
  const matchedTotal = termMatches.matched_total;
  const termTotal = termMatches.term_total;
  return (
    <Fragment>
      <p className={classNames.uiFadedText}>{matchedTotal} of {termTotal} search terms matched</p>
      <div className="rw-term-fields">{fieldElements}</div>
    </Fragment>
  );
}
