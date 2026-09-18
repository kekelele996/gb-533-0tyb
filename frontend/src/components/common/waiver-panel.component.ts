import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { LucideAngularModule } from 'lucide-angular';
import { FindingWaiver, FindingWaiverDraft, ValidationRun } from '../../types/validation-run';

interface WaiverTarget {
  key: string;
  kind: 'envelope_violation' | 'interlock_finding';
  index: number;
  title: string;
  detail: string;
}

interface TargetRow extends WaiverTarget {
  waivers: FindingWaiver[];
  effective: FindingWaiver | null;
  expired: FindingWaiver[];
}

interface DraftField {
  reason: string;
  deadline: string;
}

@Component({
  selector: 'app-waiver-panel',
  standalone: true,
  imports: [DatePipe, FormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="waiver-panel">
      <header class="waiver-head">
        <lucide-icon name="shield-half" [size]="16" />
        <div class="waiver-head-text">
          <strong>Violation waivers</strong>
          <span>One unexpired waiver per finding is required before a failed run can be accepted. Uploaders cannot waive their own program.</span>
        </div>
        <span class="coverage" [class.complete]="coveredCount() === targets().length">{{ coveredCount() }}/{{ targets().length }} covered</span>
      </header>
      @for (row of rows(); track row.key) {
        <article class="waiver-row">
          <div class="target">
            <lucide-icon [name]="row.kind === 'envelope_violation' ? 'scan-line' : 'git-branch'" [size]="15" />
            <div class="target-text">
              <strong>{{ row.title }}</strong>
              <p>{{ row.detail }}</p>
            </div>
          </div>
          @if (row.effective; as waiver) {
            <div class="state effective">
              <lucide-icon name="shield-check" [size]="15" />
              <div class="state-text">
                <strong>Effective waiver</strong>
                <span>{{ waiver.granted_by_name }} · expires {{ waiver.expires_at | date:'MMM d, yyyy HH:mm z' }}</span>
                <p class="quote">{{ waiver.reason }}</p>
              </div>
            </div>
          } @else {
            @if (row.expired.length) {
              <div class="state lapsed">
                <lucide-icon name="shield-off" [size]="15" />
                <span>Previous waiver expired {{ row.expired[0].expires_at | date:'MMM d, yyyy HH:mm z' }} — a new one is required</span>
              </div>
            }
            @if (canGrant()) {
              <div class="draft">
                <mat-form-field appearance="outline">
                  <mat-label>Justification</mat-label>
                  <textarea matInput rows="2" [ngModel]="draft(row).reason" (ngModelChange)="setReason(row, $event)" name="reason-{{ row.key }}"></textarea>
                </mat-form-field>
                <mat-form-field appearance="outline">
                  <mat-label>Valid until (local time)</mat-label>
                  <input matInput type="datetime-local" [ngModel]="draft(row).deadline" (ngModelChange)="setDeadline(row, $event)" name="deadline-{{ row.key }}" />
                </mat-form-field>
                <button mat-stroked-button type="button" (click)="save(row)" [disabled]="!valid(row)">
                  <lucide-icon name="shield-plus" [size]="15" /><span>Save waiver</span>
                </button>
              </div>
            } @else if (!row.expired.length) {
              <div class="state missing">
                <lucide-icon name="triangle-alert" [size]="15" />
                <span>No waiver recorded yet</span>
              </div>
            }
          }
          @if (history(row).length) {
            <details class="history">
              <summary>{{ history(row).length }} past waiver(s)</summary>
              @for (waiver of history(row); track waiver.id) {
                <div class="history-item">
                  <span class="history-meta">{{ waiver.granted_by_name }} · {{ waiver.granted_at | date:'MMM d, yyyy HH:mm' }} → {{ waiver.expires_at | date:'MMM d, yyyy HH:mm' }}</span>
                  <p class="history-reason">{{ waiver.reason }}</p>
                </div>
              }
            </details>
          }
        </article>
      }
    </section>
  `,
  styles: [`
    .waiver-panel{border:1px solid #c8d0ce;border-radius:4px;background:#fafbf8;margin:14px;overflow:hidden}
    .waiver-head{display:flex;align-items:center;gap:9px;padding:10px 13px;background:#eef1ec;border-bottom:1px solid #c8d0ce}
    .waiver-head-text{display:grid;gap:2px}
    .waiver-head-text strong{font-size:12px;text-transform:uppercase;letter-spacing:.03em}
    .waiver-head-text span{font-size:10px;color:#5c696d}
    .coverage{margin-left:auto;font-size:11px;font-weight:800;padding:3px 9px;border-radius:99px;background:#f3dfa0;color:#6f5510}
    .coverage.complete{background:#dcecdf;color:#2c6143}
    .waiver-row{padding:11px 13px;border-bottom:1px solid #e2e6e4}
    .waiver-row:last-child{border-bottom:0}
    .target{display:flex;gap:8px;align-items:flex-start}
    .target lucide-icon{margin-top:2px;color:#a7342c}
    .target-text strong{font-size:12px;text-transform:capitalize}
    .target-text p{margin:2px 0 0;color:#4d5a5e;font-size:11px;line-height:1.45}
    .state{display:flex;gap:8px;align-items:flex-start;margin-top:9px;padding:8px 10px;border-radius:3px;font-size:11px}
    .state-text{display:grid;gap:2px}
    .state-text strong{font-size:11px;text-transform:uppercase}
    .state-text span{color:#465257}
    .quote{margin:2px 0 0;font-style:italic}
    .state.effective{background:#e4efe6;color:#2c6143;border:1px solid #b7d5bd}
    .state.effective lucide-icon{color:#2c6143}
    .state.lapsed,.state.missing{background:#f7efd6;color:#7a5c12;border:1px solid #e2d08c}
    .state.lapsed lucide-icon,.state.missing lucide-icon{color:#8a6a17}
    .draft{display:grid;grid-template-columns:minmax(0,1.4fr) minmax(200px,.8fr) auto;gap:9px;align-items:start;margin-top:9px}
    .draft mat-form-field{margin-bottom:-1.2em}
    .draft button{white-space:nowrap;margin-top:4px;display:inline-flex;align-items:center;gap:5px}
    .history{margin-top:7px}
    .history summary{font-size:10px;color:#627075;cursor:pointer;text-transform:uppercase;letter-spacing:.03em}
    .history-item{display:grid;gap:2px;margin:6px 0 0;padding:6px 8px;background:#f0f2f0;border-radius:3px;font-size:10px;color:#3d4a4e}
    .history-meta{color:#6a777a;font-variant-numeric:tabular-nums}
    .history-reason{margin:0}
    @media(max-width:720px){
      .draft{grid-template-columns:1fr}
      .draft button{margin-top:0;white-space:normal}
      .coverage{margin-left:0}
    }
  `],
})
export class WaiverPanelComponent {
  readonly run = input.required<ValidationRun>();
  readonly canReview = input.required<boolean>();
  readonly grant = output<FindingWaiverDraft>();

  readonly nowTick = signal<number>(Date.now());
  private readonly draftMap: Record<string, DraftField> = {};

  readonly targets = computed<WaiverTarget[]>(() => {
    const run = this.run();
    const targets: WaiverTarget[] = [];
    let violationOrdinal = 0;
    run.collision_events.forEach((event) => {
      if (!event.violation) return;
      targets.push({
        key: `envelope_violation:${violationOrdinal}`, kind: 'envelope_violation', index: violationOrdinal,
        title: `Envelope violation #${violationOrdinal + 1} · ${event.zone_name}`,
        detail: `${event.evidence} — first contact ${Math.round(event.first_time_ms)} ms, ${Math.round(event.actual_speed_mm_s)}/${Math.round(event.allowed_speed_mm_s)} mm/s, clearance ${Math.round(event.clearance_mm)} mm`,
      });
      violationOrdinal++;
    });
    run.interlock_findings.forEach((finding, index) => {
      targets.push({
        key: `interlock_finding:${index}`, kind: 'interlock_finding', index,
        title: `Interlock finding #${index + 1} · ${finding.code}`,
        detail: `${finding.evidence}${finding.path?.length ? ' — path: ' + finding.path.join(' → ') : ''}`,
      });
    });
    return targets;
  });

  readonly rows = computed<TargetRow[]>(() => {
    const now = this.nowTick();
    return this.targets().map((target) => {
      const waivers = this.run().finding_waivers
        .filter((waiver) => waiver.finding_type === target.kind && waiver.finding_index === target.index)
        .sort((a, b) => b.granted_at.localeCompare(a.granted_at));
      const effective = waivers.find((waiver) => new Date(waiver.expires_at).getTime() > now) ?? null;
      const expired = waivers.filter((waiver) => new Date(waiver.expires_at).getTime() <= now);
      return { ...target, waivers, effective, expired };
    });
  });

  readonly coveredCount = computed(() => this.rows().filter((row) => !!row.effective).length);

  canGrant(): boolean {
    if (!this.canReview()) return false;
    const status = this.run().validation_status;
    return status === 'failed' || status === 'reviewed';
  }

  valid(row: TargetRow): boolean {
    const draft = this.draft(row);
    if (draft.reason.trim().length < 8) return false;
    const deadline = new Date(draft.deadline);
    return !!draft.deadline && deadline.getTime() > Date.now();
  }

  save(row: TargetRow): void {
    const draft = this.draft(row);
    if (!this.valid(row)) return;
    this.grant.emit(this.toDraft(row, draft));
  }

  // Drafts the acceptance action should append atomically: every open target
  // with a complete, future-dated justification that is not yet stored.
  pending(): FindingWaiverDraft[] {
    const result: FindingWaiverDraft[] = [];
    for (const row of this.rows()) {
      if (row.effective) continue;
      const draft = this.draft(row);
      if (this.valid(row)) result.push(this.toDraft(row, draft));
    }
    return result;
  }

  history(row: TargetRow): FindingWaiver[] {
    return row.effective ? row.expired : row.expired.slice(1);
  }

  draft(row: TargetRow): DraftField {
    const key = `${this.run().id}|${row.key}`;
    let field = this.draftMap[key];
    if (!field) {
      field = { reason: '', deadline: '' };
      this.draftMap[key] = field;
    }
    return field;
  }

  setReason(row: TargetRow, value: string): void { this.draft(row).reason = value; }
  setDeadline(row: TargetRow, value: string): void { this.draft(row).deadline = value; }

  private toDraft(row: TargetRow, draft: DraftField): FindingWaiverDraft {
    return {
      finding_type: row.kind, finding_index: row.index, reason: draft.reason.trim(),
      expires_at: new Date(draft.deadline).toISOString(),
    };
  }
}
