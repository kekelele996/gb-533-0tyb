import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { FindingWaiverDraft, ValidationRun } from '../types/validation-run';

@Injectable({ providedIn: 'root' })
export class ValidationRunApi {
  private readonly api = inject(ApiClient);
  list() { return this.api.page<ValidationRun>('/validations', { page_size: 100 }); }
  get(id: number) { return this.api.get<ValidationRun>(`/validations/${id}`); }
  create(motionProgramId: number, idempotencyKey: string, retryFailed = false) {
    return this.api.post<ValidationRun>('/validations', { motion_program_id: motionProgramId, retry_failed: retryFailed }, { 'Idempotency-Key': idempotencyKey });
  }
  review(id: number, note: string) { return this.api.post<ValidationRun>(`/validations/${id}/review`, { note }); }
  grantWaiver(id: number, waiver: FindingWaiverDraft) {
    return this.api.post<ValidationRun>(`/validations/${id}/waivers`, { waiver });
  }
  accept(id: number, note: string, waivers: FindingWaiverDraft[]) {
    return this.api.post<ValidationRun>(`/validations/${id}/accept`, { note, waivers });
  }
  void(id: number, note: string) { return this.api.post<ValidationRun>(`/validations/${id}/void`, { note }); }
}
