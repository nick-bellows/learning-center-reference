// Runs once when the Next.js server starts, before it accepts requests. The web
// configuration is otherwise read lazily per request, which would turn a bad deployment
// setting (a short SESSION_SECRET, an HTTP URL in public mode) into an error page on every
// route with nothing in the startup log. Validating here fails the process at boot with the
// reason instead.
export async function register() {
  if (process.env.NEXT_RUNTIME !== "nodejs") return;
  const { getWebConfig } = await import("./lib/config");
  const config = getWebConfig();
  console.log(
    JSON.stringify({
      level: "info",
      msg: "web configuration validated",
      deployment: config.deployment,
      authMode: config.authMode,
      apiBaseUrl: config.apiBaseUrl,
      appBaseUrl: config.appBaseUrl,
      secureCookies: config.secureCookies,
    }),
  );
}
