import "server-only";

import {
  createCipheriv,
  createDecipheriv,
  createHash,
  randomBytes,
  timingSafeEqual,
} from "node:crypto";
import { cookies } from "next/headers";
import { getWebConfig } from "./config";

const SESSION_COOKIE = "lcr_session";
const TRANSACTION_COOKIE = "lcr_oidc_transaction";

export type Session = { accessToken: string; subject: string; expiresAt: number };
export type OIDCTransaction = {
  state: string;
  nonce: string;
  verifier: string;
  returnTo: string;
  expiresAt: number;
};

function key(): Buffer {
  return createHash("sha256").update(getWebConfig().sessionSecret, "utf8").digest();
}

function seal(value: object): string {
  const iv = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", key(), iv);
  const ciphertext = Buffer.concat([
    cipher.update(JSON.stringify(value), "utf8"),
    cipher.final(),
  ]);
  return [iv, cipher.getAuthTag(), ciphertext].map((part) => part.toString("base64url")).join(".");
}

function open<T>(value: string | undefined): T | null {
  if (!value) return null;
  try {
    const [iv, tag, ciphertext] = value.split(".").map((part) => Buffer.from(part, "base64url"));
    if (!iv || !tag || !ciphertext) return null;
    const decipher = createDecipheriv("aes-256-gcm", key(), iv);
    decipher.setAuthTag(tag);
    return JSON.parse(
      Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString("utf8"),
    ) as T;
  } catch {
    return null;
  }
}

function cookieOptions(maxAge: number) {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: getWebConfig().secureCookies,
    path: "/",
    maxAge,
  };
}

export async function setSession(session: Session): Promise<void> {
  const maxAge = Math.max(1, Math.floor(session.expiresAt - Date.now() / 1000));
  (await cookies()).set(SESSION_COOKIE, seal(session), cookieOptions(maxAge));
}

export async function readSession(): Promise<Session | null> {
  const session = open<Session>((await cookies()).get(SESSION_COOKIE)?.value);
  if (!session || !session.accessToken || !session.subject || session.expiresAt <= Date.now() / 1000) {
    return null;
  }
  return session;
}

export async function clearSession(): Promise<void> {
  (await cookies()).delete(SESSION_COOKIE);
}

export async function setTransaction(transaction: OIDCTransaction): Promise<void> {
  (await cookies()).set(TRANSACTION_COOKIE, seal(transaction), cookieOptions(600));
}

export async function takeTransaction(): Promise<OIDCTransaction | null> {
  const store = await cookies();
  const transaction = open<OIDCTransaction>(store.get(TRANSACTION_COOKIE)?.value);
  store.delete(TRANSACTION_COOKIE);
  if (!transaction || transaction.expiresAt <= Date.now() / 1000) return null;
  return transaction;
}

export function valuesMatch(left: string, right: string): boolean {
  const a = Buffer.from(left);
  const b = Buffer.from(right);
  return a.length === b.length && timingSafeEqual(a, b);
}
