import { NextRequest, NextResponse } from "next/server";

// Routes that require an authenticated session. Everything else (the login
// page itself, and any future public route) is left alone.
const PROTECTED_PREFIXES = ["/dashboard", "/devices", "/incidents", "/olts", "/topology", "/syslog", "/traps"];

const SESSION_COOKIE = "routingnms_session";

// This middleware only checks whether the session cookie is *present* — it
// cannot validate the token against the database from the edge runtime.
// Actual validation happens on every API request via auth.Handler.Middleware
// in the Go backend, which is the real security boundary. This layer exists
// purely so an unauthenticated visitor is redirected straight to the login
// page instead of seeing a page shell that then fails to load data.
export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasSession = Boolean(request.cookies.get(SESSION_COOKIE)?.value);

  const isProtected = PROTECTED_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)
  );

  if (isProtected && !hasSession) {
    const loginUrl = new URL("/", request.url);
    return NextResponse.redirect(loginUrl);
  }

  if (pathname === "/" && hasSession) {
    const dashboardUrl = new URL("/dashboard", request.url);
    return NextResponse.redirect(dashboardUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/", "/dashboard/:path*", "/devices/:path*", "/incidents/:path*", "/olts/:path*", "/topology/:path*", "/syslog/:path*", "/traps/:path*"],
};
