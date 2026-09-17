import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TrajectoryPoint } from '../../types/motion-program';
import { SafetyZone } from '../../types/safety-zone';
import { ZoneSnapshot } from '../../types/validation-run';

export interface CanvasPoint { x_mm: number; y_mm: number }

@Component({
  selector: 'app-safety-canvas',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="canvas-shell" [class.editable]="editable()">
      <header>
        <span>Top view · millimetres</span>
        <div class="legend"><i class="operating"></i>Operating <i class="restricted"></i>Restricted <i class="service"></i>Service <i class="path"></i>Tool path</div>
      </header>
      <svg viewBox="0 0 600 360" preserveAspectRatio="xMidYMid meet" role="img" aria-label="Robot cell safety geometry" (click)="addPoint($event)">
        <defs>
          <pattern id="engineering-grid" width="30" height="30" patternUnits="userSpaceOnUse"><path d="M 30 0 L 0 0 0 30" fill="none" stroke="#d8dedc" stroke-width="1" /></pattern>
        </defs>
        <rect width="600" height="360" fill="#f7f8f5" />
        <rect width="600" height="360" fill="url(#engineering-grid)" />
        <line x1="300" y1="0" x2="300" y2="360" class="axis" /><line x1="0" y1="180" x2="600" y2="180" class="axis" />
        @for (zone of zones(); track zone.id) {
          <polygon [attr.points]="zonePoints(zone)" [attr.class]="'zone ' + zone.zone_type" />
          <text [attr.x]="labelPoint(zone).x" [attr.y]="labelPoint(zone).y" class="zone-label">{{ zone.name }}</text>
        }
        @if (trajectory().length > 1) {
          <polyline [attr.points]="trajectoryPoints()" class="trajectory" />
          @for (point of trajectory(); track point.time_ms) { <circle [attr.cx]="mapX(point.x_mm)" [attr.cy]="mapY(point.y_mm)" r="4" class="waypoint" /> }
        }
        @if (draftPoints().length) {
          <polyline [attr.points]="draftPolyline()" class="draft" />
          @for (point of draftPoints(); track $index) { <circle [attr.cx]="mapX(point.x_mm)" [attr.cy]="mapY(point.y_mm)" r="5" class="draft-point" /> }
        }
      </svg>
      <footer><span>X −2500 to 2500</span><strong>{{ editable() ? 'DRAW MODE' : 'SNAPSHOT' }}</strong><span>Y −1800 to 1800</span></footer>
    </section>
  `,
  styles: [`
    .canvas-shell{width:100%;min-width:0;background:#f7f8f5;border:1px solid #aeb8b7;border-radius:4px;overflow:hidden}.canvas-shell header,.canvas-shell footer{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:8px 11px;color:#4e5a5e;background:#e9edeb;font-size:10px;text-transform:uppercase}.canvas-shell header{border-bottom:1px solid #c4cdca}.canvas-shell footer{border-top:1px solid #c4cdca}.canvas-shell footer strong{color:#77590f;letter-spacing:1px}.legend{display:flex;align-items:center;gap:5px;flex-wrap:wrap;justify-content:flex-end}.legend i{width:13px;height:7px;margin-left:5px;border:2px solid}.legend .operating{border-color:#397c58}.legend .restricted{border-color:#ba3f35}.legend .service{border-color:#d39a18}.legend .path{height:0;border-width:2px 0 0;border-color:#222b2f}
    svg{display:block;width:100%;aspect-ratio:5/3;max-height:520px}.editable svg{cursor:crosshair}.axis{stroke:#aeb8b7;stroke-width:1;stroke-dasharray:4 5}.zone{stroke-width:2;vector-effect:non-scaling-stroke}.zone.operating{fill:#3f8b6325;stroke:#397c58}.zone.restricted{fill:#c9443829;stroke:#ba3f35}.zone.service{fill:#d9a21f24;stroke:#b47d07}.zone.escape{fill:#407f9c25;stroke:#397a98}.zone-label{fill:#283236;font-size:10px;font-weight:650;text-anchor:middle;paint-order:stroke;stroke:#f7f8f5;stroke-width:3px}.trajectory{fill:none;stroke:#182126;stroke-width:4;stroke-linecap:round;stroke-linejoin:round;vector-effect:non-scaling-stroke}.waypoint{fill:#f7f8f5;stroke:#182126;stroke-width:2}.draft{fill:none;stroke:#6e520d;stroke-width:3;stroke-dasharray:7 5}.draft-point{fill:#e2ae24;stroke:#5f480d;stroke-width:2}
    @media(max-width:700px){.canvas-shell header{align-items:flex-start;flex-direction:column}.legend{justify-content:flex-start}.canvas-shell footer{font-size:9px}}
  `],
})
export class SafetyCanvasComponent {
  readonly zones = input<Array<SafetyZone | ZoneSnapshot>>([]);
  readonly trajectory = input<TrajectoryPoint[]>([]);
  readonly draftPoints = input<CanvasPoint[]>([]);
  readonly editable = input(false);
  readonly pointAdded = output<CanvasPoint>();

  mapX(value: number): number { return ((value + 2500) / 5000) * 600; }
  mapY(value: number): number { return 360 - ((value + 1800) / 3600) * 360; }

  zonePoints(zone: SafetyZone | ZoneSnapshot): string {
    return this.coordinates(zone).map(([x, y]) => `${this.mapX(x)},${this.mapY(y)}`).join(' ');
  }

  trajectoryPoints(): string {
    return this.trajectory().map((point) => `${this.mapX(point.x_mm)},${this.mapY(point.y_mm)}`).join(' ');
  }

  draftPolyline(): string {
    return this.draftPoints().map((point) => `${this.mapX(point.x_mm)},${this.mapY(point.y_mm)}`).join(' ');
  }

  labelPoint(zone: SafetyZone | ZoneSnapshot): { x: number; y: number } {
    const coordinates = this.coordinates(zone);
    if (!coordinates.length) return { x: 0, y: 0 };
    const x = coordinates.reduce((sum, point) => sum + point[0], 0) / coordinates.length;
    const y = coordinates.reduce((sum, point) => sum + point[1], 0) / coordinates.length;
    return { x: this.mapX(x), y: this.mapY(y) };
  }

  addPoint(event: MouseEvent): void {
    if (!this.editable()) return;
    const svg = event.currentTarget as SVGElement;
    const bounds = svg.getBoundingClientRect();
    const x = ((event.clientX - bounds.left) / bounds.width) * 5000 - 2500;
    const y = 1800 - ((event.clientY - bounds.top) / bounds.height) * 3600;
    this.pointAdded.emit({ x_mm: Math.round(x / 10) * 10, y_mm: Math.round(y / 10) * 10 });
  }

  private coordinates(zone: SafetyZone | ZoneSnapshot): number[][] {
    const source = zone.polygon_geojson;
    if (!source || typeof source !== 'object') return [];
    let geometry = source as Record<string, unknown>;
    if (geometry['type'] === 'Feature') geometry = geometry['geometry'] as Record<string, unknown>;
    if (!geometry || typeof geometry !== 'object') return [];
    const rings = geometry['coordinates'] as number[][][] | undefined;
    return rings?.[0] ?? [];
  }
}
