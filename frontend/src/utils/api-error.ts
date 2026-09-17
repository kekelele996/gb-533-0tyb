import { HttpErrorResponse } from '@angular/common/http';
import { ApiFailure } from '../types/api';

export function apiErrorMessage(error: unknown): string {
  if (!(error instanceof HttpErrorResponse)) return 'Unexpected application error';
  const failure = error.error as ApiFailure | undefined;
  const message = failure?.error?.message ?? `Request failed with HTTP ${error.status}`;
  const requestId = failure?.request_id ? ` · Request ${failure.request_id}` : '';
  return message + requestId;
}
