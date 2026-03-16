export type JobStatus = "queued" | "processing" | "completed" | "failed";

export interface SubmitReceiptResponse {
  job_id: number;
  status: JobStatus;
  fiscal_url: string;
  receipt_id?: number;
  message?: string;
}

export interface JobResponse {
  id: number;
  house_id: number;
  fiscal_url: string;
  status: JobStatus;
  attempts: number;
  failure_reason?: string;
  error_detail?: string;
  receipt_id?: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface StoreResponse {
  id: number;
  cnpj: string;
  name: string;
  address?: string;
}

export interface ItemResponse {
  id: number;
  description: string;
  quantity: number;
  unit: string;
  unit_price: number;
  total_price: number;
}

export interface ReceiptResponse {
  id: number;
  house_id: number;
  fiscal_key: string;
  fiscal_url: string;
  issued_at: string;
  total_amount: number;
  store?: StoreResponse;
  items?: ItemResponse[];
  created_at: string;
}

export interface DatabaseJobRow {
  id: number;
  house_id: number;
  submitted_by: number;
  fiscal_url: string;
  status: JobStatus;
  attempts: number;
  failure_reason: string | null;
  error_detail: string | null;
  receipt_id: number | null;
}

export interface DatabaseReceiptRow {
  id: number;
  house_id: number;
  store_id: number;
  fiscal_key: string;
  fiscal_url: string;
  total_amount: number;
}

export interface DatabaseStoreRow {
  id: number;
  cnpj: string;
  name: string;
  address: string | null;
}

export interface DatabaseItemRow {
  id: number;
  receipt_id: number;
  description: string;
  quantity: number;
  unit: string;
  unit_price: number;
  total_price: number;
}
