export interface ApiEnvelope<T> {
  data: T;
  request_id: string;
}

export interface PageMeta {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export interface PageEnvelope<T> extends ApiEnvelope<T[]> {
  meta: PageMeta;
}

export interface ApiFailure {
  error?: { code?: string; message?: string };
  request_id?: string;
}
