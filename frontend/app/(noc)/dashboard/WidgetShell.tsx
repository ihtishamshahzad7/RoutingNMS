"use client";

import { useState } from "react";
import { MoreHorizontal, X, Palette } from "lucide-react";
import styles from "./dashboard.module.css";
import {
  useDashboardLayout,
  GRADIENT_PRESETS,
  type DashboardWidgetId,
} from "./dashboardLayout";

/**
 * Shared frame for every dashboard grid module: title + drag handle on the
 * header (native HTML5 drag-and-drop, no extra dependency), and a "..."
 * menu for the two customizations the user asked for -- remove this widget,
 * and tint it with a gradient. Removing only hides it (see dashboardLayout
 * "hidden" list); it stays in the catalog so the toolbar's "Add widget" can
 * bring it back without losing whatever data it was showing.
 */
export function WidgetShell({
  id,
  title,
  children,
  dragState,
  setDragState,
}: {
  id: DashboardWidgetId;
  title: string;
  children: React.ReactNode;
  dragState: { draggedId: DashboardWidgetId | null; overId: DashboardWidgetId | null };
  setDragState: (s: { draggedId: DashboardWidgetId | null; overId: DashboardWidgetId | null }) => void;
}) {
  const [menuOpen, setMenuOpen] = useState<"none" | "menu" | "gradient">("none");
  const removeWidget = useDashboardLayout((s) => s.removeWidget);
  const setGradient = useDashboardLayout((s) => s.setGradient);
  const reorderByDrag = useDashboardLayout((s) => s.reorderByDrag);
  const gradient = useDashboardLayout((s) => s.gradients[id]);

  return (
    <div
      className={styles.widget}
      style={gradient ? { background: gradient } : undefined}
      data-dragging={dragState.draggedId === id}
      data-drop-target={dragState.overId === id && dragState.draggedId !== id}
      onDragOver={(e) => {
        e.preventDefault();
        if (dragState.draggedId && dragState.draggedId !== id) setDragState({ ...dragState, overId: id });
      }}
      onDrop={(e) => {
        e.preventDefault();
        if (dragState.draggedId && dragState.draggedId !== id) reorderByDrag(dragState.draggedId, id);
        setDragState({ draggedId: null, overId: null });
      }}
    >
      <div
        className={styles.widgetHead}
        draggable
        onDragStart={() => setDragState({ draggedId: id, overId: null })}
        onDragEnd={() => setDragState({ draggedId: null, overId: null })}
      >
        <span className={styles.widgetTitle}>{title}</span>
        <div style={{ position: "relative" }}>
          <button
            className={styles.menuBtn}
            aria-label="Widget options"
            onClick={(e) => {
              e.stopPropagation();
              setMenuOpen((v) => (v === "none" ? "menu" : "none"));
            }}
          >
            <MoreHorizontal size={15} />
          </button>
          {menuOpen === "menu" && (
            <div className={styles.menuPanel} onMouseLeave={() => setMenuOpen("none")}>
              <button className={styles.menuItem} onClick={() => setMenuOpen("gradient")}>
                <Palette size={12} /> Set gradient…
              </button>
              <button
                className={styles.menuItemDanger}
                onClick={() => {
                  removeWidget(id);
                  setMenuOpen("none");
                }}
              >
                <X size={12} /> Remove widget
              </button>
            </div>
          )}
          {menuOpen === "gradient" && (
            <div className={styles.menuPanel} onMouseLeave={() => setMenuOpen("none")}>
              {GRADIENT_PRESETS.map((g) => (
                <button
                  key={g.label}
                  className={styles.menuItem}
                  onClick={() => {
                    setGradient(id, g.css);
                    setMenuOpen("none");
                  }}
                >
                  <span className={styles.swatch} style={{ background: g.css ?? "#161B22" }} />
                  {g.label}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      <div className={styles.widgetBody}>{children}</div>
    </div>
  );
}
