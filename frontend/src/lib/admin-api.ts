// Admin API client — uses session cookies (credentials: include).

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type ApiError = {
  code: string;
  message: string;
  fields?: Record<string, string>;
};

export class AdminApiError extends Error {
  constructor(
    public detail: ApiError,
    public status: number,
  ) {
    super(detail.message);
    this.name = "AdminApiError";
  }
}

async function adminFetch<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    cache: "no-store",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
    ...options,
  });
  if (res.status === 204) {
    return undefined as T;
  }
  if (!res.ok) {
    let err: ApiError = {
      code: "http_error",
      message: `Admin API ${res.status}: ${path}`,
    };
    try {
      err = (await res.json()) as ApiError;
    } catch {
      // fall through
    }
    throw new AdminApiError(err, res.status);
  }
  if (res.headers.get("content-type")?.includes("application/json")) {
    return res.json() as Promise<T>;
  }
  return undefined as T;
}

export type AdminAccommodation = {
  code: string;
  display_name: string;
  capacity?: number | null;
  stripe_price_id?: string | null;
};

export type AdminCamper = {
  id: string;
  group_id: string;
  first_name: string;
  last_name: string;
  attendance_type: string;
  age: number;
  accommodation_first_choice?: string | null;
  accommodation_second_choice?: string | null;
  allocated_accommodation_code?: string | null;
  billed_stripe_price_id?: string | null;
};

export type AdminGroup = {
  id: string;
  contact_first_name: string;
  contact_last_name: string;
  contact_email: string;
  contact_phone: string;
  payment_status: string;
  billing_status: string;
  total_amount_pence: number;
  currency: string;
  invoice_due_at?: string | null;
  balance_paid_at?: string | null;
  campers: AdminCamper[];
};

export type AllocateCamper = {
  camper_id: string;
  allocated_accommodation_code: string;
  billed_stripe_price_id?: string;
};

export const adminApi = {
  login: (password: string) =>
    adminFetch<void>("/admin/login", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),

  logout: () =>
    adminFetch<void>("/admin/logout", { method: "POST" }),

  checkSession: () =>
    adminFetch<void>("/admin/session", { method: "GET" }),

  listRegistrations: (params?: {
    status?: string;
    billing_status?: string;
  }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.billing_status) q.set("billing_status", params.billing_status);
    const qs = q.toString();
    return adminFetch<{ groups: AdminGroup[] }>(
      `/admin/registrations${qs ? `?${qs}` : ""}`,
    );
  },

  accommodations: () =>
    adminFetch<{ accommodations: AdminAccommodation[] }>(
      "/admin/accommodations",
    ),

  saveAllocation: (groupId: string, campers: AllocateCamper[]) =>
    adminFetch<void>(`/admin/registrations/${groupId}/allocation`, {
      method: "PUT",
      body: JSON.stringify({ campers }),
    }),

  unallocate: (groupId: string) =>
    adminFetch<void>(`/admin/registrations/${groupId}/unallocate`, {
      method: "POST",
    }),

  sendInvoice: (groupId: string) =>
    adminFetch<void>(`/admin/registrations/${groupId}/invoice`, {
      method: "POST",
    }),

  sendInvoiceBulk: (groupIds: string[]) =>
    adminFetch<void | { errors: Record<string, string> }>(
      "/admin/registrations/invoice-bulk",
      {
        method: "POST",
        body: JSON.stringify({ group_ids: groupIds }),
      },
    ),

  release: (groupId: string) =>
    adminFetch<void>(`/admin/registrations/${groupId}/release`, {
      method: "POST",
    }),

  resendInvoice: (groupId: string) =>
    adminFetch<void>(`/admin/registrations/${groupId}/invoice/resend`, {
      method: "POST",
    }),

  extendDue: (groupId: string, dueAt: string) =>
    adminFetch<void>(`/admin/registrations/${groupId}/invoice-due`, {
      method: "PATCH",
      body: JSON.stringify({ due_at: dueAt }),
    }),

  sweep: () =>
    adminFetch<{ released: number }>("/admin/billing/sweep", {
      method: "POST",
    }),
};
