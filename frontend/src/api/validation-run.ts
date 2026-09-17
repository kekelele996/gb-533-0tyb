import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { ValidationRun } from '../types/validation-run';

@Injectable({ providedIn: 'root' })
export class ValidationRunApi {
  private readonly api = inject(ApiClient);
  list() { return this.api.page<ValidationRun>('/validations', { page_size: 100 }); }
  get(id: number) { return this.api.get<ValidationRun>(`/validations/${id}`); }
  create(motionProgramId: number, idempotencyKey: string, retryFailed = false) {
    return this.api.post<ValidationRun>('/validations', { motion_program_id: motionProgramId, retry_failed: retryFailed }, { 'Idempotency-Key': idempotencyKey });
  }
  review(id: number, note: string) { return this.api.post<ValidationRun>(`/validations/${id}/review`, { note }); }
  accept(id: number, note: string) { return this.api.post<ValidationRun>(`/validations/${id}/accept`, { note }); }
  void(id: number, note: string) { return this.api.post<ValidationRun>(`/validations/${id}/void`, { note }); }
}
