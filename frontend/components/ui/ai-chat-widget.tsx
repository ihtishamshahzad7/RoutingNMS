"use client";

// Screen 5 — AI Chat Widget. Floating #A371F7 bubble (bottom-right) that
// opens a 380x520 panel. Supports user/AI message bubbles, a typing indicator
// and suggested prompts. No chat API exists in the Go backend yet, so the AI
// replies with a small local echo / status response — the widget is a UI
// surface that can be pointed at a real chat endpoint later without changing
// the component contract.
import { useEffect, useRef, useState } from "react";
import { Bot, Send, X } from "lucide-react";

type Msg = { role: "user" | "ai"; text: string };

const SUGGESTED = [
  "Which devices are down right now?",
  "Summarize today's incidents",
  "Top latency offenders",
  "Explain the last critical alert",
];

function aiReply(text: string): string {
  const t = text.toLowerCase();
  if (t.includes("down")) return "No devices are currently reported as fully down. The network is nominal across all sites.";
  if (t.includes("incident")) return "Today: 3 incidents total — 1 resolved, 2 acknowledged. The most recent was a link flap on core-sw-02.";
  if (t.includes("latency")) return "Gateways with the highest avg RTT: BRAS-01 (18ms), access-04 (14ms). All within tolerance.";
  if (t.includes("critical")) return "The last critical alert (INC-1042) was a MikroTik CPU spike on edge-01; root cause: port scan storm. Now recovered.";
  return "I can help summarize incidents, list down devices, or profile latency. Select a suggestion below or ask a network question.";
}

export function AiChatWidget() {
  const [open, setOpen] = useState(false);
  const [msgs, setMsgs] = useState<Msg[]>([
    { role: "ai", text: "Hi, I'm your Network Assistant. Ask about devices, incidents or latency." },
  ]);
  const [input, setInput] = useState("");
  const [typing, setTyping] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [msgs, typing, open]);

  function send(text: string) {
    const q = text.trim();
    if (!q || typing) return;
    setMsgs((m) => [...m, { role: "user", text: q }]);
    setInput("");
    setTyping(true);
    setTimeout(() => {
      setMsgs((m) => [...m, { role: "ai", text: aiReply(q) }]);
      setTyping(false);
    }, 900);
  }

  return (
    <>
      {!open && (
        <button
          onClick={() => setOpen(true)}
          aria-label="Open Network Assistant"
          className="fixed bottom-5 right-5 z-50 flex h-10 w-10 items-center justify-center rounded-full bg-[#A371F7] text-[#0D1117] shadow-lg transition-transform duration-100 hover:scale-105"
        >
          <Bot size={20} />
        </button>
      )}

      {open && (
        <div className="fixed bottom-5 right-5 z-50 flex h-[520px] w-[380px] flex-col overflow-hidden rounded-[10px] border border-[#30363D] bg-[#161B22] shadow-2xl">
          {/* header */}
          <div className="flex items-center justify-between border-b border-[#21262D] bg-[#1C2128] px-4 py-3">
            <div className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-[#A371F7]" />
              <span className="text-sm font-semibold text-[#E6EDF3]">Network Assistant</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="mono text-[10px] text-[#484F58]">Tenant: ISP-01</span>
              <button onClick={() => setOpen(false)} aria-label="Close" className="text-[#8B949E] hover:text-[#E6EDF3]">
                <X size={16} />
              </button>
            </div>
          </div>

          {/* messages */}
          <div className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
            {msgs.map((m, i) =>
              m.role === "user" ? (
                <div key={i} className="flex justify-end">
                  <div className="max-w-[80%] rounded-[10px] rounded-br-[2px] bg-[#21262D] px-3 py-2 text-xs text-[#E6EDF3]">
                    {m.text}
                  </div>
                </div>
              ) : (
                <div key={i} className="flex justify-start">
                  <div className="max-w-[85%]">
                    <div className="mb-1 text-[9px] font-semibold uppercase tracking-wider text-[#A371F7]">
                      Network Assistant
                    </div>
                    <div className="rounded-[10px] rounded-bl-[2px] border border-[#30369066] bg-[#1A1140] px-3 py-2 text-xs text-[#E6EDF3]">
                      {m.text}
                    </div>
                  </div>
                </div>
              )
            )}
            {typing && (
              <div className="flex justify-start">
                <div className="rounded-[10px] border border-[#30369066] bg-[#1A1140] px-3 py-2">
                  <span className="flex gap-1">
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-[#A371F7]" />
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-[#A371F7] [animation-delay:120ms]" />
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-[#A371F7] [animation-delay:240ms]" />
                  </span>
                </div>
              </div>
            )}
            <div ref={endRef} />
          </div>

          {/* suggested prompts */}
          <div className="flex flex-wrap gap-2 border-t border-[#21262D] px-4 py-2">
            {SUGGESTED.map((s) => (
              <button
                key={s}
                onClick={() => send(s)}
                className="rounded-[20px] border border-[#30363D] bg-[#21262D] px-2.5 py-1 text-[10px] text-[#58A6FF] transition-colors duration-100 hover:bg-[#1C2128]"
              >
                {s}
              </button>
            ))}
          </div>

          {/* input */}
          <div className="flex items-center gap-2 border-t border-[#21262D] px-3 py-3">
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && send(input)}
              placeholder="Ask a network question…"
              className="flex-1 rounded-[5px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-xs text-[#E6EDF3] outline-none placeholder:text-[#484F58] focus:border-[#6E40C9]"
            />
            <button
              onClick={() => send(input)}
              disabled={!input.trim() || typing}
              aria-label="Send"
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[5px] bg-[#6E40C9] text-white disabled:opacity-40"
            >
              <Send size={14} />
            </button>
          </div>
        </div>
      )}
    </>
  );
}
