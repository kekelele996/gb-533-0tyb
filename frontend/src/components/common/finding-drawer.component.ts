import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatExpansionModule } from '@angular/material/expansion';
import { LucideAngularModule } from 'lucide-angular';
import { CollisionEvent, InterlockFinding, ViolationWaiver, WaiverFindingKind } from '../../types/validation-run';

export interface WaiverDraft {
  kind: WaiverFindingKind;
  index: number;
  reason: string;
  deadline: string;
}

@Component({
  selector: 'app-waiver-panel',
  standalone: true,
  imports: [DatePipe, FormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="waiver-block" [class.has-active]="active() !== null">
      @if (active(); as waiver) {
        <div class="waiver-active"><lucide-icon name="shield-check" [size]="15" /><div><strong>Active waiver</strong><span>{{ waiver.reason }}</span><small>{{ waiver.granted_by_name }} · valid until {{ waiver.expires_at | date:'MMM d, yyyy HH:mm' }} UTC</small></div></div>
      }
      @for (waiver of history(); track waiver.id) {
        <div class="waiver-history"><lucide-icon name="history" [size]="13" /><span>{{ waiver.reason }} — {{ waiver.granted_by_name }}, expired {{ waiver.expires_at | date:'MMM d, yyyy' }}</span></div>
      }
      @if (canWaive() && active() === null) {
        <div class="waiver-form">
          <mat-form-field appearance="outline"><mat-label>Waiver justification</mat-label><textarea matInput rows="2" [ngModel]="reason()" (ngModelChange)="reason.set($event)"></textarea></mat-form-field>
          <mat-form-field appearance="outline" class="deadline"><mat-label>Valid until (UTC)</mat-label><input matInput type="datetime-local" [ngModel]="deadline()" (ngModelChange)="deadline.set($event)" /></mat-form-field>
          <div class="waiver-actions">
            <button mat-stroked-button type="button" [disabled]="!ready()" (click)="emit('save')"><lucide-icon name="file-shield" [size]="15" />Save waiver</button>
            <button mat-stroked-button type="button" class="stage" [disabled]="!ready()" (click)="emit('stage')"><lucide-icon name="file-pen-line" [size]="15" />Stage for accept</button>
          </div>
        </div>
      }
    </section>
  `,
  styles: [`
    .waiver-block{margin-top:9px;padding:9px 10px;background:#f6f3e7;border:1px solid #d8cfa8;border-radius:3px}.waiver-block.has-active{background:#eef5ec;border-color:#a9c8ab}.waiver-active{display:flex;gap:8px;align-items:flex-start;color:#2f5d3a}.waiver-active lucide-icon{margin-top:1px}.waiver-active div{display:grid;gap:2px}.waiver-active strong{font-size:10px;text-transform:uppercase}.waiver-active span{font-size:11px;color:#3c4a40}.waiver-active small{font-size:9px;color:#69776c}.waiver-history{display:flex;gap:6px;align-items:flex-start;margin-top:6px;color:#7d7355;font-size:10px}.waiver-history lucide-icon{margin-top:1px;flex:none}.waiver-form{display:flex;gap:8px;align-items:flex-start;flex-wrap:wrap;margin-top:8px}.waiver-form mat-form-field{width:230px;margin-bottom:-14px}.waiver-actions{display:grid;gap:6px}.waiver-actions button{display:flex;gap:5px;white-space:nowrap}.waiver-actions button.stage{color:#8a6d1f;border-color:#d8cfa8}@media(max-width:600px){.waiver-form mat-form-field{width:100%}.waiver-actions{width:100%}.waiver-actions button{width:100%;justify-content:center}}
  `],
})
export class WaiverPanelComponent {
  readonly kind = input.required<WaiverFindingKind>();
  readonly index = input.required<number>();
  readonly waivers = input<ViolationWaiver[]>([]);
  readonly canWaive = input(false);
  readonly saveWaiver = output<WaiverDraft>();
  readonly stageWaiver = output<WaiverDraft>();

  readonly reason = signal('');
  readonly deadline = signal('');
  readonly active = computed(() => this.waivers().find((waiver) => waiver.finding_kind === this.kind() && waiver.finding_index === this.index() && waiver.active) ?? null);
  readonly history = computed(() => this.waivers().filter((waiver) => waiver.finding_kind === this.kind() && waiver.finding_index === this.index() && !waiver.active));
  readonly ready = computed(() => this.reason().trim().length >= 8 && !!this.deadline() && new Date(this.deadline()).getTime() > Date.now());

  emit(mode: 'save' | 'stage'): void {
    if (!this.ready()) return;
    const draft: WaiverDraft = { kind: this.kind(), index: this.index(), reason: this.reason().trim(), deadline: this.deadline() };
    (mode === 'save' ? this.saveWaiver : this.stageWaiver).emit(draft);
    this.reason.set('');
    this.deadline.set('');
  }
}

@Component({
  selector: 'app-finding-drawer',
  standalone: true,
  imports: [DatePipe, DecimalPipe, MatExpansionModule, LucideAngularModule, WaiverPanelComponent],
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
            @if (item.violation) { <app-waiver-panel kind="envelope_violation" [index]="$index" [waivers]="waivers()" [canWaive]="canWaive()" (saveWaiver)="saveWaiver.emit($event)" (stageWaiver)="stageWaiver.emit($event)" /> }
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
          <article class="finding violation">
            <div><lucide-icon name="triangle-alert" [size]="15" /><strong>{{ item.code }}</strong><span>{{ item.event }}</span></div>
            <p>{{ item.evidence }}</p>
            @if (item.path?.length) { <code>{{ item.path?.join(' → ') }}</code> }
            <app-waiver-panel kind="interlock_finding" [index]="$index" [waivers]="waivers()" [canWaive]="canWaive()" (saveWaiver)="saveWaiver.emit($event)" (stageWaiver)="stageWaiver.emit($event)" />
          </article>
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
  readonly waivers = input<ViolationWaiver[]>([]);
  readonly canWaive = input(false);
  readonly saveWaiver = output<WaiverDraft>();
  readonly stageWaiver = output<WaiverDraft>();
}
