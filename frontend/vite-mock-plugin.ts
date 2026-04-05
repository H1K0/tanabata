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

				// GET /files
				if (method === 'GET' && path === '/files') {
					return json(res, 200, { items: [], next_cursor: null, prev_cursor: null });
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