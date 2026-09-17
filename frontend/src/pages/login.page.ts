import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { LucideAngularModule } from 'lucide-angular';
import { useAuth } from '../hooks/use-auth';
import { apiErrorMessage } from '../utils/api-error';

@Component({
  standalone: true,
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <main class="login-view">
      <section class="identity">
        <span class="emblem"><lucide-icon name="shield-check" [size]="34" /></span>
        <div><p>Offline engineering environment</p><h1>Cell Envelope Console</h1><span>GB-533 · 2D + height validation</span></div>
      </section>
      <section class="login-panel">
        <header><span>Authorized access</span><strong>Engineering sign in</strong></header>
        <div class="role-strip" aria-label="Demo profiles">
          @for (profile of profiles; track profile.username) { <button type="button" [class.active]="form.controls.username.value === profile.username" (click)="select(profile.username, profile.password)">{{ profile.label }}</button> }
        </div>
        <form [formGroup]="form" (ngSubmit)="submit()">
          <mat-form-field appearance="outline"><mat-label>Username</mat-label><input matInput formControlName="username" autocomplete="username" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Password</mat-label><input matInput type="password" formControlName="password" autocomplete="current-password" /></mat-form-field>
          @if (error()) { <p class="error"><lucide-icon name="triangle-alert" [size]="15" />{{ error() }}</p> }
          <button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || busy()"><lucide-icon name="log-in" [size]="17" />{{ busy() ? 'Signing in…' : 'Sign in' }}</button>
        </form>
        <footer><i></i> Simulation boundary enforced</footer>
      </section>
    </main>
  `,
  styles: [`
    .login-view{min-height:100vh;display:grid;grid-template-columns:minmax(280px,1fr) minmax(360px,520px);background:#e8ece9}.identity{display:flex;align-items:flex-end;gap:18px;padding:72px;color:#edf1ef;background:#202a2e;border-bottom:8px solid #d8a91d}.emblem{width:62px;height:62px;display:grid;place-items:center;color:#202a2e;background:#e2b323;border-radius:4px}.identity p,.identity span{margin:0;color:#aeb9b8;font-size:10px;text-transform:uppercase}.identity h1{margin:7px 0;font-size:46px;line-height:1;letter-spacing:0;max-width:600px}.login-panel{align-self:center;margin:34px;padding:30px;background:#f9faf7;border:1px solid #b8c1be;border-radius:4px}.login-panel header{display:grid;gap:5px;margin-bottom:20px}.login-panel header span{color:#687477;font-size:10px;text-transform:uppercase}.login-panel header strong{font-size:22px}.role-strip{display:grid;grid-template-columns:repeat(5,1fr);margin-bottom:19px;border:1px solid #bbc4c1;border-radius:3px;overflow:hidden}.role-strip button{min-width:0;padding:8px 3px;color:#536064;background:#e9edeb;border:0;border-right:1px solid #bbc4c1;font-size:9px;text-transform:uppercase;cursor:pointer}.role-strip button:last-child{border-right:0}.role-strip button.active{color:#202a2e;background:#e3b426;font-weight:750}.login-panel form{display:grid}.login-panel form button{height:44px;display:flex;gap:8px}.error{display:flex;align-items:flex-start;gap:7px;margin:0 0 14px;padding:9px;color:#8d302a;background:#f9e9e6;font-size:11px}.login-panel footer{display:flex;align-items:center;justify-content:center;gap:7px;margin-top:21px;color:#687477;font-size:9px;text-transform:uppercase}.login-panel footer i{width:7px;height:7px;background:#347554;border-radius:50%}
    @media(max-width:820px){.login-view{grid-template-columns:1fr;align-content:start}.identity{align-items:center;padding:24px}.identity h1{font-size:28px}.login-panel{width:min(520px,calc(100% - 24px));margin:28px auto;padding:22px}.role-strip{grid-template-columns:repeat(3,1fr)}.role-strip button{border-bottom:1px solid #bbc4c1}}
  `],
})
export class LoginPage {
  private readonly fb = inject(FormBuilder);
  private readonly router = inject(Router);
  readonly auth = useAuth();
  readonly busy = signal(false);
  readonly error = signal('');
  readonly profiles = [
    { label: 'Engineer', username: 'engineer', password: 'Safety#533' },
    { label: 'Programmer', username: 'programmer', password: 'Program#533' },
    { label: 'Reviewer', username: 'reviewer', password: 'Review#533' },
    { label: 'Auditor', username: 'auditor', password: 'Audit#533' },
    { label: 'Admin', username: 'admin', password: 'Admin#533' },
  ];
  readonly form = this.fb.nonNullable.group({ username: ['engineer', Validators.required], password: ['Safety#533', Validators.required] });

  select(username: string, password: string): void { this.form.setValue({ username, password }); }
  submit(): void {
    if (this.form.invalid) return;
    this.busy.set(true);
    this.error.set('');
    const { username, password } = this.form.getRawValue();
    this.auth.login(username, password).subscribe({
      next: () => { this.busy.set(false); void this.router.navigate(['/cells']); },
      error: (error) => { this.busy.set(false); this.error.set(apiErrorMessage(error)); },
    });
  }
}
