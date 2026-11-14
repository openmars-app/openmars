import React, { memo, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import { getBotPersonaLabel } from "@/components/ui/display/BotChips.tsx";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { useBotTraceStore } from "@/stores/botTraceStore.ts";
import {
  MessageTypeBotInspect,
  type BotCallDto,
  type BotCallStepDto,
  type BotGrudgeDto,
  type BotPlanDto,
  type BotReactionDto,
  type BotTraceDto,
  type BotTraceEventDto,
  type ErrorPayload,
  type GameDto,
} from "@/types/generated/api-types.ts";
import PlayerSelector from "../PlayerSelector.tsx";

interface BotInspectorPageProps {
  gameState: GameDto;
}

type InspectorTab = "plan" | "live" | "history" | "reactions";

const TABS: { id: InspectorTab; label: string }[] = [
  { id: "plan", label: "Plan" },
  { id: "live", label: "Live" },
  { id: "history", label: "History" },
  { id: "reactions", label: "Reactions" },
];

const ROLE_STYLES: Record<BotCallDto["role"], string> = {
  executor: "bg-indigo-600/70 text-white",
  planner: "bg-purple-600/70 text-white",
  reactor: "bg-sky-700/70 text-white",
  recap: "bg-slate-600/70 text-white",
};

const DECISION_STYLES: Record<BotReactionDto["decision"], string> = {
  reacted: "bg-emerald-700/70 text-white",
  throttled: "bg-amber-700/70 text-white",
  busy: "bg-slate-600/70 text-white",
  "own-turn": "bg-indigo-600/70 text-white",
  quiet: "bg-slate-800/70 text-white",
};

const STICK_THRESHOLD_PX = 24;

interface BotPlayer {
  id: string;
  name: string;
  botStatus?: "loading" | "ready" | "failed" | "thinking";
  botPersona?: string;
  botError?: string;
}

function isBotRunning(bot: BotPlayer | undefined): boolean {
  return bot !== undefined && bot.botStatus !== "loading" && bot.botStatus !== "failed";
}

function errorMessageOf(payload: ErrorPayload): string {
  return payload.message || "Bot inspection failed";
}

function useBotTraceSubscription(playerId: string, active: boolean) {
  const subscribedRef = useRef(false);

  useEffect(() => {
    const store = useBotTraceStore.getState();
    if (!playerId || !active) {
      if (subscribedRef.current) {
        subscribedRef.current = false;
        globalWebSocketManager.inspectBot("").catch(() => undefined);
      }
      store.reset();
      return;
    }
    store.begin(playerId);

    const subscribe = () => {
      globalWebSocketManager.inspectBot(playerId).catch((error: unknown) => {
        useBotTraceStore.getState().fail(String(error));
      });
    };
    const handleSnapshot = (snapshot: BotTraceDto) => {
      useBotTraceStore.getState().applySnapshot(snapshot);
    };
    const handleEvent = (event: BotTraceEventDto) => {
      useBotTraceStore.getState().applyEvent(event);
    };
    const handleError = (payload: ErrorPayload) => {
      if (payload.requestType === MessageTypeBotInspect) {
        useBotTraceStore.getState().fail(errorMessageOf(payload));
      }
    };

    globalWebSocketManager.on("bot-trace-snapshot", handleSnapshot);
    globalWebSocketManager.on("bot-trace-event", handleEvent);
    globalWebSocketManager.on("error", handleError);
    globalWebSocketManager.on("connect", subscribe);
    subscribedRef.current = true;
    subscribe();

    return () => {
      globalWebSocketManager.off("bot-trace-snapshot", handleSnapshot);
      globalWebSocketManager.off("bot-trace-event", handleEvent);
      globalWebSocketManager.off("error", handleError);
      globalWebSocketManager.off("connect", subscribe);
    };
  }, [playerId, active]);

  useEffect(
    () => () => {
      if (subscribedRef.current) {
        subscribedRef.current = false;
        globalWebSocketManager.inspectBot("").catch(() => undefined);
      }
      useBotTraceStore.getState().reset();
    },
    [],
  );
}

function useNow(intervalMs: number, enabled = true): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) {
      return;
    }
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs, enabled]);
  return now;
}

function formatClock(at: string | undefined): string {
  if (!at) {
    return "-";
  }
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleTimeString();
}

function formatSeconds(ms: number): string {
  if (ms < 60_000) {
    return `${(ms / 1000).toFixed(1)}s`;
  }
  const minutes = Math.floor(ms / 60_000);
  const seconds = Math.floor((ms % 60_000) / 1000);
  return `${minutes}m ${seconds}s`;
}

function formatAgo(at: string | undefined, now: number): string {
  if (!at) {
    return "never";
  }
  const time = new Date(at).getTime();
  if (Number.isNaN(time)) {
    return "never";
  }
  const seconds = Math.max(0, Math.floor((now - time) / 1000));
  if (seconds < 60) {
    return `${seconds}s ago`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  return `${Math.floor(minutes / 60)}h ago`;
}

function newestFirstOf<T>(items: T[]): T[] {
  const result: T[] = [];
  for (let i = items.length - 1; i >= 0; i--) {
    result.push(items[i]);
  }
  return result;
}

function prettyJson(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

const SectionLabel: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="font-orbitron text-[10px] uppercase tracking-wider text-white/50 mb-1">
    {children}
  </div>
);

const Badge: React.FC<{ className: string; children: React.ReactNode }> = ({
  className,
  children,
}) => (
  <span
    className={`font-orbitron text-[9px] uppercase tracking-wider px-1.5 py-0.5 rounded ${className}`}
  >
    {children}
  </span>
);

const Disclosure: React.FC<{ open: boolean; onToggle: () => void; label: string }> = ({
  open,
  onToggle,
  label,
}) => (
  <button
    type="button"
    onClick={onToggle}
    className="font-orbitron text-[10px] text-sky-300 hover:text-sky-200 cursor-pointer bg-transparent border-0 p-0"
  >
    {open ? `Hide ${label}` : `Show ${label}`}
  </button>
);

const TextStep: React.FC<{ step: BotCallStepDto }> = ({ step }) => {
  const [expanded, setExpanded] = useState(false);
  const text = step.text ?? "";
  const long = text.length > 160 || text.split("\n").length > 2;
  return (
    <div className="text-xs text-white/85">
      <div className={`whitespace-pre-wrap break-words ${expanded ? "" : "line-clamp-2"}`}>
        {text}
      </div>
      {long && <Disclosure open={expanded} onToggle={() => setExpanded(!expanded)} label="more" />}
    </div>
  );
};

const ToolStep: React.FC<{ step: BotCallStepDto }> = ({ step }) => {
  const [showInput, setShowInput] = useState(false);
  const input = step.input ?? "";
  const pretty = useMemo(() => (showInput ? prettyJson(input) : ""), [showInput, input]);
  const resultClass = step.isError ? "text-red-300" : "text-white/75";
  return (
    <div className="text-xs">
      <div className="flex items-center gap-2">
        <span
          className={`font-orbitron text-[11px] ${step.isError ? "text-red-300" : "text-amber-200"}`}
        >
          {step.tool ?? "tool"}
        </span>
        {step.isError && <Badge className="bg-red-700/70 text-white">Error</Badge>}
        {input && (
          <Disclosure open={showInput} onToggle={() => setShowInput(!showInput)} label="input" />
        )}
      </div>
      {showInput && (
        <pre className="mt-1 max-h-48 overflow-auto rounded bg-black/50 p-2 font-mono text-[11px] text-white/80 whitespace-pre-wrap break-all">
          {pretty}
        </pre>
      )}
      {step.result && (
        <div
          className={`mt-1 whitespace-pre-wrap break-words font-mono text-[11px] ${resultClass}`}
        >
          {step.result}
        </div>
      )}
    </div>
  );
};

const StepRow = memo(function StepRow({ step }: { step: BotCallStepDto }) {
  let border = "border-white/15";
  if (step.isError) {
    border = "border-red-500/70 bg-red-950/30";
  } else if (step.kind === "tool") {
    border = "border-amber-400/40";
  }
  return (
    <div className={`border-l-2 pl-2 py-1 ${border}`}>
      <div className="font-orbitron text-[9px] text-white/40 mb-0.5">{formatClock(step.at)}</div>
      {step.kind === "tool" ? <ToolStep step={step} /> : <TextStep step={step} />}
    </div>
  );
});

const StepTimeline = memo(function StepTimeline({ steps }: { steps: BotCallStepDto[] }) {
  if (steps.length === 0) {
    return <div className="text-xs text-white/50">No steps yet.</div>;
  }
  return (
    <div className="flex flex-col gap-1.5">
      {steps.map((step, index) => (
        <StepRow key={`${step.at}-${index}`} step={step} />
      ))}
    </div>
  );
});

const Elapsed: React.FC<{ call: BotCallDto }> = ({ call }) => {
  const now = useNow(1000, call.running);
  if (!call.running) {
    return <span className="font-orbitron">{formatSeconds(call.durationMs)}</span>;
  }
  const started = new Date(call.startedAt).getTime();
  return <span className="font-orbitron">{formatSeconds(Math.max(0, now - started))}</span>;
};

const CallHeader: React.FC<{ call: BotCallDto }> = ({ call }) => (
  <div className="flex flex-wrap items-center gap-2 text-[11px] text-white/70">
    <Badge className={ROLE_STYLES[call.role]}>{call.role}</Badge>
    <span className="font-orbitron">{call.model}</span>
    <Elapsed call={call} />
    {call.running && <Badge className="bg-purple-600/70 text-white">Running</Badge>}
    {!call.running && call.costUsd > 0 && (
      <span className="font-orbitron">${call.costUsd.toFixed(4)}</span>
    )}
  </div>
);

const UpdatedAgo: React.FC<{ at?: string }> = ({ at }) => {
  const now = useNow(1000);
  return (
    <span className="font-orbitron text-[10px] text-white/50">Updated {formatAgo(at, now)}</span>
  );
};

const ListBlock: React.FC<{ label: string; items: string[] }> = ({ label, items }) => (
  <div>
    <SectionLabel>{label}</SectionLabel>
    {items.length === 0 ? (
      <div className="text-xs text-white/40">None</div>
    ) : (
      <div className="flex flex-wrap gap-1">
        {items.map((item) => (
          <span key={item} className="font-orbitron text-[10px] bg-white/10 rounded px-1.5 py-0.5">
            {item}
          </span>
        ))}
      </div>
    )}
  </div>
);

const GrudgeList: React.FC<{ grudges: BotGrudgeDto[] }> = ({ grudges }) => (
  <div>
    <SectionLabel>Targeted by</SectionLabel>
    {grudges.length === 0 ? (
      <div className="text-xs text-white/40">Nobody yet</div>
    ) : (
      <ul className="flex flex-col gap-1.5 m-0 p-0 list-none">
        {grudges.map((grudge) => (
          <li key={grudge.playerId} className="text-xs">
            <div className="flex items-center gap-2">
              <span className="text-white/90">{grudge.name}</span>
              <span className="font-orbitron text-[10px] bg-red-500/20 text-red-300 rounded px-1.5 py-0.5">
                {grudge.hits}x
              </span>
            </div>
            <div className="text-white/55">{(grudge.recent ?? []).join("; ")}</div>
          </li>
        ))}
      </ul>
    )}
  </div>
);

const PlanTab: React.FC<{
  plan: BotPlanDto;
  planUpdatedAt?: string;
  grudges: BotGrudgeDto[];
}> = ({ plan, planUpdatedAt, grudges }) => {
  const nextMoves = plan.nextMoves ?? [];
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <UpdatedAgo at={planUpdatedAt} />
      </div>
      {!planUpdatedAt && <div className="text-xs text-white/50">The bot has no plan yet.</div>}
      <div>
        <SectionLabel>Summary</SectionLabel>
        <div className="text-xs text-white/85 whitespace-pre-wrap">{plan.summary || "-"}</div>
      </div>
      <div>
        <SectionLabel>Mood</SectionLabel>
        <div className="text-xs text-white/85">{plan.mood || "-"}</div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <ListBlock label="Wanted hexes" items={plan.wantedHexes ?? []} />
        <ListBlock label="Milestones" items={plan.targetMilestones ?? []} />
        <ListBlock label="Awards" items={plan.targetAwards ?? []} />
        <ListBlock label="Colonies" items={plan.targetColonies ?? []} />
        <ListBlock label="Rivals" items={plan.rivals ?? []} />
      </div>
      <GrudgeList grudges={grudges} />
      <div>
        <SectionLabel>Next moves</SectionLabel>
        {nextMoves.length === 0 ? (
          <div className="text-xs text-white/40">None</div>
        ) : (
          <ol className="flex flex-col gap-1.5 m-0 p-0 list-none">
            {nextMoves.map((move, index) => (
              <li key={`${index}-${move.move}`} className="flex gap-2 text-xs">
                <span className="font-orbitron text-purple-300 w-4 shrink-0">{index + 1}</span>
                <div>
                  <div className="text-white/90">{move.move}</div>
                  {move.why && <div className="text-white/55">{move.why}</div>}
                </div>
              </li>
            ))}
          </ol>
        )}
      </div>
      <div>
        <SectionLabel>Notes</SectionLabel>
        <div className="text-xs text-white/75 whitespace-pre-wrap">{plan.notes || "-"}</div>
      </div>
    </div>
  );
};

const LiveTab: React.FC<{ calls: BotCallDto[] }> = ({ calls }) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const call = useMemo(() => {
    for (let i = calls.length - 1; i >= 0; i--) {
      if (calls[i].running) {
        return calls[i];
      }
    }
    return calls[calls.length - 1];
  }, [calls]);
  const callId = call?.id;
  const stepCount = call?.steps.length ?? 0;

  useLayoutEffect(() => {
    stickRef.current = true;
  }, [callId]);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (el && stickRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [callId, stepCount]);

  if (!call) {
    return <div className="text-xs text-white/50">No calls recorded yet.</div>;
  }

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) {
      return;
    }
    stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_THRESHOLD_PX;
  };

  return (
    <div className="flex flex-col gap-2 flex-1 min-h-0">
      <CallHeader call={call} />
      {call.error && <div className="text-xs text-red-300">{call.error}</div>}
      <div ref={scrollRef} onScroll={handleScroll} className="flex-1 min-h-0 overflow-auto pr-1">
        <StepTimeline steps={call.steps} />
      </div>
    </div>
  );
};

const HistoryRow = memo(function HistoryRow({ call }: { call: BotCallDto }) {
  const [expanded, setExpanded] = useState(false);
  const [showPrompt, setShowPrompt] = useState(false);
  return (
    <div className="rounded border border-white/10 bg-black/30">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex flex-wrap items-center gap-2 px-2 py-1.5 text-left text-[11px] text-white/75 bg-transparent border-0 cursor-pointer hover:bg-white/5"
      >
        <Badge className={ROLE_STYLES[call.role]}>{call.role}</Badge>
        <span className="font-orbitron">{call.model}</span>
        <span className="font-orbitron text-white/50">{formatClock(call.startedAt)}</span>
        <Elapsed call={call} />
        <span className="font-orbitron">${call.costUsd.toFixed(4)}</span>
        <span className="font-orbitron text-white/50">{call.steps.length} steps</span>
        {call.running && <Badge className="bg-purple-600/70 text-white">Running</Badge>}
        {call.error && <Badge className="bg-red-700/70 text-white">Error</Badge>}
      </button>
      {expanded && (
        <div className="flex flex-col gap-2 px-2 pb-2">
          {call.error && <div className="text-xs text-red-300">{call.error}</div>}
          <div>
            <Disclosure
              open={showPrompt}
              onToggle={() => setShowPrompt(!showPrompt)}
              label={`prompt (${call.prompt.length.toLocaleString()} chars)`}
            />
            {showPrompt && (
              <pre className="mt-1 max-h-72 overflow-auto rounded bg-black/50 p-2 font-mono text-[11px] text-white/80 whitespace-pre-wrap break-words">
                {call.prompt}
              </pre>
            )}
          </div>
          <StepTimeline steps={call.steps} />
        </div>
      )}
    </div>
  );
});

const HistoryTab: React.FC<{ calls: BotCallDto[] }> = ({ calls }) => {
  const newestFirst = useMemo(() => newestFirstOf(calls), [calls]);
  if (newestFirst.length === 0) {
    return <div className="text-xs text-white/50">No calls recorded yet.</div>;
  }
  return (
    <div className="flex flex-col gap-1.5">
      {newestFirst.map((call) => (
        <HistoryRow key={call.id} call={call} />
      ))}
    </div>
  );
};

const ReactionRow = memo(function ReactionRow({ reaction }: { reaction: BotReactionDto }) {
  const flags: string[] = [];
  if (reaction.planHit) {
    flags.push("Plan hit");
  }
  if (reaction.personal) {
    flags.push("Personal");
  }
  if (reaction.directed) {
    flags.push("Directed");
  }
  if (reaction.big) {
    flags.push("Big");
  }
  return (
    <div className="rounded border border-white/10 bg-black/30 px-2 py-1.5 flex flex-col gap-1">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-orbitron text-[10px] text-white/50">{formatClock(reaction.at)}</span>
        <Badge className={DECISION_STYLES[reaction.decision]}>{reaction.decision}</Badge>
        {flags.map((flag) => (
          <Badge key={flag} className="bg-white/10 text-white/80">
            {flag}
          </Badge>
        ))}
      </div>
      <ul className="m-0 p-0 list-none flex flex-col gap-0.5">
        {(reaction.lines ?? []).map((line, index) => (
          <li key={index} className="text-xs text-white/70">
            {line}
          </li>
        ))}
      </ul>
      {reaction.output && (
        <div className="text-xs text-purple-200 whitespace-pre-wrap">{reaction.output}</div>
      )}
    </div>
  );
});

const ReactionsTab: React.FC<{ reactions: BotReactionDto[] }> = ({ reactions }) => {
  const newestFirst = useMemo(() => newestFirstOf(reactions), [reactions]);
  if (newestFirst.length === 0) {
    return <div className="text-xs text-white/50">No reactions recorded yet.</div>;
  }
  return (
    <div className="flex flex-col gap-1.5">
      {newestFirst.map((reaction, index) => (
        <ReactionRow key={`${reaction.at}-${index}`} reaction={reaction} />
      ))}
    </div>
  );
};

const BotInspectorPage: React.FC<BotInspectorPageProps> = ({ gameState }) => {
  const bots = useMemo<BotPlayer[]>(
    () =>
      [gameState.currentPlayer, ...gameState.otherPlayers].filter(
        (player) => player.playerType === "bot",
      ),
    [gameState.currentPlayer, gameState.otherPlayers],
  );
  const [selectedId, setSelectedId] = useState("");
  const [tab, setTab] = useState<InspectorTab>("plan");

  const selectedBot = bots.find((bot) => bot.id === selectedId) ?? bots[0];
  const botId = selectedBot?.id ?? "";
  const running = isBotRunning(selectedBot);

  useBotTraceSubscription(botId, running);

  const trace = useBotTraceStore((state) => state.trace);
  const error = useBotTraceStore((state) => state.error);
  if (bots.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
        <div className="font-orbitron text-sm text-white/70">No bots</div>
        <div className="text-xs text-white/50">Add a bot to this game to inspect it here.</div>
      </div>
    );
  }

  const spend = gameState.settings.botSpendUsd ?? 0;
  const cap = gameState.settings.botSpendCapUsd ?? 0;

  const renderBody = () => {
    if (!selectedBot) {
      return null;
    }
    if (!running) {
      return (
        <div className="text-xs text-white/50">
          The bot is not running. Inspection starts once it is ready.
        </div>
      );
    }
    if (error) {
      return <div className="text-xs text-red-300">{error}</div>;
    }
    if (!trace) {
      return <div className="font-orbitron text-xs text-white/50">Connecting...</div>;
    }
    if (tab === "live") {
      return <LiveTab calls={trace.calls} />;
    }
    let content: React.ReactNode = null;
    if (tab === "plan") {
      content = (
        <PlanTab
          plan={trace.plan}
          planUpdatedAt={trace.planUpdatedAt}
          grudges={trace.grudges ?? []}
        />
      );
    } else if (tab === "history") {
      content = <HistoryTab calls={trace.calls} />;
    } else {
      content = <ReactionsTab reactions={trace.reactions} />;
    }
    return <div className="flex-1 min-h-0 overflow-auto pr-1">{content}</div>;
  };

  return (
    <div className="flex flex-col gap-2 min-h-0 text-left" style={{ height: "calc(70vh - 90px)" }}>
      <PlayerSelector players={bots} selectedId={botId} onChange={setSelectedId} />
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-white/70">
        <span className="font-orbitron text-purple-300">
          {getBotPersonaLabel(selectedBot?.botPersona)}
        </span>
        <span className="font-orbitron">{selectedBot?.botStatus ?? "unknown"}</span>
        <span className="font-orbitron">
          ${spend.toFixed(2)} of ${cap.toFixed(2)}
        </span>
      </div>
      {selectedBot?.botStatus === "failed" && selectedBot.botError && (
        <div className="text-xs text-red-300">{selectedBot.botError}</div>
      )}
      <div className="flex gap-1 border-b border-white/10 pb-1">
        {TABS.map(({ id, label }) => (
          <GameButton
            key={id}
            size="xs"
            emphasis={tab === id ? "primary" : "quiet"}
            selected={tab === id}
            accent="#7c3aed"
            onClick={() => setTab(id)}
          >
            {label}
          </GameButton>
        ))}
      </div>
      <div className="flex flex-col flex-1 min-h-0">{renderBody()}</div>
    </div>
  );
};

export default BotInspectorPage;
