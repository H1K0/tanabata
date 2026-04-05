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

				// GET /files (cursor pagination — page through MOCK_FILES in chunks of 50)
				if (method === 'GET' && path === '/files') {
					const qs = new URLSearchParams(url.split('?')[1] ?? '');
					const cursor = qs.get('cursor');
					const limit = Math.min(Number(qs.get('limit') ?? 50), 200);
					const offset = cursor ? Number(Buffer.from(cursor, 'base64').toString()) : 0;
					const slice = MOCK_FILES.slice(offset, offset + limit);
					const nextOffset = offset + slice.length;
					const next_cursor = nextOffset < MOCK_FILES.length
						? Buffer.from(String(nextOffset)).toString('base64')
						: null;
					return json(res, 200, { items: slice, next_cursor, prev_cursor: null });
				}

				// GET /tags
				if (method === 'GET' && path === '/tags') {
					return json(res, 200, { items: [], total: 0, offset: 0, limit: 50 });
				}

				// GET /categories
				if (method === 'GET' && path === '/categories') {
					return json(res, 200, { items: [], total: 0, offset: 0, limit: 50 });
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