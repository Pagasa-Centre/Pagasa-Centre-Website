// Admin API client.
//
// Auth uses a bearer token stored in localStorage rather than a cookie. The
// dashboard and the API live on different domains, so a session cookie is
// "third-party" and gets silently blocked by incognito mode and mobile Safari.
// Sending the token in the Authorization header works everywhere.

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const TOKEN_KEY = "pc_admin_token";

export function getAdminToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

function setToken(token: string | null) {
  if (typeof window === "undefined") return;
  if (token) {
    window.localStorage.setItem(TOKEN_KEY, token);
  } else {
    window.localStorage.removeItem(TOKEN_KEY);
  }
}

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
  const token = getAdminToken();
  const res = await fetch(`${API_BASE}${path}`, {
    cache: "no-store",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
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

export type AdminAccommodationUnit = {
  code: string;
  accommodation_code: string;
  display_name: string;
  capacity: number;
  sort_order: number;
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
  allocated_unit_code?: string | null;
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
  created_at: string;
  invoice_due_at?: string | null;
  balance_paid_at?: string | null;
  version: number;
  last_action?: string | null;
  last_action_by?: string | null;
  last_action_at?: string | null;
  is_free: boolean;
  campers: AdminCamper[];
};

export type FreeCode = {
  id: string;
  code: string;
  created_at: string;
  created_by: string;
  note?: string | null;
  used_at?: string | null;
  used_by_group_id?: string | null;
  revoked_at?: string | null;
};

export type AdminEvent = {
  id: number;
  created_at: string;
  actor_name: string;
  action: string;
  group_id?: string | null;
  summary: string;
  metadata?: unknown;
};

export type AllocateCamper = {
  camper_id: string;
  allocated_accommodation_code: string;
  allocated_unit_code?: string;
  billed_stripe_price_id?: string;
};

export type AdminCampConfig = {
  name: string;
  location_name: string;
  location_addr: string;
  website_url: string;
  start_date: string;
  end_date: string;
  registrations_open: boolean;
};

/** Opens the admin SSE stream. Caller must close on unmount. */
export function openAdminEventStream(handlers: {
  onEvent: (ev: AdminEvent) => void;
  onOpen?: () => void;
  onError?: () => void;
}): EventSource | null {
  const token = getAdminToken();
  if (!token) return null;
  const url = `${API_BASE}/admin/stream?token=${encodeURIComponent(token)}`;
  const es = new EventSource(url);
  es.addEventListener("changed", (msg) => {
    try {
      handlers.onEvent(JSON.parse(msg.data) as AdminEvent);
    } catch {
      // ignore malformed
    }
  });
  es.onopen = () => handlers.onOpen?.();
  es.onerror = () => handlers.onError?.();
  return es;
}

export const adminApi = {
  login: async (password: string, firstName: string, lastName: string) => {
    const res = await adminFetch<{ token?: string; name?: string }>(
      "/admin/login",
      {
        method: "POST",
        body: JSON.stringify({
          password,
          first_name: firstName,
          last_name: lastName,
        }),
      },
    );
    if (res?.token) {
      setToken(res.token);
    }
    return res?.name ?? null;
  },

  logout: async () => {
    try {
      await adminFetch<void>("/admin/logout", { method: "POST" });
    } finally {
      setToken(null);
    }
  },

  checkSession: () =>
    adminFetch<{ name: string }>("/admin/session", { method: "GET" }),

  listEvents: (params?: { limit?: number; before?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.before) q.set("before", String(params.before));
    const qs = q.toString();
    return adminFetch<{ events: AdminEvent[] }>(
      `/admin/events${qs ? `?${qs}` : ""}`,
    );
  },

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

  updateContact: (
    groupId: string,
    data: {
      first_name: string;
      last_name: string;
      email: string;
      phone: string;
      resend_confirmation: boolean;
    },
  ) =>
    adminFetch<void>(`/admin/registrations/${groupId}/contact`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),

  accommodations: () =>
    adminFetch<{ accommodations: AdminAccommodation[] }>(
      "/admin/accommodations",
    ),

  accommodationUnits: () =>
    adminFetch<{ units: AdminAccommodationUnit[] }>(
      "/admin/accommodation-units",
    ),

  saveAllocation: (
    groupId: string,
    campers: AllocateCamper[],
    expectedVersion: number,
  ) =>
    adminFetch<void>(`/admin/registrations/${groupId}/allocation`, {
      method: "PUT",
      body: JSON.stringify({ campers, expected_version: expectedVersion }),
    }),

  unallocate: (groupId: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/unallocate`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  sendInvoice: (groupId: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/invoice`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  sendInvoiceBulk: (groupIds: string[]) =>
    adminFetch<void | { errors: Record<string, string> }>(
      "/admin/registrations/invoice-bulk",
      {
        method: "POST",
        body: JSON.stringify({ group_ids: groupIds }),
      },
    ),

  release: (groupId: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/release`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  deleteRegistration: (groupId: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/delete`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  resendInvoice: (groupId: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/invoice/resend`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  extendDue: (groupId: string, dueAt: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/invoice-due`, {
      method: "PATCH",
      body: JSON.stringify({
        due_at: dueAt,
        expected_version: expectedVersion,
      }),
    }),

  sweep: () =>
    adminFetch<{ released: number }>("/admin/billing/sweep", {
      method: "POST",
      body: JSON.stringify({}),
    }),

  campConfig: () => adminFetch<AdminCampConfig>("/admin/camp-config"),

  setRegistrationsOpen: (open: boolean) =>
    adminFetch<{ registrations_open: boolean }>("/admin/registrations-open", {
      method: "PUT",
      body: JSON.stringify({ open }),
    }),

  confirmFree: (groupId: string, expectedVersion: number) =>
    adminFetch<void>(`/admin/registrations/${groupId}/confirm-free`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  generateFreeCode: (password: string, note?: string) =>
    adminFetch<{ code: string }>("/admin/free-codes", {
      method: "POST",
      body: JSON.stringify({ password, note: note ?? "" }),
    }),

  listFreeCodes: () =>
    adminFetch<{ codes: FreeCode[] }>("/admin/free-codes"),

  revokeFreeCode: (id: string) =>
    adminFetch<void>(`/admin/free-codes/${id}/revoke`, { method: "POST" }),
};
