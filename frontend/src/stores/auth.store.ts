import { computed, inject, Injectable, signal } from '@angular/core';
import { tap } from 'rxjs';
import { AuthApi } from '../api/auth';
import { Role, UserView } from '../types/auth';

@Injectable({ providedIn: 'root' })
export class AuthStore {
  private readonly api = inject(AuthApi);
  private readonly userState = signal<UserView | null>(this.restoreUser());
  readonly user = this.userState.asReadonly();
  readonly authenticated = computed(() => this.userState() !== null && !!sessionStorage.getItem('robot_safety_token'));
  readonly role = computed(() => this.userState()?.role ?? null);

  login(username: string, password: string) {
    return this.api.login(username, password).pipe(tap(({ data }) => {
      sessionStorage.setItem('robot_safety_token', data.token);
      sessionStorage.setItem('robot_safety_user', JSON.stringify(data.user));
      this.userState.set(data.user);
    }));
  }

  logout(): void {
    sessionStorage.removeItem('robot_safety_token');
    sessionStorage.removeItem('robot_safety_user');
    this.userState.set(null);
  }

  can(...roles: Role[]): boolean {
    const role = this.role();
    return role !== null && roles.includes(role);
  }

  private restoreUser(): UserView | null {
    const encoded = sessionStorage.getItem('robot_safety_user');
    if (!encoded) return null;
    try {
      return JSON.parse(encoded) as UserView;
    } catch {
      sessionStorage.removeItem('robot_safety_user');
      return null;
    }
  }
}
