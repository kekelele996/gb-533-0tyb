import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { CreateMotionProgram, MotionProgram } from '../types/motion-program';
import { ProgramState } from '../types/enums/validation-status';

@Injectable({ providedIn: 'root' })
export class MotionProgramApi {
  private readonly api = inject(ApiClient);
  list(robotCellId?: number) { return this.api.page<MotionProgram>('/programs', { page_size: 100, robot_cell_id: robotCellId }); }
  get(id: number) { return this.api.get<MotionProgram>(`/programs/${id}`); }
  create(payload: CreateMotionProgram) { return this.api.post<MotionProgram>('/programs', payload); }
  transition(id: number, targetState: ProgramState) { return this.api.post<MotionProgram>(`/programs/${id}/transition`, { target_state: targetState }); }
}
