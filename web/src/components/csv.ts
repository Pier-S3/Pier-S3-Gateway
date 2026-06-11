// Minimal RFC 4180-style CSV/TSV parser for previews.
//
// SECURITY/ROBUSTNESS: the input is untrusted object content. The parser only
// splits strings (no eval, no DOM); the caller renders cells as React text
// children, which escapes them. Output size is capped so a huge or hostile
// file cannot hang the tab or balloon memory: parsing stops after
// MAX_CSV_ROWS rows and each row is cut at MAX_CSV_COLS columns.

export const MAX_CSV_ROWS = 500;
export const MAX_CSV_COLS = 50;

export interface CsvTable {
  /** First row of the file, used as the table header. */
  header: string[];
  rows: string[][];
  /** True when the file had more rows/columns than the preview caps. */
  truncated: boolean;
}

/** Pick the delimiter by file extension; .tsv is tab-separated. */
export function delimiterFor(key: string): ',' | '\t' {
  return /\.tsv$/i.test(key.replace(/\/+$/, '')) ? '\t' : ',';
}

/**
 * Parse delimiter-separated text into a capped table. Handles quoted fields
 * ("a,b"), doubled-quote escapes ("") and CR/LF/CRLF row breaks.
 */
export function parseCsv(text: string, delimiter: ',' | '\t' = ','): CsvTable {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = '';
  let inQuotes = false;
  let truncated = false;

  const pushField = () => {
    if (row.length < MAX_CSV_COLS) {
      row.push(field);
    } else {
      truncated = true;
    }
    field = '';
  };

  const pushRow = (): boolean => {
    pushField();
    rows.push(row);
    row = [];
    if (rows.length > MAX_CSV_ROWS) {
      rows.length = MAX_CSV_ROWS;
      truncated = true;
      return false;
    }
    return true;
  };

  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"' && field === '') {
      inQuotes = true;
    } else if (ch === delimiter) {
      pushField();
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && text[i + 1] === '\n') i++;
      if (!pushRow()) return finish(rows, truncated);
    } else {
      field += ch;
    }
  }
  // Flush the final field/row unless the file ended exactly on a row break.
  if (field !== '' || row.length > 0) pushRow();

  return finish(rows, truncated);
}

function finish(rows: string[][], truncated: boolean): CsvTable {
  const [header = [], ...body] = rows;
  return { header, rows: body, truncated };
}
