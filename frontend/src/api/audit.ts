import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { AuditEvent } from '../types/validation-run';

@Injectable({ providedIn: 'root' })
export class AuditApi {
  private readonly api = inject(ApiClient);
  list(filters: { actor?: string; request_id?: string; resource_type?: string; action?: string } = {}) {
    return this.api.page<AuditEvent>('/audit', { page_size: 150, ...filters });
  }
}
