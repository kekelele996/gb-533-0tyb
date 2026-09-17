import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { CreateSafetyZone, SafetyZone, UpdateSafetyZone } from '../types/safety-zone';

@Injectable({ providedIn: 'root' })
export class SafetyZoneApi {
  private readonly api = inject(ApiClient);
  list(robotCellId?: number) { return this.api.page<SafetyZone>('/zones', { page_size: 150, robot_cell_id: robotCellId }); }
  get(id: number) { return this.api.get<SafetyZone>(`/zones/${id}`); }
  create(payload: CreateSafetyZone) { return this.api.post<SafetyZone>('/zones', payload); }
  update(id: number, payload: UpdateSafetyZone) { return this.api.put<SafetyZone>(`/zones/${id}`, payload); }
  activate(id: number, version: number) { return this.api.post<SafetyZone>(`/zones/${id}/activate`, { version }); }
  deactivate(id: number, version: number) { return this.api.post<SafetyZone>(`/zones/${id}/deactivate`, { version }); }
}
