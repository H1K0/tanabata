import { redirect } from '@sveltejs/kit';

// The app has no content at the root; send users to the files view (the layout
// guard redirects to /login first when unauthenticated).
export const load = () => {
	redirect(307, '/files');
};
