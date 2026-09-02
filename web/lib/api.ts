import "server-only";

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080";
const LEARNER_TOKEN = process.env.DEMO_LEARNER_TOKEN ?? "local-learner-token";
const ADMIN_TOKEN = process.env.DEMO_ADMIN_TOKEN ?? "local-admin-token";

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
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    cache: "no-store",
    headers: {
      Authorization: `Bearer ${token}`,
      ...init?.headers,
    },
  });
  if (!response.ok) {
    throw new Error(`Learning Center API returned ${response.status}`);
  }
  return (await response.json()) as T;
}

export async function getCourses(): Promise<Course[]> {
  const body = await apiRequest<{ courses: Course[] }>("/v1/courses", LEARNER_TOKEN);
  return body.courses;
}

export function getDashboard(): Promise<Dashboard> {
  return apiRequest<Dashboard>("/v1/me/dashboard", LEARNER_TOKEN);
}

export async function enroll(courseId: string): Promise<EnrollmentProgress> {
  const body = await apiRequest<{ enrollment: EnrollmentProgress }>(
    `/v1/courses/${courseId}/enrollments`,
    LEARNER_TOKEN,
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
    LEARNER_TOKEN,
    { method: "POST" },
  );
  return body.enrollment;
}

export async function getCompliance(): Promise<ComplianceMember[]> {
  const body = await apiRequest<{ members: ComplianceMember[] }>(
    "/v1/admin/compliance",
    ADMIN_TOKEN,
  );
  return body.members;
}
