"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "../lib/api";

type LoginResponse = { username: string; mustChangePassword: boolean };

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await apiFetch<LoginResponse>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ username: username.trim(), password }),
      });
      // The backend has set an httpOnly session cookie; refresh so the
      // Next.js middleware (which reads that cookie) picks it up before
      // navigating to a protected route.
      router.replace("/dashboard");
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          setError("Invalid username or password.");
        } else {
          setError("The RoutingNMS API rejected the request. Please try again.");
        }
      } else {
        setError("Cannot reach the RoutingNMS backend. Check that the API service is running.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-950 px-4 text-slate-100">
      <section className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-8 shadow-2xl">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-cyan-500/10 text-2xl font-bold text-cyan-400">
            R
          </div>
          <h1 className="text-2xl font-bold">RoutingNMS</h1>
          <p className="mt-2 text-sm text-slate-400">Network Operations Center</p>
        </div>
        <form onSubmit={login} className="space-y-5">
          <div>
            <label htmlFor="username" className="mb-2 block text-sm text-slate-300">
              Username
            </label>
            <input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 outline-none focus:border-cyan-500"
              placeholder="Username"
            />
          </div>
          <div>
            <label htmlFor="password" className="mb-2 block text-sm text-slate-300">
              Password
            </label>
            <input
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type="password"
              autoComplete="current-password"
              className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 outline-none focus:border-cyan-500"
              placeholder="Password"
            />
          </div>
          {error && (
            <div role="alert" className="rounded-lg border border-red-900 bg-red-950/50 px-4 py-3 text-sm text-red-300">
              {error}
            </div>
          )}
          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-lg bg-cyan-500 px-4 py-3 font-semibold text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? "Signing in…" : "Sign in"}
          </button>
        </form>
        <div className="mt-6 rounded-lg border border-slate-800 bg-slate-950 p-4 text-xs text-slate-400">
          <div className="font-semibold text-slate-300">Default access (first login only)</div>
          <div className="mt-2">
            Username: <span className="text-cyan-400">admin</span>
          </div>
          <div>
            Password: <span className="text-cyan-400">admin123</span>
          </div>
          <div className="mt-2 text-amber-400">This is the initial password. Change it immediately after signing in.</div>
        </div>
      </section>
    </main>
  );
}
