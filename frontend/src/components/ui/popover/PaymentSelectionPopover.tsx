import { useEffect, useState } from "react";
import type {
  CardDto,
  PaymentDto,
  PaymentOptionDto,
  PaymentQuoteDto,
  ResourceType,
} from "@/types/generated/api-types";
import { usePaymentStore } from "@/stores/paymentStore";
import { useGameStore } from "@/stores/gameStore";
import GameIcon from "../display/GameIcon";
import GameButton from "../buttons/GameButton";
import { GameFlowPopover, GameFlowTitle, GameFlowFooter } from "./GameFlowPopover";

const key = (o: PaymentOptionDto) => JSON.stringify([o.source, o.targetResource]);
const poolKey = (o: PaymentOptionDto) => JSON.stringify(o.source);
const native = (o: PaymentOptionDto) =>
  o.source.target === "self-player" && o.source.resource === o.targetResource;

export default function PaymentSelectionPopover() {
  const pending = usePaymentStore((s) => s.pending);
  const player = useGameStore((s) => s.currentPlayer);
  const [quote, setQuote] = useState<PaymentQuoteDto | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    setQuote(pending?.quote ?? null);
    setError("");
  }, [pending]);
  useEffect(() => {
    if (!pending) return;
    let active = true;
    setLoading(true);
    void pending
      .refresh()
      .then((next) => {
        if (active) {
          setQuote(next);
          setError("");
        }
      })
      .catch((e: Error) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [pending, player]);
  if (!pending || !quote) return null;
  return (
    <PaymentPicker
      quote={quote}
      sourceCards={[
        ...(player?.playedCards ?? []),
        ...(player?.corporation ? [player.corporation] : []),
      ]}
      loading={loading}
      error={error}
      onConfirm={pending.resolve}
      onCancel={() => pending.reject(new Error("Payment cancelled"))}
    />
  );
}

export function PaymentPicker({
  quote,
  sourceCards = [],
  loading = false,
  error = "",
  onConfirm,
  onCancel,
}: {
  quote: PaymentQuoteDto;
  sourceCards?: CardDto[];
  loading?: boolean;
  error?: string;
  onConfirm: (payment: PaymentDto) => void;
  onCancel: () => void;
}) {
  const [counts, setCounts] = useState<Record<string, number>>({});
  useEffect(() => {
    setCounts((previous) => {
      const pools = new Map(quote.options.map((o) => [poolKey(o), o.available]));
      const next: Record<string, number> = {};
      for (const option of quote.options.filter((o) => !native(o))) {
        const remaining = pools.get(poolKey(option)) ?? 0;
        const amount = Math.min(previous[key(option)] ?? 0, remaining);
        next[key(option)] = amount;
        pools.set(poolKey(option), remaining - amount);
      }
      return next;
    });
  }, [quote]);
  const available = new Map(quote.options.map((o) => [poolKey(o), o.available]));
  const totals: Record<string, number> = {};
  const payment: PaymentDto = { allocations: [] };
  let valid = !loading && !error;
  const allocate = (o: PaymentOptionDto, amount: number) => {
    const left = available.get(poolKey(o)) ?? 0;
    if (amount > left) valid = false;
    available.set(poolKey(o), left - amount);
    totals[o.targetResource] = (totals[o.targetResource] ?? 0) + amount * o.conversionRate;
    if (amount > 0)
      payment.allocations.push({ source: o.source, targetResource: o.targetResource, amount });
  };
  for (const o of quote.options.filter((o) => !native(o))) allocate(o, counts[key(o)] ?? 0);
  const remainingPools = new Map(available);
  const substituteTotals = { ...totals };
  for (const o of quote.options.filter(native)) {
    allocate(
      o,
      Math.max(0, (quote.costs[o.targetResource] ?? 0) - (totals[o.targetResource] ?? 0)),
    );
  }
  for (const [rt, cost] of Object.entries(quote.costs)) if ((totals[rt] ?? 0) < cost) valid = false;
  const names = new Map(sourceCards.map((card) => [card.id, card.name]));
  return (
    <GameFlowPopover
      isVisible
      onClose={onCancel}
      className="min-w-[360px] compact:min-w-0 max-h-[min(650px,100dvh_-_32px)] text-white"
    >
      <GameFlowTitle>
        <h3 className="m-0 font-orbitron text-white text-base font-bold">Select payment</h3>
      </GameFlowTitle>
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 space-y-3">
        {quote.options
          .filter((o) => !native(o) && o.available > 0)
          .map((o) => {
            const n = counts[key(o)] ?? 0;
            return (
              <div key={key(o)} className="rounded border border-white/15 p-3">
                {o.source.cardId && (
                  <div className="mb-2 font-orbitron text-sm text-gray-300">
                    {names.get(o.source.cardId) ?? o.source.cardId}
                  </div>
                )}
                <div className="flex items-center gap-2">
                  <GameIcon iconType={o.source.resource} size="small" />
                  <span>→</span>
                  <GameIcon iconType={o.targetResource} amount={o.conversionRate} size="small" />
                  <span className="font-orbitron text-xs compact:text-[13px] text-gray-400 mr-auto">
                    {o.available} available
                  </span>
                  <GameButton
                    size="sm"
                    className="w-9 h-9 compact:size-11 p-0"
                    onClick={() => setCounts({ ...counts, [key(o)]: Math.max(0, n - 1) })}
                    aria-label={`Use less ${o.source.resource}`}
                    disabled={!n}
                  >
                    <span className="text-xl font-bold leading-none" aria-hidden="true">
                      −
                    </span>
                  </GameButton>
                  <span className="font-orbitron min-w-5 text-center">{n}</span>
                  <GameButton
                    size="sm"
                    className="w-9 h-9 compact:size-11 p-0"
                    onClick={() => setCounts({ ...counts, [key(o)]: n + 1 })}
                    aria-label={`Use more ${o.source.resource}`}
                    disabled={
                      (remainingPools.get(poolKey(o)) ?? 0) <= 0 ||
                      (substituteTotals[o.targetResource] ?? 0) >= quote.costs[o.targetResource]
                    }
                  >
                    <span className="text-xl font-bold leading-none" aria-hidden="true">
                      +
                    </span>
                  </GameButton>
                </div>
              </div>
            );
          })}
        {payment.allocations
          .filter(
            (a) => a.source.target === "self-player" && a.source.resource === a.targetResource,
          )
          .map((a) => (
            <div key={a.targetResource} className="flex justify-between items-center text-gray-300">
              <span className="font-orbitron text-sm">Remaining payment</span>
              <GameIcon iconType={a.source.resource} amount={a.amount} size="medium" />
            </div>
          ))}
        {Object.entries(quote.costs)
          .filter(([rt, cost]) => (totals[rt] ?? 0) > cost)
          .map(([rt, cost]) => (
            <div key={rt} className="flex items-center gap-2 text-sm text-white">
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="shrink-0 text-yellow-400"
                aria-hidden="true"
              >
                <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                <line x1="12" y1="9" x2="12" y2="13" />
                <line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span>Excess</span>
              <GameIcon iconType={rt as ResourceType} amount={totals[rt] - cost} size="medium" />
              <span>is lost</span>
            </div>
          ))}
        {error && <p className="text-red-400 text-sm">{error}</p>}
        {!valid && !loading && !error && (
          <p className="text-red-400 text-sm">Choose enough resources to cover the cost.</p>
        )}
      </div>
      <GameFlowFooter className="justify-end gap-3">
        <GameButton size="sm" emphasis="secondary" onClick={onCancel}>
          Cancel
        </GameButton>
        <GameButton size="sm" disabled={!valid} onClick={() => onConfirm(payment)}>
          Confirm
        </GameButton>
      </GameFlowFooter>
    </GameFlowPopover>
  );
}
