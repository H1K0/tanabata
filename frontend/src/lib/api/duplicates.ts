import { api } from '$lib/api/client';
import type { File } from '$lib/api/types';

/** A group of mutually similar files. */
export interface DuplicateCluster {
	files: File[];
}

export interface DuplicateClusterPage {
	items: DuplicateCluster[];
	total: number;
	offset: number;
	limit: number;
}

/** Per-field source for a merge. Scalars choose keep/discard; relations
 *  (tags, pools) choose keep/both; metadata can also be shallow-merged. */
export type ScalarChoice = 'keep' | 'discard';
export type RelationChoice = 'keep' | 'both';
export type MetadataChoice = 'keep' | 'discard' | 'merge';

export interface MergeFields {
	original_name?: ScalarChoice;
	notes?: ScalarChoice;
	content_datetime?: ScalarChoice;
	is_public?: ScalarChoice;
	metadata?: MetadataChoice;
	tags?: RelationChoice;
	pools?: RelationChoice;
}

export interface ResolveRequest {
	keep: string;
	discard: string;
	fields?: MergeFields;
	delete_discarded?: boolean;
}

/** Fetch a page of duplicate clusters (server reads a precomputed table). */
export function getDuplicates(limit = 20, offset = 0): Promise<DuplicateClusterPage> {
	return api.get<DuplicateClusterPage>(`/files/duplicates?limit=${limit}&offset=${offset}`);
}

/** Mark two files as "not a duplicate" so the pair stops surfacing. */
export function dismissDuplicate(a: string, b: string): Promise<void> {
	return api.post<void>('/files/duplicates/dismiss', { file_id_a: a, file_id_b: b });
}

/** Merge a duplicate pair, returning the updated survivor. */
export function resolveDuplicate(req: ResolveRequest): Promise<File> {
	return api.post<File>('/files/duplicates/resolve', req);
}
