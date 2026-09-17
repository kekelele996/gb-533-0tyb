import { inject, Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiEnvelope, PageEnvelope } from '../types/api';

@Injectable({ providedIn: 'root' })
export class ApiClient {
  private readonly http = inject(HttpClient);
  private readonly root = '/api/v1';

  get<T>(path: string, params?: Record<string, string | number | undefined>): Observable<ApiEnvelope<T>> {
    return this.http.get<ApiEnvelope<T>>(this.root + path, { params: this.params(params) });
  }

  page<T>(path: string, params?: Record<string, string | number | undefined>): Observable<PageEnvelope<T>> {
    return this.http.get<PageEnvelope<T>>(this.root + path, { params: this.params(params) });
  }

  post<T>(path: string, body: unknown, headers?: Record<string, string>): Observable<ApiEnvelope<T>> {
    return this.http.post<ApiEnvelope<T>>(this.root + path, body, { headers });
  }

  put<T>(path: string, body: unknown): Observable<ApiEnvelope<T>> {
    return this.http.put<ApiEnvelope<T>>(this.root + path, body);
  }

  private params(values?: Record<string, string | number | undefined>): HttpParams {
    let params = new HttpParams();
    for (const [key, value] of Object.entries(values ?? {})) {
      if (value !== undefined && value !== '') params = params.set(key, String(value));
    }
    return params;
  }
}
