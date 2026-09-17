import { Routes } from '@angular/router';
import { authGuard, guestGuard, roleGuard } from './guards';

export const routes: Routes = [
  { path: 'login', canActivate: [guestGuard], loadComponent: () => import('../pages/login.page').then((module) => module.LoginPage) },
  { path: 'cells', canActivate: [authGuard], loadComponent: () => import('../pages/cells.page').then((module) => module.CellsPage) },
  { path: 'zones', canActivate: [authGuard], loadComponent: () => import('../pages/zones.page').then((module) => module.ZonesPage) },
  { path: 'programs', canActivate: [authGuard], loadComponent: () => import('../pages/programs.page').then((module) => module.ProgramsPage) },
  { path: 'validation', canActivate: [authGuard], loadComponent: () => import('../pages/validation.page').then((module) => module.ValidationPage) },
  { path: 'audit', canActivate: [authGuard, roleGuard('auditor', 'reviewer', 'admin')], loadComponent: () => import('../pages/audit.page').then((module) => module.AuditPage) },
  { path: '', pathMatch: 'full', redirectTo: 'cells' },
  { path: '**', redirectTo: 'cells' },
];
