/**
 * File metadata tree model.
 *
 * The `metadata` API field is a free-form JSON object. The editor renders it as
 * a tree of nodes: each node is either a leaf (a scalar edited as text) or a
 * branch (a nested object with its own children). Arrays and other non-object
 * values stay leaves and round-trip as JSON text.
 */

/** One entry at some level of the metadata object. */
export interface MetaNode {
	/** Local-only id, for keyed list rendering. Not persisted. */
	id: number;
	key: string;
	/** `value` holds a leaf's text; `object` nests `children`. */
	kind: 'value' | 'object';
	value: string;
	children: MetaNode[];
}

let counter = 0;
/** Monotonic id for keyed rendering; uniqueness within a list is all that matters. */
export function nextMetaId(): number {
	return counter++;
}

/** A plain object (not null, not an array). */
function isPlainObject(v: unknown): v is Record<string, unknown> {
	return !!v && typeof v === 'object' && !Array.isArray(v);
}

/** Expand a stored object into editor nodes. Non-object input yields no nodes. */
export function objectToNodes(m: unknown): MetaNode[] {
	if (!isPlainObject(m)) return [];
	return Object.entries(m).map(([key, val]) => valueToNode(key, val));
}

function valueToNode(key: string, val: unknown): MetaNode {
	if (isPlainObject(val)) {
		return { id: nextMetaId(), key, kind: 'object', value: '', children: objectToNodes(val) };
	}
	return { id: nextMetaId(), key, kind: 'value', value: valueToString(val), children: [] };
}

/** Render a leaf value for the text input. Strings pass through; everything else
 *  (numbers, booleans, arrays) shows as JSON so it survives a round-trip. */
export function valueToString(val: unknown): string {
	if (val === null || val === undefined) return '';
	if (typeof val === 'string') return val;
	return JSON.stringify(val);
}

/** Collapse the node tree back into a plain object for the PATCH body. Blank keys
 *  are dropped; a later duplicate key wins. */
export function nodesToObject(nodes: MetaNode[]): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const n of nodes) {
		const k = n.key.trim();
		if (!k) continue;
		out[k] = n.kind === 'object' ? nodesToObject(n.children) : parseValue(n.value);
	}
	return out;
}

/** Parse a leaf's text: JSON-typed values (number, boolean, array, object) keep
 *  their type; anything else stays a plain string. */
export function parseValue(value: string): unknown {
	const v = value.trim();
	if (v === '') return '';
	try {
		const parsed: unknown = JSON.parse(v);
		if (parsed !== null && typeof parsed !== 'string') return parsed;
	} catch {
		// not JSON — keep the raw string
	}
	return value;
}

/** If the text is a JSON object, return it (used when expanding a leaf into a
 *  nested group); otherwise null. */
export function parseObject(value: string): Record<string, unknown> | null {
	const v = value.trim();
	if (!v) return null;
	try {
		const parsed: unknown = JSON.parse(v);
		if (isPlainObject(parsed)) return parsed;
	} catch {
		// not JSON
	}
	return null;
}

export function newValueNode(): MetaNode {
	return { id: nextMetaId(), key: '', kind: 'value', value: '', children: [] };
}

export function newObjectNode(): MetaNode {
	return { id: nextMetaId(), key: '', kind: 'object', value: '', children: [] };
}
