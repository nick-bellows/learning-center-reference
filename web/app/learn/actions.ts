"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { APIRequestError, AuthenticationRequired, completeLesson, enroll } from "@/lib/api";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function requiredUUID(formData: FormData, field: string): string {
  const value = formData.get(field);
  if (typeof value !== "string" || !UUID.test(value)) {
    throw new Error(`Invalid ${field}`);
  }
  return value;
}

export async function enrollAction(formData: FormData): Promise<void> {
  try {
    await enroll(requiredUUID(formData, "courseId"));
  } catch (error) {
    if (error instanceof AuthenticationRequired || (error instanceof APIRequestError && error.status === 401)) {
      redirect("/api/auth/login?returnTo=/learn");
    }
    throw error;
  }
  revalidatePath("/learn");
}

export async function completeLessonAction(formData: FormData): Promise<void> {
  const enrollmentId = requiredUUID(formData, "enrollmentId");
  const lessonId = requiredUUID(formData, "lessonId");
  try {
    await completeLesson(enrollmentId, lessonId);
  } catch (error) {
    if (error instanceof AuthenticationRequired || (error instanceof APIRequestError && error.status === 401)) {
      redirect("/api/auth/login?returnTo=/learn");
    }
    throw error;
  }
  revalidatePath("/learn");
}
