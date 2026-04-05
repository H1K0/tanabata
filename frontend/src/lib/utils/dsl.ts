/**
 * Filter DSL utilities.
 *
 * Token format (comma-separated inside braces):
 *   t=<uuid>        — has tag
 *   m=<mime>        — exact MIME
 *   m~<pattern>     — MIME LIKE pattern
 *   (  )  &  |  !  — grouping / boolean operators
 *
 * Example: {t=uuid1,&,!,t=uuid2}  → has tag1 AND NOT tag2
 */

/** Build the filter query string value from an ordered token list. */
export function buildDslFilter(tokens: string[]): string | null {
	if (tokens.length === 0) return null;
	return '{' + tokens.join(',') + '}';
}

/** Parse the filter query string value back into a token list. */
export function parseDslFilter(value: string | null): string[] {
	if (!value) return [];
	const inner = value.replace(/^\{/, '').replace(/\}$/, '').trim();
	if (!inner) return [];
	return inner.split(',');
}

/** Return a human-readable label for a single DSL token (for display). */
export function tokenLabel(token: string, tagNames: Map<string, string>): string {
	if (token === '&') return 'AND';
	if (token === '|') return 'OR';
	if (token === '!') return 'NOT';
	if (token === '(') return '(';
	if (token === ')') return ')';
	if (token.startsWith('t=')) {
		const id = token.slice(2);
		return tagNames.get(id) ?? token;
	}
	return token;
}