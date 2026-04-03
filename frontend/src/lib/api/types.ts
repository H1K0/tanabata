import type { components } from './schema';

export type File = components['schemas']['File'];
export type Tag = components['schemas']['Tag'];
export type Category = components['schemas']['Category'];
export type Pool = components['schemas']['Pool'];
export type PoolFile = components['schemas']['PoolFile'];
export type User = components['schemas']['User'];
export type Session = components['schemas']['Session'];
export type Permission = components['schemas']['Permission'];
export type AuditEntry = components['schemas']['AuditLogEntry'];
export type TagRule = components['schemas']['TagRule'];

export type FileCursorPage = components['schemas']['FileCursorPage'];
export type TagOffsetPage = components['schemas']['TagOffsetPage'];
export type CategoryOffsetPage = components['schemas']['CategoryOffsetPage'];
export type PoolOffsetPage = components['schemas']['PoolOffsetPage'];
export type UserOffsetPage = components['schemas']['UserOffsetPage'];
export type AuditOffsetPage = components['schemas']['AuditLogOffsetPage'];
export type PoolFileCursorPage = components['schemas']['PoolFileCursorPage'];
export type SessionList = components['schemas']['SessionList'];

export type ApiError = components['schemas']['Error'];
