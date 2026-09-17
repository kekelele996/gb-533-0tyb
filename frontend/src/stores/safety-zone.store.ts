import { inject, Injectable, signal } from '@angular/core';
import { finalize } from 'rxjs';
import { SafetyZoneApi } from '../api/safety-zone';
import { CreateSafetyZone, SafetyZone, UpdateSafetyZone } from '../types/safety-zone';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class SafetyZoneStore {
  private readonly api = inject(SafetyZoneApi);
  readonly items = signal<SafetyZone[]>([]);
  readonly selected = signal<SafetyZone | null>(null);
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

  choose(zone: SafetyZone): void { this.selected.set(zone); }
  create(payload: CreateSafetyZone, done?: () => void): void { this.mutate(this.api.create(payload), done); }
  update(id: number, payload: UpdateSafetyZone, done?: () => void): void { this.mutate(this.api.update(id, payload), done); }
  activate(zone: SafetyZone): void { this.mutate(this.api.activate(zone.id, zone.version)); }
  deactivate(zone: SafetyZone): void { this.mutate(this.api.deactivate(zone.id, zone.version)); }

  private mutate(request: ReturnType<SafetyZoneApi['create']>, done?: () => void): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.update((items) => [data, ...items.filter((item) => item.id !== data.id)].sort((a, b) => a.name.localeCompare(b.name)));
        this.selected.set(data);
        done?.();
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
