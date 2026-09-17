import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';

@Component({
  selector: 'app-cell-state-badge',
  standalone: true,
  imports: [LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="state" [class]="'state ' + tone()">
      <lucide-icon [name]="icon()" [size]="13" aria-hidden="true" />
      {{ label() }}
    </span>
  `,
  styles: [`
    .state{display:inline-flex;align-items:center;gap:5px;min-height:24px;padding:2px 8px;border:1px solid;border-radius:3px;font-size:11px;font-weight:650;line-height:1;white-space:nowrap}
    .good{color:#1e6645;background:#eaf5ef;border-color:#9dc8af}.warn{color:#72510b;background:#fff7dc;border-color:#dfc36f}
    .bad{color:#8b2c27;background:#faece9;border-color:#dea49e}.neutral{color:#4f5b60;background:#edf0f0;border-color:#c3cbcd}
    .info{color:#265e78;background:#e8f2f6;border-color:#9dc4d5}
  `],
})
export class CellStateBadgeComponent {
  readonly state = input.required<string>();
  readonly label = computed(() => this.state().replaceAll('_', ' '));
  readonly tone = computed(() => {
    if (['active', 'passed', 'accepted'].includes(this.state())) return 'good';
    if (['failed', 'rejected', 'inactive', 'voided'].includes(this.state())) return 'bad';
    if (['frozen', 'reviewed', 'ready'].includes(this.state())) return 'info';
    if (['queued', 'simulating', 'parsed', 'uploaded', 'draft'].includes(this.state())) return 'warn';
    return 'neutral';
  });
  readonly icon = computed(() => {
    if (['active', 'passed', 'accepted'].includes(this.state())) return 'circle-check';
    if (['failed', 'rejected'].includes(this.state())) return 'triangle-alert';
    if (['inactive', 'voided', 'superseded'].includes(this.state())) return 'circle-off';
    if (['frozen', 'reviewed'].includes(this.state())) return 'shield-check';
    return 'circle-dot';
  });
}
