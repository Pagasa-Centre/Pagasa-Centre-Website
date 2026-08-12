// Camp admin API client.
//
// Auth uses a bearer token stored in localStorage rather than a cookie. The
// dashboard and the API live on different domains, so a session cookie is
// "third-party" and gets silently blocked by incognito mode and mobile Safari.
// Sending the token in the Authorization header works everywhere.

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const CAMP_ADMIN_API_PREFIX = "/camp-admin";

const TOKEN_KEY = "pc_admin_token";

export function getCampAdminToken(): string | null {
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

export class CampAdminApiError extends Error {
  constructor(
    public detail: ApiError,
    public status: number,
  ) {
    super(detail.message);
    this.name = "CampAdminApiError";
  }
}

async function campAdminFetch<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const token = getCampAdminToken();
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
    throw new CampAdminApiError(err, res.status);
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
  available_for_registration?: boolean;
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
  is_main_contact?: boolean;
  first_name: string;
  last_name: string;
  attendance_type: string;
  age: number;
  needs_coach?: boolean | null;
  accommodation_first_choice?: string | null;
  accommodation_second_choice?: string | null;
  day_pass_days?: string[] | null;
  day_pass_tshirt_option?: string | null;
  day_pass_needs_catering?: boolean | null;
  shirt_size?: string | null;
  dietary_requirements?: string | null;
  deposit_credit_pence?: number;
  deposit_owed_pence?: number;
  cell_leader_name?: string;
  is_cell_leader?: boolean;
  gender?: string;
  allocated_accommodation_code?: string | null;
  allocated_unit_code?: string | null;
  billed_stripe_price_id?: string | null;
};

// EditCamperPayload replaces a camper's details wholesale — every field is sent
// every time, matching what the endpoint expects.
export type EditCamperPayload = {
  first_name: string;
  last_name: string;
  gender: string;
  age: number;
  cell_leader_name: string;
  is_cell_leader: boolean;
  shirt_size: string;
  dietary_requirements: string;
  accommodation_first_choice: string;
  accommodation_second_choice: string;
  allocated_accommodation_code: string;
  allocated_unit_code: string;
};

export type NewCamperPayload = {
  first_name: string;
  last_name: string;
  gender: string;
  age: number;
  cell_leader_name: string;
  is_cell_leader: boolean;
  attendance: {
    type: "full_week" | "day_pass";
    shirt_size?: string;
    dietary_requirements?: string;
    needs_coach?: boolean;
    accommodation_first_choice?: string;
    accommodation_second_choice?: string;
    days?: string[];
    tshirt_option?: string;
    needs_catering?: boolean;
  };
};

export type EditCamperResult = {
  camper_name: string;
  previous_name: string;
  invoice_voided: boolean;
  repriced: boolean;
};

export type AddCamperResult = {
  camper_id: string;
  camper_name: string;
  deposit_owed_pence: number;
  invoice_voided: boolean;
  needs_allocation: boolean;
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
  coach_included_in_balance?: boolean;
  stripe_coach_invoice_id?: string | null;
  coach_invoice_due_at?: string | null;
  coach_fee_paid_at?: string | null;
  coach_fee_waived_at?: string | null;
  paid_in_full_at_registration?: boolean;
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
  registration_payment_mode: "deposit" | "full";
};

/** Opens the admin SSE stream. Caller must close on unmount. */
export function openCampAdminEventStream(handlers: {
  onEvent: (ev: AdminEvent) => void;
  onOpen?: () => void;
  onError?: () => void;
}): EventSource | null {
  const token = getCampAdminToken();
  if (!token) return null;
  const url = `${API_BASE}${CAMP_ADMIN_API_PREFIX}/stream?token=${encodeURIComponent(token)}`;
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

export const campAdminApi = {
  login: async (password: string, firstName: string, lastName: string) => {
    const res = await campAdminFetch<{ token?: string; name?: string }>(
      `${CAMP_ADMIN_API_PREFIX}/login`,
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
      await campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/logout`, { method: "POST" });
    } finally {
      setToken(null);
    }
  },

  checkSession: () =>
    campAdminFetch<{ name: string }>(`${CAMP_ADMIN_API_PREFIX}/session`, { method: "GET" }),

  listEvents: (params?: { limit?: number; before?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.before) q.set("before", String(params.before));
    const qs = q.toString();
    return campAdminFetch<{ events: AdminEvent[] }>(
      `${CAMP_ADMIN_API_PREFIX}/events${qs ? `?${qs}` : ""}`,
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
    return campAdminFetch<{ groups: AdminGroup[] }>(
      `${CAMP_ADMIN_API_PREFIX}/registrations${qs ? `?${qs}` : ""}`,
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
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/contact`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),

  accommodations: () =>
    campAdminFetch<{ accommodations: AdminAccommodation[] }>(
      `${CAMP_ADMIN_API_PREFIX}/accommodations`,
    ),

  accommodationUnits: () =>
    campAdminFetch<{ units: AdminAccommodationUnit[] }>(
      `${CAMP_ADMIN_API_PREFIX}/accommodation-units`,
    ),

  setAccommodationAvailability: (code: string, available: boolean) =>
    campAdminFetch<{ code: string; available_for_registration: boolean }>(
      `${CAMP_ADMIN_API_PREFIX}/accommodations/${code}/availability`,
      { method: "PUT", body: JSON.stringify({ available }) },
    ),

  saveAllocation: (
    groupId: string,
    campers: AllocateCamper[],
    expectedVersion: number,
  ) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/allocation`, {
      method: "PUT",
      body: JSON.stringify({ campers, expected_version: expectedVersion }),
    }),

  unallocate: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/unallocate`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  sendInvoice: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/invoice`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  sendInvoiceBulk: (groupIds: string[]) =>
    campAdminFetch<void | { errors: Record<string, string> }>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/invoice-bulk`,
      {
        method: "POST",
        body: JSON.stringify({ group_ids: groupIds }),
      },
    ),

  sendCoachInvoice: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/coach-invoice`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  sendCoachInvoiceBulk: (groupIds: string[]) =>
    campAdminFetch<void | { errors: Record<string, string> }>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/coach-invoice-bulk`,
      {
        method: "POST",
        body: JSON.stringify({ group_ids: groupIds }),
      },
    ),

  waiveCoachFee: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/coach-invoice/waive`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  unwaiveCoachFee: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/coach-invoice/unwaive`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  release: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/release`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  cancel: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/cancel`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  deleteRegistration: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/delete`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  removeCamper: (groupId: string, camperId: string, expectedVersion: number) =>
    campAdminFetch<void>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}/delete`,
      {
        method: "POST",
        body: JSON.stringify({ expected_version: expectedVersion }),
      },
    ),

  convertToDayVisitor: (
    groupId: string,
    camperId: string,
    data: {
      days: string[];
      tshirt_option: string;
      shirt_size: string;
      needs_catering: boolean;
    },
    expectedVersion: number,
  ) =>
    campAdminFetch<void>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}/convert-day-visitor`,
      {
        method: "POST",
        body: JSON.stringify({
          expected_version: expectedVersion,
          ...data,
        }),
      },
    ),

  updateDayPassCamper: (
    groupId: string,
    camperId: string,
    data: {
      tshirt_option: string;
      shirt_size: string;
      needs_catering: boolean;
      dietary_requirements: string;
    },
    expectedVersion: number,
  ) =>
    campAdminFetch<void>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}/day-pass`,
      {
        method: "PATCH",
        body: JSON.stringify({
          expected_version: expectedVersion,
          ...data,
        }),
      },
    ),

  updateCamperCoach: (
    groupId: string,
    camperId: string,
    needsCoach: boolean,
    expectedVersion: number,
  ) =>
    campAdminFetch<void>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}/coach`,
      {
        method: "PATCH",
        body: JSON.stringify({
          expected_version: expectedVersion,
          needs_coach: needsCoach,
        }),
      },
    ),

  editCamper: (
    groupId: string,
    camperId: string,
    data: EditCamperPayload,
    expectedVersion: number,
  ) =>
    campAdminFetch<EditCamperResult>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}`,
      {
        method: "PATCH",
        body: JSON.stringify({ expected_version: expectedVersion, ...data }),
      },
    ),

  addCamper: (
    groupId: string,
    camper: NewCamperPayload,
    expectedVersion: number,
  ) =>
    campAdminFetch<AddCamperResult>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion, camper }),
    }),

  makeMainContact: (
    groupId: string,
    camperId: string,
    expectedVersion: number,
  ) =>
    campAdminFetch<void>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}/make-main-contact`,
      {
        method: "POST",
        body: JSON.stringify({ expected_version: expectedVersion }),
      },
    ),

  waiveCamperDeposit: (
    groupId: string,
    camperId: string,
    expectedVersion: number,
  ) =>
    campAdminFetch<void>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/campers/${camperId}/waive-deposit`,
      {
        method: "POST",
        body: JSON.stringify({ expected_version: expectedVersion }),
      },
    ),

  resendInvoice: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/invoice/resend`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  resyncAllSheets: () =>
    campAdminFetch<{ synced: number; errors?: Record<string, string> }>(
      `${CAMP_ADMIN_API_PREFIX}/registrations/sheet-resync-all`,
      {
        method: "POST",
        body: JSON.stringify({}),
      },
    ),

  extendDue: (groupId: string, dueAt: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/invoice-due`, {
      method: "PATCH",
      body: JSON.stringify({
        due_at: dueAt,
        expected_version: expectedVersion,
      }),
    }),

  sweep: () =>
    campAdminFetch<{ released: number }>(`${CAMP_ADMIN_API_PREFIX}/billing/sweep`, {
      method: "POST",
      body: JSON.stringify({}),
    }),

  campConfig: () => campAdminFetch<AdminCampConfig>(`${CAMP_ADMIN_API_PREFIX}/camp-config`),

  setRegistrationsOpen: (open: boolean) =>
    campAdminFetch<{ registrations_open: boolean }>(`${CAMP_ADMIN_API_PREFIX}/registrations-open`, {
      method: "PUT",
      body: JSON.stringify({ open }),
    }),

  setRegistrationPaymentMode: (mode: "deposit" | "full") =>
    campAdminFetch<{ registration_payment_mode: string }>(
      `${CAMP_ADMIN_API_PREFIX}/registration-payment-mode`,
      {
        method: "PUT",
        body: JSON.stringify({ mode }),
      },
    ),

  confirmFree: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/confirm-free`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  markPaid: (groupId: string, expectedVersion: number) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/registrations/${groupId}/mark-paid`, {
      method: "POST",
      body: JSON.stringify({ expected_version: expectedVersion }),
    }),

  generateFreeCode: (password: string, note?: string) =>
    campAdminFetch<{ code: string }>(`${CAMP_ADMIN_API_PREFIX}/free-codes`, {
      method: "POST",
      body: JSON.stringify({ password, note: note ?? "" }),
    }),

  listFreeCodes: () =>
    campAdminFetch<{ codes: FreeCode[] }>(`${CAMP_ADMIN_API_PREFIX}/free-codes`),

  revokeFreeCode: (id: string) =>
    campAdminFetch<void>(`${CAMP_ADMIN_API_PREFIX}/free-codes/${id}/revoke`, { method: "POST" }),
};
