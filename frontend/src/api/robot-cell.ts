import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { CreateRobotCell, RobotCell, UpdateRobotCell } from '../types/robot-cell';

@Injectable({ providedIn: 'root' })
export class RobotCellApi {
  private readonly api = inject(ApiClient);
  list() { return this.api.page<RobotCell>('/cells', { page_size: 100 }); }
  get(id: number) { return this.api.get<RobotCell>(`/cells/${id}`); }
  create(payload: CreateRobotCell) { return this.api.post<RobotCell>('/cells', payload); }
  update(id: number, payload: UpdateRobotCell) { return this.api.put<RobotCell>(`/cells/${id}`, payload); }
  freeze(id: number) { return this.api.post<RobotCell>(`/cells/${id}/freeze`, {}); }
  deactivate(id: number) { return this.api.post<RobotCell>(`/cells/${id}/deactivate`, {}); }
}
