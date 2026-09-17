import { ChangeDetectionStrategy, Component, computed, effect, inject, OnInit, signal } from '@angular/core';
import { DatePipe, DecimalPipe, SlicePipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { MotionProgramStore } from '../stores/motion-program.store';
import { RobotCellStore } from '../stores/robot-cell.store';
import { SafetyZoneApi } from '../api/safety-zone';
import { SafetyZone } from '../types/safety-zone';
import { InterlockEvent, MotionProgram, TrajectoryPoint } from '../types/motion-program';
import { InterlockFinding } from '../types/validation-run';
import { ProgramState } from '../types/enums/validation-status';
import { useAuth } from '../hooks/use-auth';
import { CellStateBadgeComponent } from '../components/common/cell-state-badge.component';
import { SafetyCanvasComponent } from '../components/common/safety-canvas.component';
import { FindingDrawerComponent } from '../components/common/finding-drawer.component';

@Component({
  standalone: true,
  imports: [DatePipe, DecimalPipe, SlicePipe, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, LucideAngularModule, CellStateBadgeComponent, SafetyCanvasComponent, FindingDrawerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Version control</span><h1>Motion programs</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="reload()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canImport()) { <button mat-flat-button color="primary" type="button" (click)="editorOpen.set(true)"><lucide-icon name="upload" [size]="16" />Import program</button> }</div></section>
    @if (programs.error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ programs.error() }}</p> }
    <section class="program-grid">
      <div class="program-register">
        <header><span>Programs</span><small>{{ programs.items().length }} versions</small></header>
        @for (program of programs.items(); track program.id) {
          <button type="button" class="program-row" [class.selected]="programs.selected()?.id === program.id" (click)="programs.choose(program)">
            <span class="version">v{{ program.version }}</span><span class="program-name"><strong>{{ program.program_code }}</strong><small>{{ program.robot_cell_code }} · {{ program.trajectory.length }} waypoints</small></span><app-cell-state-badge [state]="program.program_state" />
          </button>
        } @empty { <p class="empty">No program versions imported</p> }
      </div>
      <div class="program-detail">
        @if (programs.selected(); as program) {
          <header><div><span>{{ program.robot_cell_code }} / source</span><h2>{{ program.program_code }} · v{{ program.version }}</h2></div><app-cell-state-badge [state]="program.program_state" /></header>
          <div class="checksum"><span>SHA-256</span><code>{{ program.source_checksum }}</code></div>
          <app-safety-canvas [zones]="zones()" [trajectory]="program.trajectory" />
          <dl><div><dt>Expansion</dt><dd>{{ program.tool_radius_mm + program.payload_radius_mm | number:'1.0-0' }} mm</dd></div><div><dt>Waypoints</dt><dd>{{ program.trajectory.length }}</dd></div><div><dt>Interlocks</dt><dd>{{ program.interlock_sequence.length }}</dd></div><div><dt>Imported</dt><dd>{{ program.uploaded_at | date:'MMM d, HH:mm' }} UTC</dd></div></dl>
          @if (canImport()) { <div class="state-actions">@if (program.program_state === 'uploaded') { <button mat-flat-button color="primary" (click)="transition(program, 'parsed')">Parse structure</button> } @if (program.program_state === 'parsed') { <button mat-flat-button color="primary" (click)="transition(program, 'ready')">Mark ready</button> } @if (program.program_state === 'ready') { <button mat-flat-button color="primary" (click)="transition(program, 'active')"><lucide-icon name="play" [size]="15" />Activate version</button> } @if (program.program_state === 'active') { <button mat-stroked-button (click)="transition(program, 'superseded')">Supersede</button> }</div> }
          <app-finding-drawer [interlocks]="sequenceFindings()" />
        }
      </div>
    </section>
    @if (editorOpen()) {
      <section class="import-editor">
        <header><div><span>Structured offline import</span><h2>New motion program version</h2></div><button mat-icon-button aria-label="Close import" (click)="editorOpen.set(false)"><lucide-icon name="x" [size]="18" /></button></header>
        <form [formGroup]="form" (ngSubmit)="save()">
          <mat-form-field appearance="outline"><mat-label>Robot cell</mat-label><mat-select formControlName="robot_cell_id">@for (cell of cells.items(); track cell.id) { <mat-option [value]="cell.id">{{ cell.cell_code }} · {{ cell.name }}</mat-option> }</mat-select></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Program code</mat-label><input matInput formControlName="program_code" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Version</mat-label><input matInput type="number" formControlName="version" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Tool radius (mm)</mat-label><input matInput type="number" formControlName="tool_radius_mm" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Payload radius (mm)</mat-label><input matInput type="number" formControlName="payload_radius_mm" /></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Trajectory JSON</mat-label><textarea matInput rows="7" formControlName="trajectory"></textarea></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Interlock sequence JSON</mat-label><textarea matInput rows="7" formControlName="interlocks"></textarea></mat-form-field>
          <aside class="import-rule"><lucide-icon name="shield-check" [size]="17" /><span>Import stores geometry only. It never uploads a controller program.</span></aside>
          <div class="form-actions"><button mat-button type="button" (click)="editorOpen.set(false)">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || programs.loading()"><lucide-icon name="upload" [size]="16" />Import version</button></div>
        </form>
      </section>
    }
  `,
  styles: [`
    .program-grid{display:grid;grid-template-columns:minmax(300px,.55fr) minmax(500px,1.45fr);gap:16px;align-items:start}.program-register,.program-detail,.import-editor{background:#fafbf8;border:1px solid #bec8c4;border-radius:4px;overflow:hidden}.program-register>header{display:flex;justify-content:space-between;padding:11px 13px;background:#e6ebe8;border-bottom:1px solid #c6cfcc}.program-register header span{font-size:11px;font-weight:700;text-transform:uppercase}.program-register header small{color:#697578;font-size:10px}.program-row{width:100%;display:grid;grid-template-columns:38px minmax(0,1fr) auto;align-items:center;gap:10px;padding:11px;background:#fafbf8;border:0;border-bottom:1px solid #dce2df;text-align:left;cursor:pointer}.program-row:hover,.program-row.selected{background:#f7efcf}.version{display:grid;place-items:center;height:31px;color:#f4f6f3;background:#384448;border-radius:3px;font-size:10px;font-weight:700}.program-name{display:grid;gap:3px;min-width:0}.program-name strong{font-size:11px}.program-name small{overflow:hidden;text-overflow:ellipsis;color:#6b787b;font-size:9px;white-space:nowrap}.program-detail>header,.import-editor>header{display:flex;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #c9d1ce}.program-detail header span,.import-editor header span{color:#6b777a;font-size:9px;text-transform:uppercase}.program-detail h2,.import-editor h2{margin:3px 0 0;font-size:17px}.checksum{display:grid;grid-template-columns:auto minmax(0,1fr);gap:9px;padding:9px 14px;background:#e9edeb}.checksum span{font-size:9px;text-transform:uppercase}.checksum code{overflow:hidden;text-overflow:ellipsis;font-size:9px;white-space:nowrap}.program-detail app-safety-canvas{display:block;margin:14px}.program-detail>dl{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin:14px;padding:12px 0;border-top:1px solid #d9dfdc;border-bottom:1px solid #d9dfdc}.program-detail dt{color:#768184;font-size:9px;text-transform:uppercase}.program-detail dd{margin:3px 0 0;font-size:11px;font-weight:700}.state-actions{display:flex;gap:8px;padding:0 14px 14px}.state-actions button{display:flex;gap:6px}.program-detail app-finding-drawer{display:block;margin:0 14px 14px}.import-editor{margin-top:16px}.import-editor form{display:grid;grid-template-columns:repeat(5,1fr);gap:4px 12px;padding:15px}.import-editor .wide{grid-column:span 5}.import-rule{grid-column:1/4;display:flex;align-items:center;gap:8px;padding:9px;color:#675315;background:#fff7d8;border:1px solid #ddc46e;font-size:10px}.form-actions{grid-column:4/6;display:flex;align-items:center;justify-content:flex-end;gap:8px}.form-actions button{display:flex;gap:6px}
    @media(max-width:1050px){.program-grid{grid-template-columns:1fr}.program-detail>dl{padding-left:14px;padding-right:14px}.import-editor form{grid-template-columns:repeat(2,1fr)}.import-editor .wide,.import-rule,.form-actions{grid-column:1/-1}}@media(max-width:650px){.import-editor form{grid-template-columns:1fr}.import-editor .wide,.import-rule,.form-actions{grid-column:1}.program-detail>dl{grid-template-columns:repeat(2,1fr)}.program-row{grid-template-columns:34px minmax(0,1fr)}.program-row app-cell-state-badge{grid-column:2}}
  `],
})
export class ProgramsPage implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly zoneApi = inject(SafetyZoneApi);
  readonly programs = inject(MotionProgramStore);
  readonly cells = inject(RobotCellStore);
  readonly auth = useAuth();
  readonly zones = signal<SafetyZone[]>([]);
  readonly editorOpen = signal(false);
  readonly sequenceFindings = computed(() => this.analyze(this.programs.selected()?.interlock_sequence ?? []));
  readonly form = this.fb.nonNullable.group({
    robot_cell_id: [0, Validators.required], program_code: ['QA-PICK-533', Validators.required], version: [1, [Validators.required, Validators.min(1)]],
    tool_radius_mm: [160, Validators.min(0)], payload_radius_mm: [90, Validators.min(0)],
    trajectory: [JSON.stringify([{ x_mm: -700, y_mm: -300, z_mm: 700, time_ms: 0, speed_mm_s: 300 }, { x_mm: 200, y_mm: 0, z_mm: 850, time_ms: 3000, speed_mm_s: 320 }, { x_mm: 900, y_mm: 200, z_mm: 900, time_ms: 5500, speed_mm_s: 280 }], null, 2), Validators.required],
    interlocks: [JSON.stringify([{ name: 'emergency_stop_reset', sequence: 1, depends_on: [] }, { name: 'transfer_gate_locked', sequence: 2, depends_on: ['emergency_stop_reset'] }, { name: 'light_curtain_clear', sequence: 3, depends_on: ['transfer_gate_locked'] }, { name: 'reduced_speed_selected', sequence: 4, depends_on: ['light_curtain_clear'] }], null, 2), Validators.required],
  });
  canImport = () => this.auth.can('robot_programmer', 'admin');
  constructor() {
    effect(() => {
      const program = this.programs.selected();
      if (program) this.zoneApi.list(program.robot_cell_id).subscribe(({ data }) => this.zones.set(data.filter((zone) => zone.zone_state === 'active')));
    });
    effect(() => {
      const firstCell = this.cells.items()[0];
      if (firstCell && this.form.controls.robot_cell_id.value === 0) this.form.controls.robot_cell_id.setValue(firstCell.id);
    });
  }
  ngOnInit(): void { this.reload(); this.cells.load(); }
  reload(): void { this.programs.load(); }
  transition(program: MotionProgram, state: ProgramState): void { this.programs.transition(program, state); }
  save(): void {
    if (this.form.invalid) return;
    const value = this.form.getRawValue();
    try {
      const trajectory = JSON.parse(value.trajectory) as TrajectoryPoint[];
      const interlockSequence = JSON.parse(value.interlocks) as InterlockEvent[];
      this.programs.create({ robot_cell_id: value.robot_cell_id, program_code: value.program_code, version: value.version, tool_radius_mm: value.tool_radius_mm, payload_radius_mm: value.payload_radius_mm, trajectory, interlock_sequence: interlockSequence }, () => this.editorOpen.set(false));
    } catch { this.programs.error.set('Trajectory and interlock fields must contain valid JSON arrays'); }
  }
  private analyze(events: InterlockEvent[]): InterlockFinding[] {
    const byName = new Map(events.map((event) => [event.name, event]));
    const findings: InterlockFinding[] = [];
    for (const event of events) for (const dependency of event.depends_on) {
      const prerequisite = byName.get(dependency);
      if (!prerequisite) findings.push({ code: 'missing_prerequisite', event: event.name, depends_on: dependency, evidence: `${event.name} references missing prerequisite ${dependency}` });
      else if (prerequisite.sequence >= event.sequence) findings.push({ code: 'reversed_order', event: event.name, depends_on: dependency, evidence: `${dependency} occurs at sequence ${prerequisite.sequence}, not before ${event.sequence}` });
    }
    return findings;
  }
}
