import { ChangeDetectionStrategy, Component, computed, effect, inject, OnInit } from '@angular/core';
import { DatePipe, DecimalPipe, SlicePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { MotionProgramStore } from '../stores/motion-program.store';
import { useValidationRun } from '../hooks/use-validation-run';
import { useAuth } from '../hooks/use-auth';
import { ValidationRun } from '../types/validation-run';
import { CellStateBadgeComponent } from '../components/common/cell-state-badge.component';
import { SafetyCanvasComponent } from '../components/common/safety-canvas.component';
import { FindingDrawerComponent } from '../components/common/finding-drawer.component';

@Component({
  standalone: true,
  imports: [DatePipe, DecimalPipe, SlicePipe, FormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatProgressBarModule, MatSelectModule, LucideAngularModule, CellStateBadgeComponent, SafetyCanvasComponent, FindingDrawerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Offline simulation</span><h1>Envelope validation</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="reload()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canSimulate()) { <mat-form-field appearance="outline" class="program-select"><mat-label>Ready program</mat-label><mat-select [(ngModel)]="programId">@for (program of eligiblePrograms(); track program.id) { <mat-option [value]="program.id">{{ program.program_code }} · v{{ program.version }}</mat-option> }</mat-select></mat-form-field><button mat-flat-button color="primary" type="button" (click)="run()" [disabled]="!programId || runs.loading()"><lucide-icon name="play" [size]="16" />Run simulation</button> }</div></section>
    <aside class="decision-boundary"><lucide-icon name="triangle-alert" [size]="17" /><div><strong>Acceptance is not permission to operate</strong><span>Certification, risk assessment, commissioning and site authorization remain independent.</span></div></aside>
    @if (runs.loading()) { <mat-progress-bar mode="indeterminate" /> }
    @if (runs.error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ runs.error() }}</p> }
    <section class="validation-grid">
      <div class="run-register">
        <header><span>Immutable runs</span><small>{{ runs.items().length }} snapshots</small></header>
        @for (run of runs.items(); track run.id) {
          <button type="button" class="run-row" [class.selected]="runs.selected()?.id === run.id" (click)="runs.choose(run)">
            <span class="run-id">#{{ run.id }}</span><span><strong>{{ run.program_code }} · v{{ run.program_version }}</strong><small>{{ run.started_at | date:'MMM d, HH:mm:ss' }} · attempt {{ run.attempt }}</small></span><span class="risk">{{ run.risk_score | number:'1.0-0' }}</span><app-cell-state-badge [state]="run.validation_status" />
          </button>
        } @empty { <p class="empty">No validation runs recorded</p> }
      </div>
      <article class="evidence-panel">
        @if (runs.selected(); as run) {
          <header><div><span>Run #{{ run.id }} · {{ run.algorithm_version }}</span><h2>{{ run.program_code }} / attempt {{ run.attempt }}</h2></div><app-cell-state-badge [state]="run.validation_status" /></header>
          <div class="evidence-strip"><div><span>Risk score</span><strong>{{ run.risk_score | number:'1.0-0' }}<small>/100</small></strong></div><div><span>Envelope events</span><strong>{{ run.collision_events.length }}</strong></div><div><span>Interlock findings</span><strong>{{ run.interlock_findings.length }}</strong></div><div><span>Input hash</span><code>{{ run.input_hash | slice:0:12 }}…</code></div></div>
          <app-safety-canvas [zones]="run.zone_snapshot" [trajectory]="run.program_snapshot.trajectory ?? []" />
          <p class="explanation"><lucide-icon name="info" [size]="16" />{{ run.explanation }}</p>
          <app-finding-drawer [collisions]="run.collision_events" [interlocks]="run.interlock_findings" />
          <section class="review-block">
            <div><span>Human review</span>@if (run.review_note) { <p>{{ run.review_note }}</p> } @else { <p>No review note recorded.</p> }</div>
            @if (canReview() && (run.validation_status === 'passed' || run.validation_status === 'failed')) { <mat-form-field appearance="outline"><mat-label>Review note</mat-label><input matInput [(ngModel)]="reviewNote" /></mat-form-field><button mat-flat-button color="primary" type="button" (click)="review(run)" [disabled]="reviewNote.trim().length < 8"><lucide-icon name="user-check" [size]="16" />Record review</button> }
            @if (canReview() && run.validation_status === 'reviewed') { <mat-form-field appearance="outline"><mat-label>Acceptance note</mat-label><input matInput [(ngModel)]="reviewNote" /></mat-form-field><button mat-flat-button color="primary" type="button" (click)="accept(run)" [disabled]="reviewNote.trim().length < 8"><lucide-icon name="circle-check" [size]="16" />Accept evidence</button><button mat-stroked-button type="button" (click)="voidRun(run)" [disabled]="reviewNote.trim().length < 8">Void</button> }
            @if (canSimulate() && run.validation_status === 'failed') { <button mat-stroked-button type="button" (click)="retry(run)"><lucide-icon name="rotate-ccw" [size]="15" />Retry failed input</button> }
          </section>
        } @else { <p class="empty">Select a run to inspect frozen evidence</p> }
      </article>
    </section>
  `,
  styles: [`
    .page-head .head-actions{align-items:center}.program-select{width:250px;margin-bottom:-20px}.decision-boundary{display:flex;gap:9px;margin-bottom:16px;padding:10px 12px;color:#7e2c27;background:#f9e9e6;border:1px solid #dfa59f;border-radius:3px}.decision-boundary div{display:grid;gap:2px}.decision-boundary strong{font-size:11px;text-transform:uppercase}.decision-boundary span{font-size:11px}.validation-grid{display:grid;grid-template-columns:minmax(330px,.6fr) minmax(560px,1.4fr);gap:16px;align-items:start}.run-register,.evidence-panel{background:#fafbf8;border:1px solid #bec8c4;border-radius:4px;overflow:hidden}.run-register>header{display:flex;justify-content:space-between;padding:11px 13px;background:#e6ebe8;border-bottom:1px solid #c6cfcc}.run-register header span{font-size:11px;font-weight:700;text-transform:uppercase}.run-register header small{font-size:10px}.run-row{width:100%;display:grid;grid-template-columns:35px minmax(0,1fr) 34px auto;align-items:center;gap:9px;padding:11px;background:#fafbf8;border:0;border-bottom:1px solid #dce2df;text-align:left;cursor:pointer}.run-row:hover,.run-row.selected{background:#f7efcf}.run-id{font-size:10px;font-weight:800}.run-row>span:nth-child(2){display:grid;gap:3px}.run-row strong{font-size:11px}.run-row small{color:#6a777a;font-size:9px}.risk{height:31px;display:grid;place-items:center;color:#f4f6f3;background:#39464a;border-radius:3px;font-size:11px;font-weight:800}.evidence-panel>header{display:flex;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #c9d1ce}.evidence-panel header span{color:#6b777a;font-size:9px;text-transform:uppercase}.evidence-panel h2{margin:3px 0 0;font-size:17px}.evidence-strip{display:grid;grid-template-columns:repeat(4,1fr);background:#e8ece9;border-bottom:1px solid #cbd3d0}.evidence-strip>div{min-width:0;padding:10px 13px;border-right:1px solid #c5ceca}.evidence-strip>div:last-child{border-right:0}.evidence-strip span{display:block;color:#687578;font-size:9px;text-transform:uppercase}.evidence-strip strong{display:block;margin-top:3px;font-size:20px}.evidence-strip strong small{font-size:9px}.evidence-strip code{display:block;overflow:hidden;margin-top:6px;text-overflow:ellipsis;font-size:10px}.evidence-panel app-safety-canvas,.evidence-panel app-finding-drawer{display:block;margin:14px}.explanation{display:flex;align-items:flex-start;gap:8px;margin:0 14px 14px;padding:10px;background:#edf1ef;color:#4d5a5e;font-size:11px;line-height:1.5}.review-block{display:flex;align-items:center;gap:9px;flex-wrap:wrap;margin:14px;padding-top:13px;border-top:1px solid #d7dedb}.review-block>div{display:grid;gap:3px;margin-right:auto}.review-block>div span{font-size:9px;text-transform:uppercase}.review-block p{margin:0;color:#5c696d;font-size:10px}.review-block mat-form-field{width:240px;margin-bottom:-20px}.review-block button{display:flex;gap:6px}
    @media(max-width:1120px){.validation-grid{grid-template-columns:1fr}.evidence-strip{grid-template-columns:repeat(2,1fr)}.evidence-strip>div:nth-child(2){border-right:0}}@media(max-width:720px){.page-head .head-actions{align-items:stretch}.program-select{width:100%;margin:0}.run-row{grid-template-columns:32px minmax(0,1fr) 32px}.run-row app-cell-state-badge{grid-column:2}.evidence-strip{grid-template-columns:1fr}.evidence-strip>div{border-right:0;border-bottom:1px solid #c5ceca}.review-block{align-items:stretch;flex-direction:column}.review-block mat-form-field{width:100%;margin:0}.review-block>div{margin-right:0}}
  `],
})
export class ValidationPage implements OnInit {
  readonly programs = inject(MotionProgramStore);
  readonly runs = useValidationRun();
  readonly auth = useAuth();
  readonly eligiblePrograms = computed(() => this.programs.items().filter((program) => program.program_state === 'ready' || program.program_state === 'active'));
  programId: number | null = null;
  reviewNote = 'Independent offline evidence review completed.';
  canSimulate = () => this.auth.can('safety_engineer', 'admin');
  canReview = () => this.auth.can('reviewer', 'admin');
  constructor() {
    effect(() => {
      const first = this.eligiblePrograms()[0];
      if (first && !this.programId) this.programId = first.id;
    });
  }
  ngOnInit(): void { this.reload(); }
  reload(): void { this.programs.load(); this.runs.load(); }
  run(): void { if (this.programId) this.runs.create(this.programId); }
  retry(run: ValidationRun): void { this.runs.create(run.motion_program_id, true); }
  review(run: ValidationRun): void { this.runs.review(run, this.reviewNote); }
  accept(run: ValidationRun): void { this.runs.accept(run, this.reviewNote); }
  voidRun(run: ValidationRun): void { this.runs.void(run, this.reviewNote); }
}
