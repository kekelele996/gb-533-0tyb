import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { LoginResponse } from '../types/auth';

@Injectable({ providedIn: 'root' })
export class AuthApi {
  private readonly api = inject(ApiClient);
  login(username: string, password: string) {
    return this.api.post<LoginResponse>('/auth/login', { username, password });
  }
}
