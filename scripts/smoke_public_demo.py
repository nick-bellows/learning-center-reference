"""Read-only smoke checks for a configured recruiter deployment.

This does not create resources, authenticate a user, or mutate demonstration data.
The browser OIDC journey remains a separate manual or environment-specific E2E check.
"""

from __future__ import annotations

import argparse
import json
from urllib.error import HTTPError
from urllib.parse import urljoin, urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener, urlopen


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # type: ignore[no-untyped-def]
        return None


def fetch(url: str) -> tuple[int, dict[str, str], bytes]:
    request = Request(url, headers={"User-Agent": "learning-center-smoke/1.0"})
    try:
        with urlopen(request, timeout=10) as response:
            return response.status, {key.lower(): value for key, value in response.headers.items()}, response.read()
    except HTTPError as error:
        return error.code, {key.lower(): value for key, value in error.headers.items()}, error.read()


def redirect(url: str) -> tuple[int, dict[str, str]]:
    opener = build_opener(NoRedirect)
    request = Request(url, headers={"User-Agent": "learning-center-smoke/1.0"})
    try:
        response = opener.open(request, timeout=10)
        return response.status, {key.lower(): value for key, value in response.headers.items()}
    except HTTPError as error:
        return error.code, {key.lower(): value for key, value in error.headers.items()}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--app-url", required=True)
    parser.add_argument("--api-url", required=True)
    parser.add_argument("--issuer-host", required=True, help="Expected host in the login redirect")
    args = parser.parse_args()

    health_status, _, health_body = fetch(urljoin(args.api_url.rstrip("/") + "/", "health"))
    health = json.loads(health_body)
    assert health_status == 200 and health.get("status") == "ok", "API readiness failed"

    home_status, home_headers, home_body = fetch(args.app_url.rstrip("/") + "/")
    assert home_status == 200, "web landing page failed"
    assert b"Independent portfolio reference implementation" in home_body, "portfolio disclaimer missing"
    assert home_headers.get("x-content-type-options") == "nosniff", "web security headers missing"

    login_status, login_headers = redirect(urljoin(args.app_url.rstrip("/") + "/", "api/auth/login"))
    assert login_status in {302, 303, 307, 308}, "login did not redirect"
    location = login_headers.get("location", "")
    assert urlparse(location).netloc == args.issuer_host, "login redirected to an unexpected issuer host"

    print("PASS: API health, landing page, security headers, and OIDC redirect")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
