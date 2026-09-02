import { NextRequest, NextResponse } from "next/server";
import { getWebConfig } from "@/lib/config";
import { clearSession } from "@/lib/session";

export async function POST(request: NextRequest) {
  await clearSession();
  const config = getWebConfig();
  return NextResponse.redirect(new URL("/", config.appBaseUrl || request.url));
}
