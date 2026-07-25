// API client — all server calls route through here.

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type ApiError = {
  code: string;
  message: string;
  fields?: Record<string, string>;
};

export class ApiClientError extends Error {
  constructor(
    public detail: ApiError,
    public status: number,
  ) {
    super(detail.message);
    this.name = "ApiClientError";
  }
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
    ...options,
  });
  if (!res.ok) {
    let err: ApiError = {
      code: "http_error",
      message: `API ${res.status}: ${path}`,
    };
    try {
      err = (await res.json()) as ApiError;
    } catch {
      // fall through with default
    }
    throw new ApiClientError(err, res.status);
  }
  return res.json() as Promise<T>;
}

// --- Auth (future) ---
export const auth = {
  login: (email: string, password: string) =>
    apiFetch<{ token: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  register: (email: string, password: string, name: string) =>
    apiFetch<{ token: string }>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password, name }),
    }),
};

// --- Newsletter ---
export const newsletter = {
  subscribe: (email: string) =>
    apiFetch<{ message: string }>("/newsletter/subscribe", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),
};

// --- Events (future) ---
export const events = {
  list: () => apiFetch<unknown[]>("/events"),
};

// --- Camp registration ----------------------------------------------------

export type CampConfig = {
  name: string;
  location_name: string;
  location_addr: string;
  website_url: string;
  start_date: string;
  end_date: string;
  registrations_open: boolean;
  registration_payment_mode?: "deposit" | "full";
};

export type RegistrationPricing = {
  mode: "deposit" | "full";
  currency: string;
  deposit_amount_pence: number;
  accommodation_tiers: {
    code: string;
    display_name: string;
    amount_pence: number;
  }[];
  child_under3_amount_pence: number;
  day_pass_amount_pence: number;
  coach_amount_pence: number;
};

// v2 collapsed the price catalogue to a single per-full-week-camper deposit.
export type PriceCode = "deposit";

export type Price = {
  code: PriceCode;
  display_name: string;
  amount_pence: number;
  currency: string;
};

export type Accommodation = {
  code: string;
  display_name: string;
  notes?: string;
  available_for_registration: boolean;
};

export type ShirtSize = {
  code: string;
  display_name: string;
  category: "adult" | "child";
};

export type DayCode = "mon" | "tue" | "wed" | "thu" | "fri";

export type FullWeekAttendance = {
  type: "full_week";
  shirt_size: string;
  dietary_requirements: string;
  needs_coach: boolean;
  accommodation_first_choice: string;
  accommodation_second_choice: string;
  roommate_requests: string;
};

export type DayPassAttendance = {
  type: "day_pass";
  days: DayCode[];
  tshirt_option: "team_activities" | "tshirt_only" | "none";
  shirt_size: string;
  needs_catering: boolean;
  dietary_requirements: string;
};

export type CamperSubmission = {
  first_name: string;
  last_name: string;
  gender: "male" | "female";
  age: number;
  cell_leader_name: string;
  is_cell_leader: boolean;
  is_main_contact: boolean;
  attendance: FullWeekAttendance | DayPassAttendance;
};

export type SubmitRequest = {
  contact: {
    first_name: string;
    last_name: string;
    email: string;
    phone: string;
  };
  campers: CamperSubmission[];
  free_code?: string;
};

export type SubmitResponse = {
  group_id: string;
  // Empty when total_amount_pence is 0 (day-pass-only). In that case the
  // backend has already marked the group paid and sent the confirmation email
  // — the frontend should send the user straight to the success page.
  checkout_url: string;
  total_amount_pence: number;
  has_minor: boolean;
  consent_form_url?: string;
};

export type SummaryCamper = {
  first_name: string;
  last_name: string;
};

export type SummaryResponse = {
  group_id: string;
  payment_status: string;
  total_amount_pence: number;
  currency: string;
  contact_email: string;
  campers: SummaryCamper[];
};

export const camp = {
  config: () => apiFetch<CampConfig>("/api/camp"),
  prices: () => apiFetch<{ prices: Price[] }>("/api/prices"),
  registrationPricing: () =>
    apiFetch<RegistrationPricing>("/api/registration-pricing"),
  accommodations: () =>
    apiFetch<{ accommodations: Accommodation[] }>("/api/accommodations"),
  shirtSizes: () =>
    apiFetch<{
      sizes: ShirtSize[];
      by_category: Record<string, ShirtSize[]>;
      not_applicable: string;
    }>("/api/shirt-sizes"),
  submit: (body: SubmitRequest) =>
    apiFetch<SubmitResponse>("/api/registrations", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  // Fetch the public-facing summary of a registration for the success page.
  // Pass either session_id (Stripe path) or group_id (£0 path). Backend
  // returns minimal data: camper first/last names + payment status.
  summary: (params: { sessionId?: string; groupId?: string }) => {
    const q = new URLSearchParams();
    if (params.sessionId) q.set("session_id", params.sessionId);
    if (params.groupId) q.set("group_id", params.groupId);
    return apiFetch<SummaryResponse>(
      `/api/registrations/summary?${q.toString()}`,
    );
  },
};
