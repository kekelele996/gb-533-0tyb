import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { LucideAngularModule } from 'lucide-angular';
import { useAuth } from '../hooks/use-auth';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatButtonModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (auth.authenticated()) {
      <div class="app-frame">
        <header class="topbar">
          <a class="brand" routerLink="/cells"><span class="brand-mark"><lucide-icon name="shield-check" [size]="21" /></span><span><strong>Envelope / 533</strong><small>Robot cell safety validation</small></span></a>
          <div class="session"><span><b>{{ auth.user()?.username }}</b><small>{{ auth.role()?.replaceAll('_', ' ') }}</small></span><button mat-icon-button type="button" aria-label="Sign out" title="Sign out" (click)="logout()"><lucide-icon name="log-out" [size]="18" /></button></div>
        </header>
        <nav class="rail" aria-label="Primary navigation">
          <a routerLink="/cells" routerLinkActive="active"><lucide-icon name="boxes" [size]="17" /><span>Cells</span></a>
          <a routerLink="/zones" routerLinkActive="active"><lucide-icon name="map" [size]="17" /><span>Zones</span></a>
          <a routerLink="/programs" routerLinkActive="active"><lucide-icon name="file-code-2" [size]="17" /><span>Programs</span></a>
          <a routerLink="/validation" routerLinkActive="active"><lucide-icon name="scan-line" [size]="17" /><span>Validation</span></a>
          @if (auth.can('auditor', 'reviewer', 'admin')) { <a routerLink="/audit" routerLinkActive="active"><lucide-icon name="clipboard-check" [size]="17" /><span>Audit</span></a> }
        </nav>
        <main class="workspace"><router-outlet /></main>
      </div>
    } @else {
      <router-outlet />
    }
  `,
  styles: [`
    .app-frame{min-height:100vh;display:grid;grid-template-columns:86px minmax(0,1fr);grid-template-rows:58px minmax(0,1fr);background:#f2f4f1}.topbar{position:sticky;top:0;z-index:20;grid-column:1/-1;display:flex;align-items:center;justify-content:space-between;padding:0 18px;color:#edf1ef;background:#202a2e;border-bottom:3px solid #d9a91c}.brand{display:flex;align-items:center;gap:10px;color:inherit;text-decoration:none}.brand-mark{width:34px;height:34px;display:grid;place-items:center;color:#202a2e;background:#e2b323;border-radius:3px}.brand span:last-child{display:grid}.brand strong{font-size:14px;letter-spacing:0}.brand small{color:#aeb9b8;font-size:9px;text-transform:uppercase}.session{display:flex;align-items:center;gap:8px}.session>span{display:grid;text-align:right}.session b{font-size:11px}.session small{color:#aeb9b8;font-size:9px;text-transform:uppercase}.session button{color:#edf1ef}.rail{position:sticky;top:58px;height:calc(100vh - 58px);display:flex;flex-direction:column;padding:12px 8px;gap:4px;background:#dfe4e1;border-right:1px solid #bcc6c2}.rail a{height:54px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:5px;color:#48555a;border-radius:3px;font-size:10px;text-decoration:none}.rail a:hover{background:#eef1ef;color:#202a2e}.rail a.active{color:#172126;background:#f9faf7;box-shadow:inset 3px 0 #c5940b;font-weight:700}.workspace{min-width:0;padding:22px clamp(14px,2.4vw,34px) 42px}
    @media(max-width:760px){.app-frame{display:block;padding-top:58px}.topbar{position:fixed;left:0;right:0;height:58px}.brand small,.session>span{display:none}.rail{position:fixed;z-index:18;top:auto;bottom:0;left:0;right:0;height:62px;flex-direction:row;justify-content:space-around;padding:5px 4px;border-top:1px solid #adb9b5;border-right:0}.rail a{flex:1;height:51px}.rail a.active{box-shadow:inset 0 3px #c5940b}.workspace{padding:16px 12px 82px}}
  `],
})
export class AppComponent {
  readonly auth = useAuth();
  private readonly router = inject(Router);
  logout(): void { this.auth.logout(); void this.router.navigate(['/login']); }
}
