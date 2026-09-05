"use client";

// Shared helper for grouping a device list under named section headers --
// used by the Devices page and the Reachability board so both render
// device groups identically. Devices not assigned to any group render under
// a plain "Ungrouped" section; if there are no groups at all (or every
// device is ungrouped), the caller sees a single "Ungrouped" section, which
// reads fine on its own.

import { ReactNode, useState } from "react";
import { ChevronDown, ChevronRight, FolderTree } from "lucide-react";

export type DeviceGroup = { id: number; name: string; sortOrder: number };
export type GroupMember = { groupId: number; subjectType: string; subjectId: string; sortOrder: number };

export type GroupSection<T> = { key: string; name: string; groupId: number | null; items: T[] };

/** Buckets devices into their assigned group (in group sortOrder, then name)
 * plus a trailing "Ungrouped" bucket -- pass memberOf as {deviceId: groupId}. */
export function groupSections<T extends { id: string }>(
  items: T[],
  groups: DeviceGroup[],
  memberOf: Record<string, number>
): GroupSection<T>[] {
  const byGroup = new Map<number, T[]>();
  const ungrouped: T[] = [];
  for (const item of items) {
    const gid = memberOf[item.id];
    if (gid === undefined) { ungrouped.push(item); continue; }
    const bucket = byGroup.get(gid);
    if (bucket) bucket.push(item); else byGroup.set(gid, [item]);
  }
  const sections: GroupSection<T>[] = [...groups]
    .sort((a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name))
    .filter((g) => byGroup.has(g.id))
    .map((g) => ({ key: `group-${g.id}`, name: g.name, groupId: g.id, items: byGroup.get(g.id)! }));
  if (ungrouped.length || sections.length === 0) {
    sections.push({ key: "ungrouped", name: "Ungrouped", groupId: null, items: ungrouped });
  }
  return sections;
}

/** Renders one collapsible group section as a `<tbody>`-safe fragment: a
 * header row (name + count + collapse toggle) followed by each item's row
 * via `render`, unless the caller has collapsed it. */
export function GroupSectionRows<T extends { id: string }>({
  section, collapsed, onToggle, render, colSpan = 8,
}: {
  section: GroupSection<T>;
  collapsed: boolean;
  onToggle: () => void;
  render: (item: T) => ReactNode;
  colSpan?: number;
}) {
  return (
    <>
      <tr className="border-b border-[#21262D] bg-[#0D1117]/60">
        <td colSpan={colSpan} className="py-1.5">
          <button onClick={onToggle} className="flex w-full items-center gap-2 px-1 text-left text-[10px] font-semibold uppercase tracking-[0.08em] text-[#8B949E] hover:text-[#E6EDF3]">
            {collapsed ? <ChevronRight size={12} /> : <ChevronDown size={12} />}
            <FolderTree size={12} />
            <span>{section.name}</span>
            <span className="font-normal normal-case tracking-normal text-[#484F58]">({section.items.length})</span>
          </button>
        </td>
      </tr>
      {!collapsed && section.items.map((item) => render(item))}
    </>
  );
}

export function useCollapsedSections() {
  return useState<Set<string>>(new Set());
}
