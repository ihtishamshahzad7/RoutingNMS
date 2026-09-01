"use client";

// Screen 2 variant — force-directed topology canvas (D3 v7).
// Nodes = discovered devices (sized/colored by health), edges = LLDP links
// (colored by link status). D3 simulation runs a stable force layout with
// drag support; the parent page passes raw graph data and this component owns
// the layout + local selection state.
import { useEffect, useRef, useState } from "react";
import {
  drag,
  forceCenter,
  forceCollide,
  forceLink,
  forceManyBody,
  forceSimulation,
  select,
  type SimulationNodeDatum,
} from "d3";

export type TopoNode = { id: string; name: string; type: string; address?: string; health: number };
export type TopoLink = { id: string; source: string; target: string; status: string; latencyMs?: number; packetLossPct?: number };

type SimNode = SimulationNodeDatum & TopoNode & { color: string };
type SimLink = { id: string; source: SimNode | string | number; target: SimNode | string | number; status: string; latencyMs?: number };

const W = 980;
const H = 540;

function nodeColor(health: number): string {
  if (health >= 90) return "#3FB950";
  if (health >= 70) return "#D29922";
  return "#F78166";
}
function linkColor(status: string): string {
  if (status === "down") return "#F78166";
  if (status === "degraded") return "#D29922";
  return "#3FB950";
}

export function TopologyForce({ nodes, links }: { nodes: TopoNode[]; links: TopoLink[] }) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [selected, setSelected] = useState<TopoNode | null>(null);

  useEffect(() => {
    const svgEl = svgRef.current;
    if (!svgEl) return;

    const simNodes: SimNode[] = nodes.map((n) => ({ ...n, color: nodeColor(n.health) }));
    const byId = new Map(simNodes.map((n) => [n.id, n]));
    const simLinks: SimLink[] = links
      .filter((l) => byId.has(l.source) && byId.has(l.target))
      .map((l) => ({ id: l.id, source: byId.get(l.source)!, target: byId.get(l.target)!, status: l.status, latencyMs: l.latencyMs }));

    const svg = select(svgEl);
    svg.selectAll("*").remove();
    if (simNodes.length === 0) return;

    const sim = forceSimulation(simNodes)
      .force("link", forceLink<SimNode, SimLink>(simLinks).id((d) => d.id).distance(95).strength(0.6))
      .force("charge", forceManyBody().strength(-300))
      .force("center", forceCenter(W / 2, H / 2))
      .force("collide", forceCollide<SimNode>(28));

    // Edges
    svg.append("g")
      .selectAll("line")
      .data(simLinks)
      .join("line")
      .attr("stroke", (d) => linkColor(d.status))
      .attr("stroke-width", (d) => (d.status === "down" ? 2.5 : 1.5))
      .attr("stroke-dasharray", (d) => (d.status === "degraded" ? "7 5" : null))
      .attr("opacity", 0.55);

    // Edge labels (latency)
    svg.append("g")
      .selectAll("text")
      .data(simLinks)
      .join("text")
      .attr("class", "mono")
      .attr("font-size", 9)
      .attr("fill", "#484F58")
      .text((d) => (d.latencyMs ?? 0) > 0 ? `${d.latencyMs}ms` : "");

    // Nodes
    const g = svg.append("g")
      .selectAll<SVGGElement, SimNode>("g")
      .data(simNodes)
      .join("g")
      .style("cursor", "pointer")
      .on("click", (_e, d) => {
        setSelected({ id: d.id, name: d.name, type: d.type, address: d.address, health: d.health });
        sim.alphaTarget(0.1).restart();
      });

    g.append("circle")
      .attr("r", 11)
      .attr("fill", "#0D1117")
      .attr("stroke", (d) => d.color)
      .attr("stroke-width", 1.6)
      .append("title")
      .text((d) => `${d.name} — ${d.type} (health ${d.health}%)`);

    g.append("text")
      .attr("dy", 24)
      .attr("text-anchor", "middle")
      .attr("font-family", "JetBrains Mono, monospace")
      .attr("font-size", 9.5)
      .attr("fill", "#8B949E")
      .text((d) => d.name);

    // Drag
    g.call(
      drag<SVGGElement, SimNode>()
        .on("start", (ev, d) => {
          if (!ev.active) sim.alphaTarget(0.3).restart();
          d.fx = d.x; d.fy = d.y;
        })
        .on("drag", (ev, d) => {
          d.fx = ev.x; d.fy = ev.y;
          sim.alphaTarget(0.1).restart();
        })
        .on("end", (ev, d) => {
          if (!ev.active) sim.alphaTarget(0);
          d.fx = null; d.fy = null;
        })
    );

    // Tick: move lines, latency labels, node groups
    const linkEls = svg.select<SVGGElement>("g:nth-of-type(1)").selectAll<SVGLineElement, SimLink>("line");
    const latencyEls = svg.select<SVGGElement>("g:nth-of-type(2)").selectAll<SVGTextElement, SimLink>("text");
    sim.on("tick", () => {
      linkEls
        .attr("x1", (d) => (d.source as SimNode).x ?? 0).attr("y1", (d) => (d.source as SimNode).y ?? 0)
        .attr("x2", (d) => (d.target as SimNode).x ?? 0).attr("y2", (d) => (d.target as SimNode).y ?? 0);
      latencyEls
        .attr("x", (d) => (((d.source as SimNode).x ?? 0) + ((d.target as SimNode).x ?? 0)) / 2)
        .attr("y", (d) => (((d.source as SimNode).y ?? 0) + ((d.target as SimNode).y ?? 0)) / 2 - 10);
      g.attr("transform", (d) => `translate(${(d.x ?? 0)},${(d.y ?? 0)})`);
    });

    return () => { sim.stop(); };
  }, [nodes, links]);

  const detail = selected ? (
    <div className="pointer-events-none absolute bottom-4 left-4 rounded-[5px] border border-[#30363D] bg-[#1C2128] px-3 py-2 shadow-xl">
      <div className="text-[11px] font-semibold text-[#E6EDF3]">{selected.name}</div>
      <div className="mt-0.5 text-[10px] text-[#8B949E]">
        {selected.type}{selected.address ? ` · ${selected.address}` : ""} · health {selected.health}%
      </div>
    </div>
  ) : null;

  return (
    <div className="relative overflow-hidden rounded-[8px] border border-[#21262D] bg-[#0D1117]">
      <svg ref={svgRef} width="100%" height={H} viewBox={`0 0 ${W} ${H}`} className="block" role="img" aria-label="Network topology graph" />
      {detail}
    </div>
  );
}