import { inject, Injectable, signal } from '@angular/core';
import { finalize } from 'rxjs';
import { MotionProgramApi } from '../api/motion-program';
import { CreateMotionProgram, MotionProgram } from '../types/motion-program';
import { ProgramState } from '../types/enums/validation-status';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class MotionProgramStore {
  private readonly api = inject(MotionProgramApi);
  readonly items = signal<MotionProgram[]>([]);
  readonly selected = signal<MotionProgram | null>(null);
  readonly loading = signal(false);
  readonly error = signal('');

  load(robotCellId?: number): void {
    this.loading.set(true);
    this.error.set('');
    this.api.list(robotCellId).pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.set(data);
        if (!this.selected() && data.length) this.selected.set(data[0]);
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }

  choose(program: MotionProgram): void { this.selected.set(program); }
  create(payload: CreateMotionProgram, done?: () => void): void { this.mutate(this.api.create(payload), done); }
  transition(program: MotionProgram, target: ProgramState): void { this.mutate(this.api.transition(program.id, target)); }

  private mutate(request: ReturnType<MotionProgramApi['create']>, done?: () => void): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.update((items) => [data, ...items.filter((item) => item.id !== data.id)]);
        this.selected.set(data);
        done?.();
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
