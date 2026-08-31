import Sidebar from "../../components/sidebar";

// Shared chrome for every authenticated NOC page (dashboard, devices, olts,
// incidents, topology). A route group -- "(noc)" is stripped from the URL,
// so /dashboard, /devices etc. are unaffected -- this only adds the
// persistent sidebar around whatever each page renders.
export default function NocLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen bg-slate-950 text-slate-100">
      <Sidebar />
      <div className="min-w-0 flex-1 overflow-x-hidden">{children}</div>
    </div>
  );
}
