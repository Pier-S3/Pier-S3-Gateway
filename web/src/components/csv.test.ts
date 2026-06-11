import { describe, it, expect } from 'vitest';
import { parseCsv, delimiterFor, MAX_CSV_ROWS, MAX_CSV_COLS } from './csv';

describe('delimiterFor', () => {
  it('uses tab for .tsv and comma otherwise', () => {
    expect(delimiterFor('data/report.tsv')).toBe('\t');
    expect(delimiterFor('data/report.TSV')).toBe('\t');
    expect(delimiterFor('data/report.csv')).toBe(',');
  });
});

describe('parseCsv', () => {
  it('splits header and rows', () => {
    const t = parseCsv('a,b,c\n1,2,3\n4,5,6');
    expect(t.header).toEqual(['a', 'b', 'c']);
    expect(t.rows).toEqual([['1', '2', '3'], ['4', '5', '6']]);
    expect(t.truncated).toBe(false);
  });

  it('handles quoted fields with delimiters, newlines and "" escapes', () => {
    const t = parseCsv('name,note\n"Smith, John","line1\nline2"\n"say ""hi""",x');
    expect(t.rows[0]).toEqual(['Smith, John', 'line1\nline2']);
    expect(t.rows[1]).toEqual(['say "hi"', 'x']);
  });

  it('handles CRLF row breaks and a trailing newline', () => {
    const t = parseCsv('a,b\r\n1,2\r\n');
    expect(t.header).toEqual(['a', 'b']);
    expect(t.rows).toEqual([['1', '2']]);
  });

  it('parses tab-separated content', () => {
    const t = parseCsv('a\tb\n1\t2', '\t');
    expect(t.header).toEqual(['a', 'b']);
    expect(t.rows).toEqual([['1', '2']]);
  });

  it('caps rows and reports truncation', () => {
    const lines = ['h'];
    for (let i = 0; i < MAX_CSV_ROWS + 100; i++) lines.push(String(i));
    const t = parseCsv(lines.join('\n'));
    expect(t.rows.length).toBe(MAX_CSV_ROWS - 1); // header consumed one capped row
    expect(t.truncated).toBe(true);
  });

  it('caps columns and reports truncation', () => {
    const wide = Array.from({ length: MAX_CSV_COLS + 10 }, (_, i) => `c${i}`).join(',');
    const t = parseCsv(`${wide}\n${wide}`);
    expect(t.header.length).toBe(MAX_CSV_COLS);
    expect(t.rows[0].length).toBe(MAX_CSV_COLS);
    expect(t.truncated).toBe(true);
  });

  it('returns an empty table for empty input', () => {
    const t = parseCsv('');
    expect(t.header).toEqual([]);
    expect(t.rows).toEqual([]);
  });
});
