import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { AuditApi } from '../api/audit';
import { AuditEvent, InterlockFinding } from '../types/validation-run';
import { apiErrorMessage } from '../utils/api-error';
import { FindingDrawerComponent } from '../components/common/finding-drawer.component';

@Component({
  standalone: true,
  imports: [DatePipe, FormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, LucideAngularModule, FindingDrawerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Traceability</span><h1>Audit center</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="load()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button></div></section>
    <section class="audit-filters">
      <mat-form-field appearance="outline"><mat-label>Actor</mat-label><input matInput [(ngModel)]="actor" /></mat-form-field>
      <mat-form-field appearance="outline"><mat-label>Request ID</mat-label><input matInput [(ngModel)]="requestId" /></mat-form-field>
      <mat-form-field appearance="outline"><mat-label>Entity</mat-label><mat-select [(ngModel)]="resourceType"><mat-option value="">All entities</mat-option><mat-option value="robot_cell">RobotCell</mat-option><mat-option value="safety_zone">SafetyZone</mat-option><mat-option value="motion_program">MotionProgram</mat-option><mat-option value="validation_run">ValidationRun</mat-option></mat-select></mat-form-field>
      <button mat-flat-button color="primary" type="button" (click)="load()">Apply filters</button>
    </section>
    @if (error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ error() }}</p> }
    <section class="audit-layout">
      <div class="event-stream">
        <header><span>Append-only stream</span><small>{{ events().length }} events</small></header>
        @for (event of events(); track event.id) {
          <button type="button" class="event-row" [class.selected]="selected()?.id === event.id" (click)="selected.set(event)">
            <span class="event-icon"><lucide-icon [name]="icon(event.resource_type)" [size]="16" /></span><span class="event-main"><strong>{{ event.action }}</strong><small>{{ event.resource_type }} #{{ event.resource_id }} · {{ event.actor }}</small></span><span class="event-time">{{ event.created_at | date:'MMM d' }}<small>{{ event.created_at | date:'HH:mm:ss' }}</small></span>
          </button>
        } @empty { <p class="empty">No events match these filters</p> }
      </div>
      <aside class="audit-detail">
        @if (selected(); as event) {
          <header><div><span>Event #{{ event.id }}</span><h2>{{ event.action }}</h2></div><strong>{{ event.role.replaceAll('_', ' ') }}</strong></header>
          <dl><div><dt>Actor</dt><dd>{{ event.actor }}</dd></div><div><dt>Entity</dt><dd>{{ event.resource_type }} #{{ event.resource_id }}</dd></div><div class="wide"><dt>Request ID</dt><dd><code>{{ event.request_id }}</code></dd></div><div class="wide"><dt>Timestamp</dt><dd>{{ event.created_at | date:'medium' }} UTC</dd></div></dl>
          <app-finding-drawer [interlockTitle]="'Change projection'" [interlocks]="projection()" />
          <section class="json-evidence"><div><span>Before</span><pre>{{ pretty(event.before) }}</pre></div><div><span>After</span><pre>{{ pretty(event.after) }}</pre></div><div><span>Parameters</span><pre>{{ pretty(event.parameters) }}</pre></div></section>
        } @else { <p class="empty">Select an event to inspect its immutable projection</p> }
      </aside>
    </section>
  `,
  styles: [`
    .audit-filters{display:grid;grid-template-columns:1fr 1.5fr 1fr auto;gap:10px;align-items:start;margin-bottom:16px;padding:12px;background:#e6ebe8;border:1px solid #c1cbc7;border-radius:4px}.audit-filters button{height:40px}.audit-layout{display:grid;grid-template-columns:minmax(360px,.7fr) minmax(500px,1.3fr);gap:16px;align-items:start}.event-stream,.audit-detail{background:#fafbf8;border:1px solid #bec8c4;border-radius:4px;overflow:hidden}.event-stream>header{display:flex;justify-content:space-between;padding:11px 13px;background:#e6ebe8;border-bottom:1px solid #c6cfcc}.event-stream header span{font-size:11px;font-weight:700;text-transform:uppercase}.event-stream header small{font-size:10px}.event-row{width:100%;display:grid;grid-template-columns:34px minmax(0,1fr) auto;align-items:center;gap:9px;padding:11px;background:#fafbf8;border:0;border-bottom:1px solid #dce2df;text-align:left;cursor:pointer}.event-row:hover,.event-row.selected{background:#f7efcf}.event-icon{width:31px;height:31px;display:grid;place-items:center;color:#f3f5f2;background:#39464a;border-radius:3px}.event-main{display:grid;gap:3px;min-width:0}.event-main strong{overflow:hidden;text-overflow:ellipsis;font-size:11px}.event-main small{overflow:hidden;text-overflow:ellipsis;color:#6b777a;font-size:9px;white-space:nowrap}.event-time{display:grid;gap:2px;text-align:right;font-size:9px;text-transform:uppercase}.event-time small{color:#6b777a}.audit-detail>header{display:flex;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #c9d1ce}.audit-detail header span{color:#6b777a;font-size:9px;text-transform:uppercase}.audit-detail h2{margin:3px 0 0;font-size:17px}.audit-detail header>strong{font-size:10px;text-transform:uppercase}.audit-detail dl{display:grid;grid-template-columns:repeat(2,1fr);gap:12px;margin:14px}.audit-detail dl .wide{grid-column:1/-1}.audit-detail dt{color:#748084;font-size:9px;text-transform:uppercase}.audit-detail dd{margin:3px 0;font-size:11px}.audit-detail code{word-break:break-all}.audit-detail app-finding-drawer{display:block;margin:14px}.json-evidence{display:grid;grid-template-columns:repeat(3,1fr);gap:1px;margin-top:14px;background:#c8d0cd;border-top:1px solid #c8d0cd}.json-evidence>div{min-width:0;padding:11px;background:#eef1ef}.json-evidence span{font-size:9px;text-transform:uppercase}.json-evidence pre{min-height:88px;max-height:220px;overflow:auto;margin:6px 0 0;color:#344044;font-size:9px;white-space:pre-wrap;word-break:break-word}
    @media(max-width:1020px){.audit-layout{grid-template-columns:1fr}.audit-filters{grid-template-columns:repeat(2,1fr)}}@media(max-width:650px){.audit-filters{grid-template-columns:1fr}.json-evidence{grid-template-columns:1fr}.event-row{grid-template-columns:32px minmax(0,1fr)}.event-time{grid-column:2;text-align:left}}
  `],
})
export class AuditPage implements OnInit {
  private readonly api = inject(AuditApi);
  readonly events = signal<AuditEvent[]>([]);
  readonly selected = signal<AuditEvent | null>(null);
  readonly error = signal('');
  readonly projection = computed<InterlockFinding[]>(() => {
    const event = this.selected();
    if (!event) return [];
    return [{ code: 'immutable_projection', event: event.action, evidence: `Recorded ${event.resource_type} change by ${event.actor}; compare the before and after snapshots below.` }];
  });
  actor = '';
  requestId = '';
  resourceType = '';
  ngOnInit(): void { this.load(); }
  load(): void {
    this.error.set('');
    this.api.list({ actor: this.actor, request_id: this.requestId, resource_type: this.resourceType }).subscribe({
      next: ({ data }) => { this.events.set(data); this.selected.set(data[0] ?? null); },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
  pretty(value: Record<string, unknown>): string { return JSON.stringify(value, null, 2); }
  icon(resource: string): string {
    if (resource === 'robot_cell') return 'boxes';
    if (resource === 'safety_zone') return 'map';
    if (resource === 'motion_program') return 'file-code-2';
    return 'scan-line';
  }
}
