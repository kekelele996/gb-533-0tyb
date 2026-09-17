import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { RobotCellStore } from '../stores/robot-cell.store';
import { SafetyZoneStore } from '../stores/safety-zone.store';
import { SafetyZone } from '../types/safety-zone';
import { ZoneType, ZONE_TYPES } from '../types/enums/zone-type';
import { useAuth } from '../hooks/use-auth';
import { CanvasPoint, SafetyCanvasComponent } from '../components/common/safety-canvas.component';
import { CellStateBadgeComponent } from '../components/common/cell-state-badge.component';

@Component({
  standalone: true,
  imports: [DecimalPipe, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, LucideAngularModule, SafetyCanvasComponent, CellStateBadgeComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Geometry editor</span><h1>Safety zones</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="reload()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canEdit()) { <button mat-flat-button color="primary" type="button" (click)="newZone()"><lucide-icon name="plus" [size]="16" />Draw zone</button> }</div></section>
    @if (zones.error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ zones.error() }}</p> }
    <section class="zone-workbench">
      <div class="canvas-area">
        <div class="canvas-toolbar"><mat-form-field appearance="outline"><mat-label>Robot cell</mat-label><mat-select [value]="cellId()" (selectionChange)="selectCell($event.value)">@for (cell of cells.items(); track cell.id) { <mat-option [value]="cell.id">{{ cell.cell_code }} · {{ cell.name }}</mat-option> }</mat-select></mat-form-field><span>{{ activeCount() }} active / {{ zones.items().length }} total</span></div>
        <app-safety-canvas [zones]="zones.items()" [draftPoints]="draftPoints()" [editable]="editorOpen()" (pointAdded)="addPoint($event)" />
        @if (editorOpen()) { <div class="draw-controls"><span>{{ draftPoints().length }} vertices</span><button mat-stroked-button type="button" (click)="undoPoint()" [disabled]="!draftPoints().length"><lucide-icon name="rotate-ccw" [size]="15" />Undo</button><button mat-stroked-button type="button" (click)="resetPoints()">Reset</button></div> }
      </div>
      <aside class="zone-register">
        <header><span>Zone register</span><strong>{{ selectedCellCode() }}</strong></header>
        <div class="zone-list">
          @for (zone of zones.items(); track zone.id) {
            <button type="button" class="zone-row" [class.selected]="zones.selected()?.id === zone.id" (click)="zones.choose(zone)">
              <i [class]="zone.zone_type"></i><span><strong>{{ zone.name }}</strong><small>{{ zone.zone_type }} · v{{ zone.version }}</small></span><app-cell-state-badge [state]="zone.zone_state" />
            </button>
          } @empty { <p class="empty">No zones for this cell</p> }
        </div>
        @if (zones.selected(); as zone) {
          <dl><div><dt>Height</dt><dd>{{ zone.min_height_mm | number:'1.0-0' }}–{{ zone.max_height_mm | number:'1.0-0' }} mm</dd></div><div><dt>Speed limit</dt><dd>{{ zone.speed_limit_mm_s | number:'1.0-0' }} mm/s</dd></div><div class="wide"><dt>Access rule</dt><dd>{{ zone.access_rule }}</dd></div></dl>
          @if (canEdit()) { <div class="zone-actions"><button mat-stroked-button type="button" (click)="editZone(zone)">Revise</button>@if (zone.zone_state === 'draft') { <button mat-flat-button color="primary" type="button" (click)="zones.activate(zone)">Activate</button> } @if (zone.zone_state !== 'inactive') { <button mat-button class="danger" type="button" (click)="zones.deactivate(zone)">Deactivate</button> }</div> }
        }
      </aside>
    </section>
    @if (editorOpen()) {
      <section class="zone-editor">
        <header><div><span>{{ editing() ? 'Zone revision' : 'New geometry' }}</span><h2>{{ editing()?.name ?? 'Click the canvas to place vertices' }}</h2></div><button mat-icon-button aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="18" /></button></header>
        <form [formGroup]="form" (ngSubmit)="save()">
          <mat-form-field appearance="outline"><mat-label>Name</mat-label><input matInput formControlName="name" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Zone type</mat-label><mat-select formControlName="zone_type">@for (type of zoneTypes; track type) { <mat-option [value]="type">{{ type }}</mat-option> }</mat-select></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Minimum height (mm)</mat-label><input matInput type="number" formControlName="min_height_mm" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Maximum height (mm)</mat-label><input matInput type="number" formControlName="max_height_mm" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Speed limit (mm/s)</mat-label><input matInput type="number" formControlName="speed_limit_mm_s" /></mat-form-field>
          <mat-form-field appearance="outline" class="rule"><mat-label>Access rule</mat-label><input matInput formControlName="access_rule" /></mat-form-field>
          <div class="geometry-readout"><span>Polygon</span><code>{{ polygonPreview() }}</code></div>
          <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || draftPoints().length < 3 || zones.loading()"><lucide-icon name="save" [size]="16" />{{ editing() ? 'Save revision' : 'Create zone' }}</button></div>
        </form>
      </section>
    }
  `,
  styles: [`
    .zone-workbench{display:grid;grid-template-columns:minmax(520px,1.35fr) minmax(310px,.65fr);gap:16px}.canvas-area,.zone-register,.zone-editor{background:#fafbf8;border:1px solid #bec8c4;border-radius:4px}.canvas-area{padding:12px}.canvas-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px}.canvas-toolbar mat-form-field{width:min(440px,70%)}.canvas-toolbar span{color:#667376;font-size:10px;text-transform:uppercase}.draw-controls{display:flex;align-items:center;justify-content:flex-end;gap:8px;margin-top:10px}.draw-controls span{margin-right:auto;color:#6b590e;font-size:10px;text-transform:uppercase}.draw-controls button{display:flex;gap:5px}.zone-register{overflow:hidden}.zone-register>header{display:flex;justify-content:space-between;padding:12px 13px;background:#e6ebe8;border-bottom:1px solid #c6cfcc}.zone-register header span{font-size:10px;text-transform:uppercase}.zone-register header strong{font-size:11px}.zone-list{max-height:460px;overflow:auto}.zone-row{width:100%;display:grid;grid-template-columns:6px minmax(0,1fr) auto;gap:10px;align-items:center;padding:11px 12px;background:#fafbf8;border:0;border-bottom:1px solid #dde2e0;text-align:left;cursor:pointer}.zone-row:hover,.zone-row.selected{background:#f7efcf}.zone-row>i{height:32px;background:#3c7d59}.zone-row>i.restricted{background:#b33b32}.zone-row>i.service{background:#c1880f}.zone-row>i.escape{background:#397a98}.zone-row span{display:grid;gap:3px;min-width:0}.zone-row strong{overflow:hidden;text-overflow:ellipsis;font-size:11px}.zone-row small{color:#718083;font-size:9px;text-transform:uppercase}.zone-register dl{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:0;padding:14px}.zone-register dl .wide{grid-column:1/-1}.zone-register dt{color:#768184;font-size:9px;text-transform:uppercase}.zone-register dd{margin:3px 0;font-size:11px}.zone-actions{display:flex;gap:6px;flex-wrap:wrap;padding:0 12px 14px}.danger{color:#98312b!important}.zone-editor{margin-top:16px}.zone-editor>header{display:flex;justify-content:space-between;padding:13px 15px;border-bottom:1px solid #cad2cf}.zone-editor header span{color:#6a7679;font-size:9px;text-transform:uppercase}.zone-editor h2{margin:3px 0 0;font-size:16px}.zone-editor form{display:grid;grid-template-columns:repeat(3,1fr);gap:4px 12px;padding:15px}.rule{grid-column:span 2}.geometry-readout{grid-column:1/-1;display:grid;gap:5px;padding:9px;background:#edf0ee}.geometry-readout span{font-size:9px;text-transform:uppercase}.geometry-readout code{overflow:auto;white-space:nowrap;font-size:10px}.form-actions{grid-column:1/-1;display:flex;justify-content:flex-end;gap:8px}.form-actions button{display:flex;gap:6px}
    @media(max-width:1020px){.zone-workbench{grid-template-columns:1fr}.zone-editor form{grid-template-columns:repeat(2,1fr)}}@media(max-width:650px){.zone-editor form{grid-template-columns:1fr}.rule,.geometry-readout,.form-actions{grid-column:1}.canvas-toolbar{align-items:flex-start;flex-direction:column}.canvas-toolbar mat-form-field{width:100%}}
  `],
})
export class ZonesPage implements OnInit {
  private readonly fb = inject(FormBuilder);
  readonly cells = inject(RobotCellStore);
  readonly zones = inject(SafetyZoneStore);
  readonly auth = useAuth();
  readonly cellId = signal<number | undefined>(undefined);
  readonly editorOpen = signal(false);
  readonly editing = signal<SafetyZone | null>(null);
  readonly draftPoints = signal<CanvasPoint[]>([]);
  readonly zoneTypes = ZONE_TYPES;
  readonly activeCount = computed(() => this.zones.items().filter((zone) => zone.zone_state === 'active').length);
  readonly selectedCellCode = computed(() => this.cells.items().find((cell) => cell.id === this.cellId())?.cell_code ?? 'All cells');
  readonly polygonPreview = computed(() => JSON.stringify(this.polygon()));
  readonly form = this.fb.nonNullable.group({
    name: ['QA light curtain buffer', Validators.required], zone_type: ['restricted' as ZoneType, Validators.required],
    min_height_mm: [0, Validators.min(0)], max_height_mm: [2200, [Validators.required, Validators.min(1)]],
    speed_limit_mm_s: [250, Validators.min(0)], access_rule: ['Gate lock and light curtain clear required', Validators.required],
  });
  canEdit = () => this.auth.can('safety_engineer', 'admin');
  ngOnInit(): void { this.cells.load(); this.zones.load(); }
  reload(): void { this.cells.load(); this.zones.load(this.cellId()); }
  selectCell(id: number): void { this.cellId.set(id); this.zones.selected.set(null); this.zones.load(id); }
  newZone(): void {
    const defaultCell = this.cellId() ?? this.cells.items()[0]?.id;
    if (defaultCell) this.cellId.set(defaultCell);
    this.editing.set(null); this.draftPoints.set([]); this.form.reset({ name: 'QA light curtain buffer', zone_type: 'restricted', min_height_mm: 0, max_height_mm: 2200, speed_limit_mm_s: 250, access_rule: 'Gate lock and light curtain clear required' }); this.editorOpen.set(true);
  }
  editZone(zone: SafetyZone): void {
    this.editing.set(zone); this.cellId.set(zone.robot_cell_id); this.draftPoints.set(this.coordinates(zone));
    this.form.setValue({ name: zone.name, zone_type: zone.zone_type, min_height_mm: zone.min_height_mm, max_height_mm: zone.max_height_mm, speed_limit_mm_s: zone.speed_limit_mm_s, access_rule: zone.access_rule }); this.editorOpen.set(true);
  }
  addPoint(point: CanvasPoint): void { this.draftPoints.update((points) => [...points, point]); }
  undoPoint(): void { this.draftPoints.update((points) => points.slice(0, -1)); }
  resetPoints(): void { this.draftPoints.set([]); }
  closeEditor(): void { this.editorOpen.set(false); this.editing.set(null); this.draftPoints.set([]); }
  save(): void {
    const robotCellId = this.cellId();
    if (this.form.invalid || !robotCellId || this.draftPoints().length < 3) return;
    const value = this.form.getRawValue();
    const current = this.editing();
    const base = { name: value.name, zone_type: value.zone_type, polygon_geojson: this.polygon(), min_height_mm: value.min_height_mm, max_height_mm: value.max_height_mm, speed_limit_mm_s: value.speed_limit_mm_s, access_rule: value.access_rule };
    if (current) this.zones.update(current.id, { ...base, version: current.version }, () => this.closeEditor());
    else this.zones.create({ ...base, robot_cell_id: robotCellId }, () => this.closeEditor());
  }
  private polygon(): Record<string, unknown> {
    const points = this.draftPoints();
    const closed = points.length ? [...points, points[0]] : [];
    return { type: 'Polygon', coordinates: [closed.map((point) => [point.x_mm, point.y_mm])] };
  }
  private coordinates(zone: SafetyZone): CanvasPoint[] {
    const raw = zone.polygon_geojson as { coordinates?: number[][][] };
    return (raw.coordinates?.[0] ?? []).slice(0, -1).map(([x, y]) => ({ x_mm: x, y_mm: y }));
  }
}
