import { ChangeDetectionStrategy, Component, effect, inject, OnInit, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatTableModule } from '@angular/material/table';
import { LucideAngularModule } from 'lucide-angular';
import { RobotCellStore } from '../stores/robot-cell.store';
import { SafetyZoneApi } from '../api/safety-zone';
import { SafetyZone } from '../types/safety-zone';
import { RobotCell } from '../types/robot-cell';
import { useAuth } from '../hooks/use-auth';
import { CellStateBadgeComponent } from '../components/common/cell-state-badge.component';
import { SafetyCanvasComponent } from '../components/common/safety-canvas.component';

@Component({
  standalone: true,
  imports: [DecimalPipe, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatTableModule, LucideAngularModule, CellStateBadgeComponent, SafetyCanvasComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head">
      <div><span>Cell registry</span><h1>Robot work cells</h1></div>
      <div class="head-actions"><button mat-stroked-button type="button" (click)="reload()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canEdit()) { <button mat-flat-button color="primary" type="button" (click)="openCreate()"><lucide-icon name="plus" [size]="16" />New cell</button> }</div>
    </section>
    <aside class="boundary"><lucide-icon name="shield-check" [size]="17" /><div><strong>Simulation boundary</strong><span>Offline geometry evidence only. No controller connection or operation authorization.</span></div></aside>
    @if (store.error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ store.error() }}</p> }
    <section class="cell-layout">
      <div class="registry-panel">
        <div class="panel-label"><span>{{ store.items().length }} records</span><small>Layout / zone / program inventory</small></div>
        <div class="table-scroll">
          <table mat-table [dataSource]="store.items()">
            <ng-container matColumnDef="cell"><th mat-header-cell *matHeaderCellDef>Cell</th><td mat-cell *matCellDef="let cell"><button class="cell-link" type="button" (click)="select(cell)"><strong>{{ cell.cell_code }}</strong><span>{{ cell.name }}</span></button></td></ng-container>
            <ng-container matColumnDef="robot"><th mat-header-cell *matHeaderCellDef>Robot / controller</th><td mat-cell *matCellDef="let cell"><strong>{{ cell.robot_model }}</strong><small>{{ cell.controller_model }}</small></td></ng-container>
            <ng-container matColumnDef="reach"><th mat-header-cell *matHeaderCellDef>Reach</th><td mat-cell *matCellDef="let cell">{{ cell.max_reach_mm | number:'1.0-0' }} mm</td></ng-container>
            <ng-container matColumnDef="assets"><th mat-header-cell *matHeaderCellDef>Assets</th><td mat-cell *matCellDef="let cell">{{ cell.zone_count }} Z / {{ cell.program_count }} P</td></ng-container>
            <ng-container matColumnDef="state"><th mat-header-cell *matHeaderCellDef>State</th><td mat-cell *matCellDef="let cell"><app-cell-state-badge [state]="cell.cell_state" /></td></ng-container>
            <tr mat-header-row *matHeaderRowDef="columns"></tr><tr mat-row *matRowDef="let row; columns: columns" [class.selected]="store.selected()?.id === row.id" (click)="select(row)"></tr>
          </table>
        </div>
      </div>
      <aside class="detail-panel">
        @if (store.selected(); as cell) {
          <header><div><span>{{ cell.cell_code }} · layout v{{ cell.layout_version }}</span><h2>{{ cell.name }}</h2></div><app-cell-state-badge [state]="cell.cell_state" /></header>
          <app-safety-canvas [zones]="zones()" />
          <dl class="cell-facts"><div><dt>Owner</dt><dd>{{ cell.owner_team }}</dd></div><div><dt>Robot</dt><dd>{{ cell.robot_model }}</dd></div><div><dt>Controller</dt><dd>{{ cell.controller_model }}</dd></div><div><dt>Maximum reach</dt><dd>{{ cell.max_reach_mm | number:'1.0-0' }} mm</dd></div></dl>
          @if (canEdit()) { <div class="detail-actions">@if (cell.cell_state === 'draft') { <button mat-stroked-button type="button" (click)="openEdit(cell)">Edit layout</button><button mat-flat-button color="primary" type="button" (click)="store.freeze(cell)"><lucide-icon name="shield-check" [size]="15" />Freeze v{{ cell.layout_version }}</button> } @if (cell.cell_state !== 'inactive') { <button mat-button class="danger" type="button" (click)="store.deactivate(cell)">Deactivate</button> }</div> }
        } @else { <p class="empty">No work cell available</p> }
      </aside>
    </section>
    @if (editorOpen()) {
      <section class="editor-band">
        <header><div><span>{{ editing() ? 'Layout revision' : 'Cell registration' }}</span><h2>{{ editing() ? editing()?.cell_code : 'New robot cell' }}</h2></div><button mat-icon-button aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="18" /></button></header>
        <form [formGroup]="form" (ngSubmit)="save()">
          <mat-form-field appearance="outline"><mat-label>Cell code</mat-label><input matInput formControlName="cell_code" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Name</mat-label><input matInput formControlName="name" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Robot model</mat-label><input matInput formControlName="robot_model" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Controller</mat-label><input matInput formControlName="controller_model" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Maximum reach (mm)</mat-label><input matInput type="number" formControlName="max_reach_mm" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Owner team</mat-label><input matInput formControlName="owner_team" /></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Layout GeoJSON</mat-label><textarea matInput rows="4" formControlName="layout_geojson"></textarea></mat-form-field>
          <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || store.loading()"><lucide-icon name="save" [size]="16" />Save cell</button></div>
        </form>
      </section>
    }
  `,
  styles: [`
    .boundary{display:flex;align-items:flex-start;gap:9px;margin:0 0 18px;padding:10px 12px;color:#5f4a12;background:#fff7d9;border:1px solid #ddc473;border-radius:3px}.boundary div{display:grid;gap:2px}.boundary strong{font-size:11px;text-transform:uppercase}.boundary span{font-size:11px}.cell-layout{display:grid;grid-template-columns:minmax(480px,1.25fr) minmax(360px,.75fr);gap:16px;align-items:start}.registry-panel,.detail-panel,.editor-band{background:#fafbf8;border:1px solid #bec8c4;border-radius:4px}.panel-label{display:flex;justify-content:space-between;padding:10px 13px;background:#e6ebe8;border-bottom:1px solid #c6cfcc}.panel-label span{font-size:11px;font-weight:700;text-transform:uppercase}.panel-label small{color:#667376;font-size:10px}.table-scroll{overflow:auto}table{width:100%;min-width:700px}th{color:#697477!important;font-size:9px!important;text-transform:uppercase}td{font-size:11px!important}td strong,td small{display:block}td small{margin-top:3px;color:#697477}.cell-link{display:grid;gap:2px;padding:0;color:#253034;background:none;border:0;text-align:left;cursor:pointer}.cell-link span{color:#5b686c;font-size:10px}.mat-mdc-row{cursor:pointer}.mat-mdc-row.selected{background:#f7efcf}.detail-panel{position:sticky;top:80px;overflow:hidden}.detail-panel>header,.editor-band>header{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #c8d0cd}.detail-panel header span,.editor-band header span{color:#687579;font-size:9px;text-transform:uppercase}.detail-panel h2,.editor-band h2{margin:3px 0 0;font-size:17px}.detail-panel app-safety-canvas{display:block;margin:14px}.cell-facts{display:grid;grid-template-columns:repeat(2,1fr);gap:12px;margin:14px;padding-top:12px;border-top:1px solid #dce1df}.cell-facts dt{color:#738084;font-size:9px;text-transform:uppercase}.cell-facts dd{margin:3px 0 0;font-size:11px;font-weight:650}.detail-actions{display:flex;gap:8px;flex-wrap:wrap;padding:0 14px 14px}.danger{color:#9a302a!important}.editor-band{margin-top:16px}.editor-band form{display:grid;grid-template-columns:repeat(3,1fr);gap:4px 12px;padding:16px}.editor-band .wide{grid-column:1/-1}.form-actions{grid-column:1/-1;display:flex;justify-content:flex-end;gap:8px}.form-actions button{display:flex;gap:6px}
    @media(max-width:1100px){.cell-layout{grid-template-columns:1fr}.detail-panel{position:static}.editor-band form{grid-template-columns:repeat(2,1fr)}}@media(max-width:650px){.editor-band form{grid-template-columns:1fr}.editor-band .wide,.form-actions{grid-column:1}.panel-label small{display:none}.cell-layout{display:block}.detail-panel{margin-top:12px}}
  `],
})
export class CellsPage implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly zoneApi = inject(SafetyZoneApi);
  readonly store = inject(RobotCellStore);
  readonly auth = useAuth();
  readonly zones = signal<SafetyZone[]>([]);
  readonly editorOpen = signal(false);
  readonly editing = signal<RobotCell | null>(null);
  readonly columns = ['cell', 'robot', 'reach', 'assets', 'state'];
  readonly form = this.fb.nonNullable.group({
    cell_code: ['CELL-B21', [Validators.required, Validators.minLength(3)]], name: ['Inspection and transfer cell', Validators.required],
    robot_model: ['FANUC M-710iC', Validators.required], controller_model: ['R-30iB Plus', Validators.required],
    max_reach_mm: [2606, [Validators.required, Validators.min(1)]], owner_team: ['Integration QA', Validators.required],
    layout_geojson: ['{"type":"FeatureCollection","features":[]}', Validators.required],
  });
  canEdit = () => this.auth.can('safety_engineer', 'admin');
  constructor() {
    effect(() => {
      const cell = this.store.selected();
      if (cell) this.zoneApi.list(cell.id).subscribe(({ data }) => this.zones.set(data));
    });
  }
  ngOnInit(): void { this.reload(); }
  reload(): void { this.store.load(); }
  select(cell: RobotCell): void {
    this.store.choose(cell);
    this.zoneApi.list(cell.id).subscribe(({ data }) => this.zones.set(data));
  }
  openCreate(): void { this.editing.set(null); this.form.enable(); this.form.reset({ cell_code: 'CELL-B21', name: 'Inspection and transfer cell', robot_model: 'FANUC M-710iC', controller_model: 'R-30iB Plus', max_reach_mm: 2606, owner_team: 'Integration QA', layout_geojson: '{"type":"FeatureCollection","features":[]}' }); this.editorOpen.set(true); }
  openEdit(cell: RobotCell): void { this.editing.set(cell); this.form.setValue({ cell_code: cell.cell_code, name: cell.name, robot_model: cell.robot_model, controller_model: cell.controller_model, max_reach_mm: cell.max_reach_mm, owner_team: cell.owner_team, layout_geojson: JSON.stringify(cell.layout_geojson) }); this.form.controls.cell_code.disable(); this.editorOpen.set(true); }
  closeEditor(): void { this.editorOpen.set(false); this.editing.set(null); this.form.controls.cell_code.enable(); }
  save(): void {
    if (this.form.invalid) return;
    let layout: Record<string, unknown>;
    try { layout = JSON.parse(this.form.getRawValue().layout_geojson) as Record<string, unknown>; } catch { this.store.error.set('Layout GeoJSON is not valid JSON'); return; }
    const value = this.form.getRawValue();
    const current = this.editing();
    if (current) this.store.update(current.id, { name: value.name, robot_model: value.robot_model, controller_model: value.controller_model, max_reach_mm: value.max_reach_mm, owner_team: value.owner_team, layout_geojson: layout, layout_version: current.layout_version }, () => this.closeEditor());
    else this.store.create({ cell_code: value.cell_code, name: value.name, robot_model: value.robot_model, controller_model: value.controller_model, max_reach_mm: value.max_reach_mm, owner_team: value.owner_team, layout_geojson: layout }, () => this.closeEditor());
  }
}
