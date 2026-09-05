import Sidebar from "../../components/sidebar";
import { TopBar } from "../../components/top-bar";
import VoiceAlerts from "../../components/voice-alerts";

// Shared chrome for every authenticated NOC page. Wraps each page in the
// persistent left sidebar and a 40px top bar (status + UTC clock + alert
// badge), so 24/7 wall-display surfaces are consistent.
export default function NocLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen overflow-hidden bg-[#0D1117] text-[#E6EDF3]">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <div className="min-w-0 flex-1 overflow-y-auto">{children}</div>
      </div>
      <VoiceAlerts />
    </div>
  );
}
