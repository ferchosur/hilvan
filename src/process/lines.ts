/**
 * SR-009: incremental line splitter with hard limits, for JSONL streams from agent CLIs.
 * Feeds complete lines to the callback; never buffers more than `maxLine` bytes of a partial line.
 */
export class LineLimitError extends Error {
  override name = 'LineLimitError';
  constructor(readonly reason: 'line' | 'total') {
    super(
      reason === 'line' ? 'line exceeds the maximum length' : 'stream exceeds the maximum size',
    );
  }
}

export interface LineSplitterOptions {
  maxLine?: number;
  maxTotal?: number;
}

export class LineSplitter {
  private partial = '';
  private total = 0;
  private readonly maxLine: number;
  private readonly maxTotal: number;

  constructor(
    private readonly onLine: (line: string) => void,
    opts: LineSplitterOptions = {},
  ) {
    this.maxLine = opts.maxLine ?? 1024 * 1024;
    this.maxTotal = opts.maxTotal ?? 256 * 1024 * 1024;
  }

  push(chunk: string): void {
    this.total += chunk.length;
    if (this.total > this.maxTotal) throw new LineLimitError('total');
    let start = 0;
    for (;;) {
      const nl = chunk.indexOf('\n', start);
      if (nl === -1) break;
      const line = this.partial + chunk.slice(start, nl);
      this.partial = '';
      if (line.length > this.maxLine) throw new LineLimitError('line');
      this.emit(line);
      start = nl + 1;
    }
    this.partial += chunk.slice(start);
    if (this.partial.length > this.maxLine) throw new LineLimitError('line');
  }

  /** Flushes a trailing line without newline. */
  end(): void {
    if (this.partial) {
      const line = this.partial;
      this.partial = '';
      this.emit(line);
    }
  }

  private emit(line: string): void {
    const l = line.endsWith('\r') ? line.slice(0, -1) : line;
    if (l.trim()) this.onLine(l);
  }
}
