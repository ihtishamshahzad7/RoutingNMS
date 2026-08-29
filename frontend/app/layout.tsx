import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "RoutingNMS",
  description: "AI-powered Network Operations Management System",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
