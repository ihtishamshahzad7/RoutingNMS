const stats = [
  ["Devices", "0", "Monitored"],
  ["Healthy", "0", "Operational"],
  ["Incidents", "0", "Active"],
  ["Packet Loss", "0%", "Network-wide"],
];

export default function Home() {
  return (
    <main style={{ minHeight: "100vh", padding: 32, maxWidth: 1400, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 13, letterSpacing: 2, opacity: .6 }}>NETWORK OPERATIONS CENTER</div>
          <h1 style={{ margin: "8px 0 0", fontSize: 32 }}>RoutingNMS</h1>
        </div>
        <div style={{ padding: "8px 12px", border: "1px solid #203247", borderRadius: 8, fontSize: 13 }}>System initializing</div>
      </header>

      <section style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 16 }}>
        {stats.map(([label, value, detail]) => (
          <article key={label} style={{ border: "1px solid #203247", borderRadius: 12, padding: 20, background: "#0b1828" }}>
            <div style={{ opacity: .65, fontSize: 13 }}>{label}</div>
            <div style={{ fontSize: 30, fontWeight: 700, margin: "12px 0 4px" }}>{value}</div>
            <div style={{ opacity: .5, fontSize: 12 }}>{detail}</div>
          </article>
        ))}
      </section>

      <section style={{ marginTop: 24, border: "1px solid #203247", borderRadius: 12, padding: 24, background: "#0b1828" }}>
        <h2 style={{ marginTop: 0 }}>NOC overview</h2>
        <p style={{ opacity: .65 }}>Monitoring engine, device discovery, topology, alerts, syslog and AI incident analysis will be connected here.</p>
      </section>
    </main>
  );
}
