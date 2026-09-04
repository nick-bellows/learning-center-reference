import "server-only";

import { connection } from "next/server";
import { getWebConfig } from "./config";
import { readSession } from "./session";

export class AuthenticationRequired extends Error {}

export class APIRequestError extends Error {
  constructor(public readonly status: number) {
    super(`Learning Center API returned ${status}`);
  }
}

export type Course = {
  id: string;
  title: string;
  slug: string;
  ordering: "sequential" | "open";
  lesson_count: number;
};

export type LessonProgress = {
  id: string;
  title: string;
  type: "video" | "reading" | "quiz";
  position: number;
  completed: boolean;
  completed_at?: string;
};

export type EnrollmentProgress = {
  enrollment_id: string;
  course_id: string;
  course_title: string;
  status: "active" | "completed" | "withdrawn";
  completed_lessons: number;
  total_lessons: number;
  percent_complete: number;
  lessons: LessonProgress[];
};

export type Dashboard = {
  member: { id: string; display_name: string; roles: string[] };
  enrollments: EnrollmentProgress[];
};

export type ComplianceMember = {
  id: string;
  display_name: string;
  roles: string[];
  status: "eligible" | "suspended" | "ineligible_lapsed";
  reason: string;
  next_expiration?: string;
};

async function apiRequest<T>(path: string, token: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${getWebConfig().apiBaseUrl}${path}`, {
    ...init,
    cache: "no-store",
    headers: {
      Authorization: `Bearer ${token}`,
      ...init?.headers,
    },
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }
  return (await response.json()) as T;
}

async function accessToken(demoRole: "learner" | "admin"): Promise<string> {
  const config = getWebConfig();
  if (config.authMode === "demo") {
    return demoRole === "learner"
      ? process.env.DEMO_LEARNER_TOKEN ?? "local-learner-token"
      : process.env.DEMO_ADMIN_TOKEN ?? "local-admin-token";
  }
  const session = await readSession();
  if (!session) throw new AuthenticationRequired();
  return session.accessToken;
}

export async function getViewerState(): Promise<{
  authMode: "demo" | "oidc";
  signedIn: boolean;
  subject?: string;
}> {
  // The header nav depends on request state (deployment auth mode and, in OIDC mode, the
  // session cookie), so it must never be baked into a static prerender. Without this, a
  // build with no auth env set freezes "local demo" into the landing and error pages.
  await connection();
  const config = getWebConfig();
  const session = config.authMode === "oidc" ? await readSession() : null;
  return { authMode: config.authMode, signedIn: config.authMode === "demo" || Boolean(session), subject: session?.subject };
}

export async function getCourses(): Promise<Course[]> {
  const body = await apiRequest<{ courses: Course[] }>("/v1/courses", await accessToken("learner"));
  return body.courses;
}

export function getDashboard(): Promise<Dashboard> {
  return accessToken("learner").then((token) => apiRequest<Dashboard>("/v1/me/dashboard", token));
}

export async function enroll(courseId: string): Promise<EnrollmentProgress> {
  const body = await apiRequest<{ enrollment: EnrollmentProgress }>(
    `/v1/courses/${courseId}/enrollments`,
    await accessToken("learner"),
    { method: "POST" },
  );
  return body.enrollment;
}

export async function completeLesson(
  enrollmentId: string,
  lessonId: string,
): Promise<EnrollmentProgress> {
  const body = await apiRequest<{ enrollment: EnrollmentProgress }>(
    `/v1/enrollments/${enrollmentId}/lessons/${lessonId}/complete`,
    await accessToken("learner"),
    { method: "POST" },
  );
  return body.enrollment;
}

export async function getCompliance(): Promise<ComplianceMember[]> {
  const body = await apiRequest<{ members: ComplianceMember[] }>(
    "/v1/admin/compliance",
    await accessToken("admin"),
  );
  return body.members;
}
