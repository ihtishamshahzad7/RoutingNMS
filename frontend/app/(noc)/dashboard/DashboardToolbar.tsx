"use client";

import { useState } from "react";
import { Plus, RotateCcw } from "lucide-react";
import styles from "./dashboard.module.css";
import { useDashboardLayout, WIDGET_CATALOG } from "./dashboardLayout";

export function DashboardToolbar() {
  const [open, setOpen] = useState(false);
  const hidden = useDashboardLayout((s) => s.hidden);
  const addWidget = useDashboardLayout((s) => s.addWidget);
  const resetLayout = useDashboardLayout((s) => s.resetLayout);
  const hiddenCatalog = WIDGET_CATALOG.filter((w) => hidden.includes(w.id));

  return (
    <div className={styles.toolbar}>
      <button className={styles.toolbarBtn} onClick={resetLayout} title="Restore default widgets, order and colors">
        <RotateCcw size={12} /> Reset layout
      </button>
      <button
        className={styles.toolbarBtn}
        onClick={() => setOpen((v) => !v)}
        disabled={hiddenCatalog.length === 0}
        title={hiddenCatalog.length === 0 ? "Every widget is already on the dashboard" : "Add a removed widget back"}
      >
        <Plus size={12} /> Add widget {hiddenCatalog.length > 0 && `(${hiddenCatalog.length})`}
      </button>
      {open && (
        <div className={styles.dropPanel} onMouseLeave={() => setOpen(false)}>
          <div className={styles.dropPanelHint}>Drag a widget's header to reorder it on the grid.</div>
          {hiddenCatalog.map((w) => (
            <button
              key={w.id}
              className={styles.menuItem}
              onClick={() => {
                addWidget(w.id);
                setOpen(false);
              }}
            >
              {w.title}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
