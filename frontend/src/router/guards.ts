import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthStore } from '../stores/auth.store';
import { Role } from '../types/auth';

export const authGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  return auth.authenticated() ? true : inject(Router).createUrlTree(['/login']);
};

export const guestGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  return auth.authenticated() ? inject(Router).createUrlTree(['/cells']) : true;
};

export function roleGuard(...roles: Role[]): CanActivateFn {
  return () => {
    const auth = inject(AuthStore);
    return auth.can(...roles) ? true : inject(Router).createUrlTree(['/cells']);
  };
}
