/**
 * Dev-only Vite plugin that intercepts /api/v1/* and returns mock responses.
 * Login: any username + password "password" succeeds.
 */
import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'http';

function readBody(req: IncomingMessage): Promise<unknown> {
	return new Promise((resolve) => {
		let data = '';
		req.on('data', (chunk) => (data += chunk));
		req.on('end', () => {
			try {
				resolve(data ? JSON.parse(data) : {});
			} catch {
				resolve({});
			}
		});
	});
}

function json(res: ServerResponse, status: number, body: unknown) {
	const payload = JSON.stringify(body);
	res.writeHead(status, {
		'Content-Type': 'application/json',
		'Content-Length': Buffer.byteLength(payload),
	});
	res.end(payload);
}

function noContent(res: ServerResponse) {
	res.writeHead(204);
	res.end();
}

const MOCK_ACCESS_TOKEN = 'mock-access-token';
const MOCK_REFRESH_TOKEN = 'mock-refresh-token';

const TOKEN_PAIR = {
	access_token: MOCK_ACCESS_TOKEN,
	refresh_token: MOCK_REFRESH_TOKEN,
	expires_in: 900,
};

const ME = {
	id: 1,
	name: 'admin',
	is_admin: true,
	can_create: true,
	is_blocked: false,
};

const THUMB_COLORS = [
	'#9592B5', '#4DC7ED', '#DB6060', '#F5E872', '#7ECBA1',
	'#E08C5A', '#A67CB8', '#5A9ED4', '#C4A44A', '#6DB89E',
];

function mockThumbSvg(id: string): string {
	const color = THUMB_COLORS[id.charCodeAt(id.length - 1) % THUMB_COLORS.length];
	const label = id.slice(-4);
	return `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="160">
  <rect width="160" height="160" fill="${color}"/>
  <text x="80" y="88" text-anchor="middle" font-family="monospace" font-size="18" fill="rgba(0,0,0,0.4)">${label}</text>
</svg>`;
}

const MOCK_FILES = Array.from({ length: 75 }, (_, i) => {
	const mimes = ['image/jpeg', 'image/png', 'image/webp', 'video/mp4'];
	const exts  = ['jpg',        'png',       'webp',       'mp4'      ];
	const mi = i % mimes.length;
	const id = `00000000-0000-7000-8000-${String(i + 1).padStart(12, '0')}`;
	return {
		id,
		original_name: `photo-${String(i + 1).padStart(3, '0')}.${exts[mi]}`,
		mime_type: mimes[mi],
		mime_extension: exts[mi],
		content_datetime: new Date(Date.now() - i * 3_600_000).toISOString(),
		notes: null,
		metadata: null,
		exif: {},
		phash: null,
		creator_id: 1,
		creator_name: 'admin',
		is_public: false,
		is_deleted: false,
		created_at: new Date(Date.now() - i * 3_600_000).toISOString(),
	};
});

const TAG_NAMES = [
	'nature', 'portrait', 'travel', 'architecture', 'food', 'street', 'macro',
	'landscape', 'wildlife', 'urban', 'abstract', 'black-and-white', 'night',
	'golden-hour', 'blue-hour', 'aerial', 'underwater', 'infrared', 'long-exposure',
	'panorama', 'astrophotography', 'documentary', 'editorial', 'fashion', 'wedding',
	'newborn', 'maternity', 'family', 'pet', 'sport', 'concert', 'theatre',
	'interior', 'exterior', 'product', 'still-life', 'automotive', 'aviation',
	'marine', 'industrial', 'medical', 'scientific', 'satellite', 'drone',
	'film', 'analog', 'polaroid', 'tilt-shift', 'fisheye', 'telephoto',
	'wide-angle', 'bokeh', 'silhouette', 'reflection', 'shadow', 'texture',
	'pattern', 'color', 'minimal', 'surreal', 'conceptual', 'fine-art',
	'photojournalism', 'war', 'protest', 'people', 'crowd', 'solitude',
	'children', 'elderly', 'culture', 'tradition', 'festival', 'religion',
	'asia', 'europe', 'africa', 'americas', 'oceania', 'arctic', 'desert',
	'forest', 'mountain', 'ocean', 'lake', 'river', 'waterfall', 'cave',
	'volcano', 'canyon', 'glacier', 'field', 'garden', 'park', 'city',
	'village', 'ruins', 'bridge', 'road', 'railway', 'harbor', 'airport',
	'market', 'cafe', 'restaurant', 'bar', 'museum', 'library', 'school',
	'hospital', 'church', 'mosque', 'temple', 'shrine', 'cemetery', 'stadium',
	'spring', 'summer', 'autumn', 'winter', 'rain', 'snow', 'fog', 'storm',
	'sunrise', 'sunset', 'cloudy', 'clear', 'rainbow', 'lightning', 'wind',
	'cat', 'dog', 'bird', 'horse', 'fish', 'insect', 'reptile', 'mammal',
	'flower', 'tree', 'grass', 'moss', 'mushroom', 'fruit', 'vegetable',
	'fire', 'water', 'earth', 'air', 'smoke', 'ice', 'stone', 'wood', 'metal',
	'glass', 'fabric', 'paper', 'plastic', 'ceramic', 'leather', 'concrete',
	'red', 'orange', 'yellow', 'green', 'cyan', 'blue', 'purple', 'pink',
	'brown', 'white', 'grey', 'dark', 'bright', 'pastel', 'vivid', 'muted',
	'raw', 'edited', 'hdr', 'composite', 'retouched', 'unedited', 'scanned',
	'selfie', 'candid', 'posed', 'staged', 'spontaneous', 'planned', 'series',
];

const TAG_COLORS = [
	'7ECBA1', '9592B5', '4DC7ED', 'E08C5A', 'DB6060',
	'F5E872', 'A67CB8', '5A9ED4', 'C4A44A', '6DB89E',
	'E07090', '70B0E0', 'C0A060', '80C080', 'D080B0',
];

const MOCK_CATEGORIES = [
	{ id: '00000000-0000-7000-8002-000000000001', name: 'Style', color: '9592B5', notes: null, created_at: new Date().toISOString() },
	{ id: '00000000-0000-7000-8002-000000000002', name: 'Subject', color: '4DC7ED', notes: null, created_at: new Date().toISOString() },
	{ id: '00000000-0000-7000-8002-000000000003', name: 'Location', color: '7ECBA1', notes: null, created_at: new Date().toISOString() },
	{ id: '00000000-0000-7000-8002-000000000004', name: 'Season', color: 'E08C5A', notes: null, created_at: new Date().toISOString() },
	{ id: '00000000-0000-7000-8002-000000000005', name: 'Color', color: 'DB6060', notes: null, created_at: new Date().toISOString() },
];

// Assign some tags to categories
const CATEGORY_ASSIGNMENTS: Record<string, string> = {};
TAG_NAMES.forEach((name, i) => {
	if (['film', 'analog', 'polaroid', 'bokeh', 'silhouette', 'long-exposure', 'tilt-shift', 'fisheye', 'telephoto', 'wide-angle', 'macro', 'infrared', 'hdr', 'composite'].includes(name))
		CATEGORY_ASSIGNMENTS[name] = MOCK_CATEGORIES[0].id; // Style
	else if (['portrait', 'wildlife', 'people', 'children', 'elderly', 'cat', 'dog', 'bird', 'horse', 'flower', 'tree', 'insect', 'reptile', 'mammal'].includes(name))
		CATEGORY_ASSIGNMENTS[name] = MOCK_CATEGORIES[1].id; // Subject
	else if (['asia', 'europe', 'africa', 'americas', 'oceania', 'arctic', 'desert', 'forest', 'mountain', 'ocean', 'lake', 'river', 'city', 'village'].includes(name))
		CATEGORY_ASSIGNMENTS[name] = MOCK_CATEGORIES[2].id; // Location
	else if (['spring', 'summer', 'autumn', 'winter'].includes(name))
		CATEGORY_ASSIGNMENTS[name] = MOCK_CATEGORIES[3].id; // Season
	else if (['red', 'orange', 'yellow', 'green', 'cyan', 'blue', 'purple', 'pink', 'brown', 'white', 'grey', 'dark', 'bright', 'pastel', 'vivid', 'muted'].includes(name))
		CATEGORY_ASSIGNMENTS[name] = MOCK_CATEGORIES[4].id; // Color
});

function getCategoryForId(catId: string | null) {
	if (!catId) return null;
	return MOCK_CATEGORIES.find((c) => c.id === catId) ?? null;
}

type MockTag = {
	id: string;
	name: string;
	color: string;
	notes: string | null;
	category_id: string | null;
	category_name: string | null;
	category_color: string | null;
	is_public: boolean;
	created_at: string;
};

const mockTagsArr: MockTag[] = TAG_NAMES.map((name, i) => {
	const catId = CATEGORY_ASSIGNMENTS[name] ?? null;
	const cat = getCategoryForId(catId);
	return {
		id: `00000000-0000-7000-8001-${String(i + 1).padStart(12, '0')}`,
		name,
		color: TAG_COLORS[i % TAG_COLORS.length],
		notes: null,
		category_id: catId,
		category_name: cat?.name ?? null,
		category_color: cat?.color ?? null,
		is_public: false,
		created_at: new Date(Date.now() - i * 3_600_000).toISOString(),
	};
});

// Backwards-compatible reference for existing file-tag lookups
const MOCK_TAGS = mockTagsArr;

// Tag rules: Map<tagId, Set<thenTagId>>
const tagRules = new Map<string, Set<string>>();

// Mutable in-memory state for file metadata and tags
const fileOverrides = new Map<string, Partial<typeof MOCK_FILES[0]>>();
const fileTags = new Map<string, Set<string>>(); // fileId → Set<tagId>

function getMockFile(id: string) {
	const base = MOCK_FILES.find((f) => f.id === id);
	if (!base) return null;
	return { ...base, ...(fileOverrides.get(id) ?? {}) };
}

export function mockApiPlugin(): Plugin {
	return {
		name: 'mock-api',
		configureServer(server) {
			server.middlewares.use(async (req, res, next) => {
				const url = req.url ?? '';
				const method = req.method ?? 'GET';

				if (!url.startsWith('/api/v1')) {
					return next();
				}

				const path = url.replace('/api/v1', '').split('?')[0];

				// POST /auth/login
				if (method === 'POST' && path === '/auth/login') {
					const body = (await readBody(req)) as Record<string, string>;
					if (body.password === 'password') {
						return json(res, 200, TOKEN_PAIR);
					}
					return json(res, 401, { code: 'unauthorized', message: 'Invalid credentials' });
				}

				// POST /auth/refresh
				if (method === 'POST' && path === '/auth/refresh') {
					return json(res, 200, TOKEN_PAIR);
				}

				// POST /auth/logout
				if (method === 'POST' && path === '/auth/logout') {
					return noContent(res);
				}

				// GET /auth/sessions
				if (method === 'GET' && path === '/auth/sessions') {
					return json(res, 200, {
						items: [
							{
								id: 1,
								user_agent: 'Mock Browser',
								started_at: new Date().toISOString(),
								expires_at: null,
								last_activity: new Date().toISOString(),
								is_current: true,
							},
						],
						total: 1,
					});
				}

				// GET /users/me
				if (method === 'GET' && path === '/users/me') {
					return json(res, 200, ME);
				}

				// GET /files/{id}/thumbnail
				const thumbMatch = path.match(/^\/files\/([^/]+)\/thumbnail$/);
				if (method === 'GET' && thumbMatch) {
					const svg = mockThumbSvg(thumbMatch[1]);
					res.writeHead(200, { 'Content-Type': 'image/svg+xml', 'Content-Length': Buffer.byteLength(svg) });
					return res.end(svg);
				}

				// GET /files/{id}/preview  (same SVG, just bigger)
				const previewMatch = path.match(/^\/files\/([^/]+)\/preview$/);
				if (method === 'GET' && previewMatch) {
					const id = previewMatch[1];
					const color = THUMB_COLORS[id.charCodeAt(id.length - 1) % THUMB_COLORS.length];
					const label = id.slice(-4);
					const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600">
  <rect width="800" height="600" fill="${color}"/>
  <text x="400" y="315" text-anchor="middle" font-family="monospace" font-size="48" fill="rgba(0,0,0,0.35)">${label}</text>
</svg>`;
					res.writeHead(200, { 'Content-Type': 'image/svg+xml', 'Content-Length': Buffer.byteLength(svg) });
					return res.end(svg);
				}

				// GET /files/{id}/tags
				const fileTagsGetMatch = path.match(/^\/files\/([^/]+)\/tags$/);
				if (method === 'GET' && fileTagsGetMatch) {
					const ids = fileTags.get(fileTagsGetMatch[1]) ?? new Set<string>();
					return json(res, 200, MOCK_TAGS.filter((t) => ids.has(t.id)));
				}

				// PUT /files/{id}/tags/{tag_id}  — add tag
				const fileTagPutMatch = path.match(/^\/files\/([^/]+)\/tags\/([^/]+)$/);
				if (method === 'PUT' && fileTagPutMatch) {
					const [, fid, tid] = fileTagPutMatch;
					if (!fileTags.has(fid)) fileTags.set(fid, new Set());
					fileTags.get(fid)!.add(tid);
					const ids = fileTags.get(fid)!;
					return json(res, 200, MOCK_TAGS.filter((t) => ids.has(t.id)));
				}

				// DELETE /files/{id}/tags/{tag_id}  — remove tag
				const fileTagDelMatch = path.match(/^\/files\/([^/]+)\/tags\/([^/]+)$/);
				if (method === 'DELETE' && fileTagDelMatch) {
					const [, fid, tid] = fileTagDelMatch;
					fileTags.get(fid)?.delete(tid);
					return noContent(res);
				}

				// GET /files/{id}  — single file
				const fileGetMatch = path.match(/^\/files\/([^/]+)$/);
				if (method === 'GET' && fileGetMatch) {
					const f = getMockFile(fileGetMatch[1]);
					if (!f) return json(res, 404, { code: 'not_found', message: 'File not found' });
					return json(res, 200, f);
				}

				// PATCH /files/{id}  — update metadata
				const filePatchMatch = path.match(/^\/files\/([^/]+)$/);
				if (method === 'PATCH' && filePatchMatch) {
					const id = filePatchMatch[1];
					const base = getMockFile(id);
					if (!base) return json(res, 404, { code: 'not_found', message: 'File not found' });
					const body = (await readBody(req)) as Record<string, unknown>;
					fileOverrides.set(id, { ...(fileOverrides.get(id) ?? {}), ...body });
					return json(res, 200, getMockFile(id));
				}

				// POST /files  — upload (mock: drain body, return a new fake file)
				if (method === 'POST' && path === '/files') {
					// Drain the multipart body without parsing it
					await new Promise<void>((resolve) => {
						req.on('data', () => {});
						req.on('end', resolve);
					});
					const idx = MOCK_FILES.length;
					const id = `00000000-0000-7000-8000-${String(Date.now()).slice(-12)}`;
					const ct = req.headers['content-type'] ?? '';
					// Extract filename from Content-Disposition if present (best-effort)
					const nameMatch = ct.match(/name="([^"]+)"/);
					const newFile = {
						id,
						original_name: nameMatch ? nameMatch[1] : `upload-${idx + 1}.jpg`,
						mime_type: 'image/jpeg',
						mime_extension: 'jpg',
						content_datetime: new Date().toISOString(),
						notes: null,
						metadata: null,
						exif: {},
						phash: null,
						creator_id: 1,
						creator_name: 'admin',
						is_public: false,
						is_deleted: false,
						created_at: new Date().toISOString(),
					};
					MOCK_FILES.unshift(newFile);
					return json(res, 201, newFile);
				}

				// GET /files (cursor pagination + anchor support)
				if (method === 'GET' && path === '/files') {
					const qs = new URLSearchParams(url.split('?')[1] ?? '');
					const anchor = qs.get('anchor');
					const cursor = qs.get('cursor');
					const limit = Math.min(Number(qs.get('limit') ?? 50), 200);

					if (anchor) {
						// Anchor mode: return the anchor file surrounded by neighbors
						const anchorIdx = MOCK_FILES.findIndex((f) => f.id === anchor);
						if (anchorIdx < 0) return json(res, 404, { code: 'not_found', message: 'Anchor not found' });
						const from = Math.max(0, anchorIdx - Math.floor(limit / 2));
						const slice = MOCK_FILES.slice(from, from + limit);
						const next_cursor = from + slice.length < MOCK_FILES.length
							? Buffer.from(String(from + slice.length)).toString('base64') : null;
						const prev_cursor = from > 0
							? Buffer.from(String(from)).toString('base64') : null;
						return json(res, 200, { items: slice, next_cursor, prev_cursor });
					}

					const offset = cursor ? Number(Buffer.from(cursor, 'base64').toString()) : 0;
					const slice = MOCK_FILES.slice(offset, offset + limit);
					const nextOffset = offset + slice.length;
					const next_cursor = nextOffset < MOCK_FILES.length
						? Buffer.from(String(nextOffset)).toString('base64') : null;
					return json(res, 200, { items: slice, next_cursor, prev_cursor: null });
				}

				// GET /tags/{id}/rules
				const tagRulesGetMatch = path.match(/^\/tags\/([^/]+)\/rules$/);
				if (method === 'GET' && tagRulesGetMatch) {
					const tid = tagRulesGetMatch[1];
					const ruleIds = [...(tagRules.get(tid) ?? new Set<string>())];
					const items = ruleIds.map((thenId) => {
						const t = MOCK_TAGS.find((x) => x.id === thenId);
						return { tag_id: tid, then_tag_id: thenId, then_tag_name: t?.name ?? null, is_active: true };
					});
					return json(res, 200, items);
				}

				// POST /tags/{id}/rules
				const tagRulesPostMatch = path.match(/^\/tags\/([^/]+)\/rules$/);
				if (method === 'POST' && tagRulesPostMatch) {
					const tid = tagRulesPostMatch[1];
					const body = (await readBody(req)) as Record<string, unknown>;
					const thenId = body.then_tag_id as string;
					if (!tagRules.has(tid)) tagRules.set(tid, new Set());
					tagRules.get(tid)!.add(thenId);
					const t = MOCK_TAGS.find((x) => x.id === thenId);
					return json(res, 201, { tag_id: tid, then_tag_id: thenId, then_tag_name: t?.name ?? null, is_active: true });
				}

				// DELETE /tags/{id}/rules/{then_id}
				const tagRulesDelMatch = path.match(/^\/tags\/([^/]+)\/rules\/([^/]+)$/);
				if (method === 'DELETE' && tagRulesDelMatch) {
					const [, tid, thenId] = tagRulesDelMatch;
					tagRules.get(tid)?.delete(thenId);
					return noContent(res);
				}

				// GET /tags/{id}
				const tagGetMatch = path.match(/^\/tags\/([^/]+)$/);
				if (method === 'GET' && tagGetMatch) {
					const t = MOCK_TAGS.find((x) => x.id === tagGetMatch[1]);
					if (!t) return json(res, 404, { code: 'not_found', message: 'Tag not found' });
					return json(res, 200, t);
				}

				// PATCH /tags/{id}
				const tagPatchMatch = path.match(/^\/tags\/([^/]+)$/);
				if (method === 'PATCH' && tagPatchMatch) {
					const idx = MOCK_TAGS.findIndex((x) => x.id === tagPatchMatch[1]);
					if (idx < 0) return json(res, 404, { code: 'not_found', message: 'Tag not found' });
					const body = (await readBody(req)) as Partial<MockTag>;
					const catId = body.category_id ?? MOCK_TAGS[idx].category_id;
					const cat = getCategoryForId(catId);
					Object.assign(MOCK_TAGS[idx], {
						...body,
						category_name: cat?.name ?? null,
						category_color: cat?.color ?? null,
					});
					return json(res, 200, MOCK_TAGS[idx]);
				}

				// DELETE /tags/{id}
				const tagDelMatch = path.match(/^\/tags\/([^/]+)$/);
				if (method === 'DELETE' && tagDelMatch) {
					const idx = MOCK_TAGS.findIndex((x) => x.id === tagDelMatch[1]);
					if (idx >= 0) MOCK_TAGS.splice(idx, 1);
					return noContent(res);
				}

				// GET /tags
				if (method === 'GET' && path === '/tags') {
					const qs = new URLSearchParams(url.split('?')[1] ?? '');
					const search = qs.get('search')?.toLowerCase() ?? '';
					const sort = qs.get('sort') ?? 'name';
					const order = qs.get('order') ?? 'asc';
					const limit = Math.min(Number(qs.get('limit') ?? 100), 500);
					const offset = Number(qs.get('offset') ?? 0);

					let filtered = search
						? MOCK_TAGS.filter((t) => t.name.toLowerCase().includes(search))
						: [...MOCK_TAGS];

					filtered.sort((a, b) => {
						let av: string, bv: string;
						if (sort === 'color') { av = a.color; bv = b.color; }
						else if (sort === 'category_name') { av = a.category_name ?? ''; bv = b.category_name ?? ''; }
						else if (sort === 'created') { av = a.created_at; bv = b.created_at; }
						else { av = a.name; bv = b.name; }
						const cmp = av.localeCompare(bv);
						return order === 'desc' ? -cmp : cmp;
					});

					const items = filtered.slice(offset, offset + limit);
					return json(res, 200, { items, total: filtered.length, offset, limit });
				}

				// POST /tags
				if (method === 'POST' && path === '/tags') {
					const body = (await readBody(req)) as Partial<MockTag>;
					const catId = body.category_id ?? null;
					const cat = getCategoryForId(catId);
					const newTag: MockTag = {
						id: `00000000-0000-7000-8001-${String(Date.now()).slice(-12)}`,
						name: body.name ?? 'Unnamed',
						color: body.color ?? '444455',
						notes: body.notes ?? null,
						category_id: catId,
						category_name: cat?.name ?? null,
						category_color: cat?.color ?? null,
						is_public: body.is_public ?? false,
						created_at: new Date().toISOString(),
					};
					MOCK_TAGS.unshift(newTag);
					return json(res, 201, newTag);
				}

				// GET /categories
				if (method === 'GET' && path === '/categories') {
					return json(res, 200, { items: MOCK_CATEGORIES, total: MOCK_CATEGORIES.length, offset: 0, limit: 50 });
				}

				// GET /pools
				if (method === 'GET' && path === '/pools') {
					return json(res, 200, { items: [], total: 0, offset: 0, limit: 50 });
				}

				// Fallback: 404
				return json(res, 404, { code: 'not_found', message: `Mock: no handler for ${method} ${path}` });
			});
		},
	};
}