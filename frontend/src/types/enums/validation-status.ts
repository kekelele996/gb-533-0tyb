export type ValidationStatus = 'queued' | 'simulating' | 'passed' | 'failed' | 'reviewed' | 'accepted' | 'voided';
export type ProgramState = 'uploaded' | 'parsed' | 'ready' | 'active' | 'superseded' | 'rejected';
export type CellState = 'draft' | 'frozen' | 'inactive';

export const VALIDATION_STATUSES: readonly ValidationStatus[] = ['queued', 'simulating', 'passed', 'failed', 'reviewed', 'accepted', 'voided'];
