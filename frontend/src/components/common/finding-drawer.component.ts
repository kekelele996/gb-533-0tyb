import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { MatExpansionModule } from '@angular/material/expansion';
import { LucideAngularModule } from 'lucide-angular';
import { CollisionEvent, InterlockFinding } from '../../types/validation-run';

@Component({
  selector: 'app-finding-drawer',
  standalone: true,
  imports: [DecimalPipe, MatExpansionModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <mat-accordion class="finding-drawer" multi>
      <mat-expansion-panel [expanded]="collisions().length > 0">
        <mat-expansion-panel-header>
          <mat-panel-title><lucide-icon name="scan-line" [size]="16" /> Envelope events</mat-panel-title>
          <mat-panel-description>{{ collisions().length }}</mat-panel-description>
        </mat-expansion-panel-header>
        @if (!collisions().length) { <p class="empty"><lucide-icon name="circle-check" [size]="16" /> No envelope event recorded</p> }
        @for (item of collisions(); track $index) {
          <article class="finding" [class.violation]="item.violation">
            <div><lucide-icon [name]="item.violation ? 'triangle-alert' : 'info'" [size]="15" /><strong>{{ item.zone_name }}</strong><span>{{ item.zone_type }}</span></div>
            <p>{{ item.evidence }}</p>
            <dl><div><dt>Segment</dt><dd>{{ item.segment_index }}</dd></div><div><dt>First contact</dt><dd>{{ item.first_time_ms | number:'1.0-0' }} ms</dd></div><div><dt>Speed</dt><dd>{{ item.actual_speed_mm_s | number:'1.0-0' }} / {{ item.allowed_speed_mm_s | number:'1.0-0' }} mm/s</dd></div></dl>
          </article>
        }
      </mat-expansion-panel>
      <mat-expansion-panel [expanded]="interlocks().length > 0">
        <mat-expansion-panel-header>
          <mat-panel-title><lucide-icon name="git-branch" [size]="16" /> {{ interlockTitle() }}</mat-panel-title>
          <mat-panel-description>{{ interlocks().length }}</mat-panel-description>
        </mat-expansion-panel-header>
        @if (!interlocks().length) { <p class="empty"><lucide-icon name="circle-check" [size]="16" /> Dependency sequence has no findings</p> }
        @for (item of interlocks(); track $index) {
          <article class="finding violation"><div><lucide-icon name="triangle-alert" [size]="15" /><strong>{{ item.code }}</strong><span>{{ item.event }}</span></div><p>{{ item.evidence }}</p>@if (item.path?.length) { <code>{{ item.path?.join(' → ') }}</code> }</article>
        }
      </mat-expansion-panel>
    </mat-accordion>
  `,
  styles: [`
    .finding-drawer{display:block}.mat-expansion-panel{border:1px solid #c8d0ce;border-radius:4px!important;box-shadow:none!important;margin-bottom:8px}.mat-expansion-panel-header-title{display:flex;align-items:center;gap:8px;font-size:13px;font-weight:700}.mat-expansion-panel-header-description{justify-content:flex-end;font-variant-numeric:tabular-nums}.finding{padding:10px 0;border-top:1px solid #e2e6e4}.finding:first-of-type{border-top:0}.finding>div{display:flex;align-items:center;gap:7px}.finding strong{font-size:12px;text-transform:capitalize}.finding span{margin-left:auto;color:#627075;font-size:10px;text-transform:uppercase}.finding p{margin:5px 0;color:#465257;font-size:11px;line-height:1.45}.finding.violation>div lucide-icon{color:#a7342c}.finding dl{display:flex;gap:20px;margin:8px 0 0}.finding dl div{display:grid;gap:2px}.finding dt{color:#7a8588;font-size:9px;text-transform:uppercase}.finding dd{margin:0;font-size:11px;font-weight:650}.finding code{display:block;overflow:auto;padding:6px;background:#f0f2f0;color:#303a3e;font-size:10px}.empty{display:flex;align-items:center;gap:7px;margin:4px 0;color:#31704f;font-size:11px}
    @media(max-width:600px){.finding dl{display:grid;grid-template-columns:repeat(2,1fr);gap:8px}.mat-expansion-panel-header-description{flex-grow:0}}
  `],
})
export class FindingDrawerComponent {
  readonly collisions = input<CollisionEvent[]>([]);
  readonly interlocks = input<InterlockFinding[]>([]);
  readonly interlockTitle = input('Interlock evidence');
}
