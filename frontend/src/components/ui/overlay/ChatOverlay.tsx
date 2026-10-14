import { useOverlayEventTarget } from "@/contexts/OverlayEnvironment";
import React, { useState, useRef, useEffect, useLayoutEffect, useMemo } from "react";
import { ChatMessageDto } from "@/types/generated/api-types.ts";
import { Z_INDEX } from "@/constants/zIndex";
import { useGameStore } from "@/stores/gameStore.ts";
import { useBotPresenceStore } from "@/stores/botPresenceStore.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import EmoteIcon, { EMOTES } from "@/components/ui/display/EmoteIcon.tsx";
import GameButton from "@/components/ui/buttons/GameButton.tsx";

const BAR_HEIGHT = 90;
const SNAP_THRESHOLD = 40;

interface ChatOverlayProps {
  messages: ChatMessageDto[];
  onSendMessage: (message: string) => void;
  embedded?: boolean;
  fill?: boolean;
  isEndgame?: boolean;
  playerColorMap?: Map<string, string>;
  onBoundsChange?: (bounds: DOMRectReadOnly | null) => void;
}

const CHAT_WIDTH = 416;
const MIN_VISIBLE = 80;

function clampToViewport(element: HTMLElement | null, x: number, y: number) {
  const w = element?.offsetWidth ?? CHAT_WIDTH;
  const h = element?.offsetHeight ?? 0;
  const clampedX = Math.max(MIN_VISIBLE - w, Math.min(window.innerWidth - MIN_VISIBLE, x));
  const clampedY = Math.max(0, Math.min(window.innerHeight - h, y));
  return { x: clampedX, y: clampedY };
}

const ChatOverlay: React.FC<ChatOverlayProps> = (props) => {
  const isSpectator = useGameStore((s) => s.isSpectator);
  const game = useGameStore((s) => s.game);
  const typing = useBotPresenceStore((s) => s.typing);

  const typingNames = useMemo(() => {
    const names: string[] = [];
    const players = [
      ...(game?.currentPlayer?.id ? [game.currentPlayer] : []),
      ...(game?.otherPlayers ?? []),
    ];
    for (const player of players) {
      if (typing[player.id]) {
        names.push(player.name);
      }
    }
    return names;
  }, [game?.currentPlayer, game?.otherPlayers, typing]);

  return (
    <ChatOverlayView
      {...props}
      isSpectator={isSpectator}
      typingNames={typingNames}
      onSendEmote={(kind) => {
        void globalWebSocketManager.sendEmote(kind);
      }}
    />
  );
};

export const ChatOverlayView: React.FC<
  ChatOverlayProps & {
    isSpectator: boolean;
    typingNames: string[];
    onSendEmote: (kind: Parameters<typeof globalWebSocketManager.sendEmote>[0]) => void;
  }
> = ({
  isSpectator,
  typingNames,
  onSendEmote,
  messages,
  onSendMessage,
  embedded,
  fill,
  isEndgame,
  playerColorMap,
  onBoundsChange,
}) => {
  const eventTarget = useOverlayEventTarget();
  const [inputValue, setInputValue] = useState("");
  const [position, setPosition] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const [isSnapped, setIsSnapped] = useState(true);
  const [snapX, setSnapX] = useState(0);
  const dragOffset = useRef({ x: 0, y: 0 });
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messagesRef = useRef<HTMLDivElement>(null);
  const stuckToBottomRef = useRef(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const [showEmotePicker, setShowEmotePicker] = useState(false);
  const emotePickerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showEmotePicker) {
      return undefined;
    }
    const handleClick = (e: PointerEvent) => {
      if (emotePickerRef.current && !emotePickerRef.current.contains(e.target as Node)) {
        setShowEmotePicker(false);
      }
    };
    eventTarget.addEventListener("pointerdown", handleClick);
    return () => eventTarget.removeEventListener("pointerdown", handleClick);
  }, [showEmotePicker, eventTarget]);

  const snapBottom = BAR_HEIGHT;

  useLayoutEffect(() => {
    const element = containerRef.current;
    if (!element || embedded || !onBoundsChange) {
      return;
    }
    const measure = () => onBoundsChange(element.getBoundingClientRect());
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    window.addEventListener("resize", measure);
    measure();
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [embedded, onBoundsChange, position, snapX, isSnapped, isEndgame]);

  useEffect(() => () => onBoundsChange?.(null), [onBoundsChange]);

  useEffect(() => {
    messagesEndRef.current?.parentElement?.scrollTo({
      top: messagesEndRef.current.parentElement.scrollHeight,
    });
  }, [messages]);

  useEffect(() => {
    const list = messagesRef.current;
    if (!fill || !list) {
      return undefined;
    }
    const observer = new ResizeObserver(() => {
      if (stuckToBottomRef.current) {
        list.scrollTop = list.scrollHeight;
      }
    });
    observer.observe(list);
    return () => observer.disconnect();
  }, [fill]);

  const handleMessagesScroll = () => {
    const list = messagesRef.current;
    if (!list) {
      return;
    }
    stuckToBottomRef.current = list.scrollHeight - list.scrollTop - list.clientHeight < 24;
  };

  useEffect(() => {
    if (embedded) {
      return undefined;
    }
    const handleResize = () => {
      const element = containerRef.current;
      setSnapX((x) => (x > 0 ? clampToViewport(element, x, 0).x : x));
      setPosition((p) => clampToViewport(element, p.x, p.y));
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [embedded]);

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    const target = e.target as HTMLElement;
    if (embedded || target.tagName === "INPUT") {
      return;
    }
    if (e.pointerType !== "mouse" && messagesRef.current?.contains(target)) {
      return;
    }
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) {
      return;
    }
    dragOffset.current = { x: e.clientX - rect.left, y: e.clientY - rect.top };
    e.currentTarget.setPointerCapture(e.pointerId);
    setIsDragging(true);
  };

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDragging) {
      return;
    }
    const element = containerRef.current;
    const rawX = e.clientX - dragOffset.current.x;
    const rawY = e.clientY - dragOffset.current.y;
    const bottomEdge = rawY + (element?.offsetHeight ?? 0);
    const snapY = window.innerHeight - snapBottom;

    if (Math.abs(bottomEdge - snapY) < SNAP_THRESHOLD) {
      setIsSnapped(true);
      setSnapX(clampToViewport(element, rawX, 0).x);
    } else {
      setIsSnapped(false);
      setPosition(clampToViewport(element, rawX, rawY));
    }
  };

  const handlePointerEnd = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDragging) {
      return;
    }
    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
      e.currentTarget.releasePointerCapture(e.pointerId);
    }
    setIsDragging(false);
  };

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const trimmed = inputValue.trim();
    if (!trimmed) {
      return;
    }
    onSendMessage(trimmed);
    setInputValue("");
  };

  let containerClass = "w-[min(416px,calc(100vw-16px))] select-none touch-none bg-white/5";
  if (embedded && fill) {
    containerClass = "w-full h-full min-w-0 min-h-0 flex flex-col";
  } else if (embedded) {
    containerClass =
      "w-full min-w-0 min-h-[240px] max-h-[320px] flex-1 flex flex-col border-t border-white/10 pt-3";
  }

  const style: React.CSSProperties = isSnapped
    ? isEndgame
      ? {
          position: "fixed",
          right: 16,
          top: "50%",
          transform: "translateY(-50%)",
          zIndex: Z_INDEX.UI_BASE,
        }
      : {
          position: "fixed",
          bottom: snapBottom,
          right: snapX > 0 ? undefined : 80,
          left: snapX > 0 ? snapX : undefined,
          zIndex: Z_INDEX.UI_BASE,
        }
    : {
        position: "fixed",
        left: position.x,
        top: position.y,
        zIndex: Z_INDEX.UI_BASE,
      };

  return (
    <>
      {isDragging && (
        <div className="fixed inset-0" style={{ zIndex: Z_INDEX.DRAG_SHIELD, cursor: "default" }} />
      )}
      <div
        ref={containerRef}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
        onPointerCancel={handlePointerEnd}
        className={containerClass}
        style={embedded ? undefined : { ...style, cursor: "default" }}
      >
        <div
          ref={messagesRef}
          onScroll={handleMessagesScroll}
          className={`${embedded ? "h-0 min-h-0 flex-1" : "h-[200px]"} overflow-y-auto overflow-x-hidden overscroll-contain touch-pan-y px-2 py-1 flex flex-col gap-1.5`}
        >
          {messages.map((msg, i) => {
            const time = msg.timestamp ? new Date(msg.timestamp) : null;
            const timeStr = time
              ? `${String(time.getHours()).padStart(2, "0")}:${String(time.getMinutes()).padStart(2, "0")}`
              : "";
            const senderColor =
              (msg.senderId && playerColorMap?.get(msg.senderId)) || msg.senderColor;
            if (msg.kind === "system") {
              return (
                <div key={i} className="shrink-0 text-left text-xs italic text-white/40 min-w-0">
                  {timeStr && (
                    <span className="not-italic text-white/20 text-[10px] mr-1.5">{timeStr}</span>
                  )}
                  {msg.message}
                </div>
              );
            }
            if (msg.kind === "recap") {
              return (
                <div
                  key={i}
                  className="shrink-0 text-left min-w-0 border-l-2 bg-white/5 px-2 py-1.5 my-0.5"
                  style={{ borderLeftColor: senderColor }}
                >
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="font-orbitron text-[9px] font-bold tracking-[0.15em] text-white/50">
                      RECAP
                    </span>
                    <span className="text-sm font-semibold" style={{ color: senderColor }}>
                      {msg.senderName}
                    </span>
                  </div>
                  <p className="text-sm text-white/75 leading-relaxed break-words">{msg.message}</p>
                </div>
              );
            }
            return (
              <div key={i} className="text-sm leading-relaxed flex shrink-0 text-left min-w-0">
                <span className="shrink-0">
                  {timeStr && <span className="text-white/20 text-[10px] mr-1.5">{timeStr}</span>}
                  <span className="font-semibold" style={{ color: senderColor }}>
                    {msg.senderName}
                  </span>
                  {msg.isSpectator && (
                    <span className="text-white/25 text-[10px] ml-0.5">(spectator)</span>
                  )}
                  <span className="text-white/40 mr-2"> </span>
                </span>
                <span className="text-white/70 break-words min-w-0">{msg.message}</span>
              </div>
            );
          })}
          <div ref={messagesEndRef} />
        </div>

        {typingNames.length > 0 && (
          <div className="shrink-0 px-2 pb-1 text-[11px] italic text-white/40 text-left">
            {`${typingNames.join(", ")} ${typingNames.length > 1 ? "are" : "is"} typing…`}
          </div>
        )}

        <form
          onSubmit={handleSubmit}
          className="border-t border-white/15 shrink-0 flex items-center gap-1"
        >
          {!isSpectator && (
            <div
              ref={emotePickerRef}
              className="relative shrink-0"
              onPointerDown={(e) => e.stopPropagation()}
            >
              <button
                type="button"
                aria-label="Send an emote"
                aria-expanded={showEmotePicker}
                onClick={() => setShowEmotePicker((v) => !v)}
                className={`flex items-center justify-center w-7 h-7 compact:w-11 compact:h-11 cursor-pointer transition-opacity ${showEmotePicker ? "opacity-100" : "opacity-50 hover:opacity-100"}`}
              >
                <EmoteIcon emote="laugh" size={18} />
              </button>
              {showEmotePicker && (
                <div
                  className="absolute bottom-full left-0 mb-1 w-max max-h-[40dvh] overflow-y-auto overscroll-contain grid grid-cols-[repeat(4,2.25rem)] compact:grid-cols-[repeat(4,2.75rem)] gap-1 p-1.5 bg-[rgba(10,10,15,0.97)] border border-white/20 rounded-lg shadow-[0_4px_16px_rgba(0,0,0,0.6)]"
                  style={{ zIndex: Z_INDEX.DROPDOWNS }}
                >
                  {EMOTES.map(({ kind, label }) => (
                    <button
                      key={kind}
                      type="button"
                      aria-label={label}
                      onClick={() => {
                        setShowEmotePicker(false);
                        onSendEmote(kind);
                      }}
                      className="flex items-center justify-center w-9 h-9 compact:w-11 compact:h-11 rounded cursor-pointer hover:bg-white/10 hover:scale-110 transition-transform"
                    >
                      <EmoteIcon emote={kind} size={24} />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
          <input
            type="text"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder="Message…"
            aria-label="Chat message"
            spellCheck={false}
            autoComplete="off"
            enterKeyHint="send"
            className="flex-1 min-w-0 bg-transparent text-white text-sm compact:text-base px-1 py-2 compact:min-h-11 outline-none placeholder:text-white/25 border-b border-white/15"
          />
          {fill && (
            <GameButton type="submit" size="sm" className="shrink-0 !min-h-11">
              Send
            </GameButton>
          )}
        </form>
      </div>
    </>
  );
};

export default ChatOverlay;
