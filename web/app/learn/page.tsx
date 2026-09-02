import { completeLessonAction, enrollAction } from "./actions";
import { getCourses, getDashboard } from "@/lib/api";

export default async function LearnPage() {
  let data;
  try {
    const [courses, dashboard] = await Promise.all([getCourses(), getDashboard()]);
    data = { courses, dashboard };
  } catch {
    return (
      <main className="page-shell">
        <p className="eyebrow">Learner workspace</p>
        <h1>Course service unavailable</h1>
        <p className="lede">Start the local API and PostgreSQL services, then reload this page.</p>
      </main>
    );
  }

  const enrolledCourseIds = new Set(data.dashboard.enrollments.map((item) => item.course_id));
  const availableCourses = data.courses.filter((course) => !enrolledCourseIds.has(course.id));

  return (
    <main className="page-shell space-y-10">
      <section className="hero-panel">
        <div>
          <p className="eyebrow">Learner workspace</p>
          <h1>Welcome back, {data.dashboard.member.display_name}</h1>
          <p className="lede max-w-2xl">
            Your identity was verified at the API boundary and the learner role below was
            resolved from PostgreSQL—not accepted from the request.
          </p>
        </div>
        <span className="role-chip">learner</span>
      </section>

      {data.dashboard.enrollments.map((enrollment) => {
        let earlierLessonsComplete = true;
        return (
          <section className="card" key={enrollment.enrollment_id} aria-labelledby={`course-${enrollment.course_id}`}>
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="eyebrow">Current course</p>
                <h2 id={`course-${enrollment.course_id}`}>{enrollment.course_title}</h2>
                <p className="mt-2 text-sm text-slate-600">
                  {enrollment.completed_lessons} of {enrollment.total_lessons} lessons complete
                </p>
              </div>
              <span className={enrollment.status === "completed" ? "status status-good" : "status status-active"}>
                {enrollment.status}
              </span>
            </div>

            <div className="mt-5">
              <div className="mb-2 flex justify-between text-sm font-medium">
                <span>Progress</span>
                <span>{enrollment.percent_complete}%</span>
              </div>
              <div
                className="progress-track"
                role="progressbar"
                aria-label={`${enrollment.course_title} progress`}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={enrollment.percent_complete}
              >
                <div className="progress-fill" style={{ width: `${enrollment.percent_complete}%` }} />
              </div>
            </div>

            <ol className="mt-6 space-y-3">
              {enrollment.lessons.map((lesson) => {
                const canComplete = !lesson.completed && earlierLessonsComplete;
                if (!lesson.completed) earlierLessonsComplete = false;
                return (
                  <li className="lesson-row" key={lesson.id}>
                    <div className="flex items-start gap-3">
                      <span className={lesson.completed ? "lesson-number lesson-done" : "lesson-number"} aria-hidden="true">
                        {lesson.completed ? "✓" : lesson.position}
                      </span>
                      <div>
                        <h3>{lesson.title}</h3>
                        <p className="text-sm capitalize text-slate-500">{lesson.type}</p>
                      </div>
                    </div>
                    {lesson.completed ? (
                      <span className="text-sm font-medium text-emerald-700">Complete</span>
                    ) : canComplete ? (
                      <form action={completeLessonAction}>
                        <input type="hidden" name="enrollmentId" value={enrollment.enrollment_id} />
                        <input type="hidden" name="lessonId" value={lesson.id} />
                        <button className="button button-secondary" type="submit">Mark complete</button>
                      </form>
                    ) : (
                      <span className="text-sm text-slate-500">Complete in order</span>
                    )}
                  </li>
                );
              })}
            </ol>
          </section>
        );
      })}

      {availableCourses.length > 0 && (
        <section aria-labelledby="catalog-title">
          <p className="eyebrow">Course catalog</p>
          <h2 id="catalog-title">Published courses</h2>
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            {availableCourses.map((course) => (
              <article className="card" key={course.id}>
                <h3>{course.title}</h3>
                <p className="mt-2 text-sm text-slate-600">
                  {course.lesson_count} lessons · {course.ordering} order
                </p>
                <form action={enrollAction} className="mt-5">
                  <input type="hidden" name="courseId" value={course.id} />
                  <button className="button button-primary" type="submit">Enroll in course</button>
                </form>
              </article>
            ))}
          </div>
        </section>
      )}

      {data.dashboard.enrollments.length === 0 && availableCourses.length === 0 && (
        <p className="card text-slate-600">No published courses are available.</p>
      )}
    </main>
  );
}
