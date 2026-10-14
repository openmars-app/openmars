import { useEffect, useState } from "react";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import { gatewayServer, switchServer, type GatewayServerEntry } from "@/utils/gateway.ts";
import { probeServer, type ServerProbe } from "@/utils/serverProbe.ts";

const RECHECK_MS = 5000;
const SERVERS: GatewayServerEntry[] = gatewayServer?.servers ?? [];

/**
 * The gateway's servers in its order, re-checked every few seconds while
 * shown. Online servers can be switched to (a reload into that server), down
 * ones show as offline, and ones that answer but not as a gateway server are
 * left out.
 */
export default function ServerList({
  active,
  rowClassName,
}: {
  active: boolean;
  rowClassName: string;
}) {
  const probes = useServerProbes(SERVERS, active);
  return (
    <ServerListView
      servers={SERVERS}
      probes={probes}
      currentAlias={gatewayServer?.alias}
      onSelect={switchServer}
      rowClassName={rowClassName}
    />
  );
}

export function ServerListView({
  servers,
  probes,
  currentAlias,
  onSelect,
  rowClassName,
}: {
  servers: GatewayServerEntry[];
  probes: Record<string, ServerProbe>;
  currentAlias?: string;
  onSelect: (alias: string) => void;
  rowClassName: string;
}) {
  return (
    <>
      {servers.map((server) => (
        <ServerRow
          key={server.alias}
          server={server}
          probe={probes[server.alias]}
          className={rowClassName}
          current={server.alias === currentAlias}
          onSelect={onSelect}
        />
      ))}
    </>
  );
}

function ServerRow({
  server,
  probe,
  className,
  current,
  onSelect,
}: {
  current: boolean;
  onSelect: (alias: string) => void;
  server: GatewayServerEntry;
  probe?: ServerProbe;
  className: string;
}) {
  if (probe?.status === "invalid") {
    return null;
  }
  const online = probe?.status === "online";
  let status = "Checking…";
  if (probe?.status === "online") {
    status = `${probe.latencyMs} ms`;
  } else if (probe?.status === "down") {
    status = "Offline";
  }
  return (
    <GameButton
      emphasis="quiet"
      disabled={!online || current}
      onClick={() => onSelect(server.alias)}
      className={`w-full !justify-start gap-3 text-left hover:bg-white/10 transition-colors ${
        current ? "!opacity-100" : ""
      } ${className}`}
    >
      <span className="server-dot" data-status={probe?.status ?? "checking"} />
      <span className="whitespace-nowrap">
        {online ? probe.meta.name : server.alias.toUpperCase()}
      </span>
      <span className="min-w-0 flex-1 truncate text-xs text-white/40">
        {online ? probe.meta.version : ""}
      </span>
      {current && <span className="text-xs text-white/40">Current</span>}
      <span className="whitespace-nowrap text-xs text-white/40">{status}</span>
    </GameButton>
  );
}

function useServerProbes(servers: GatewayServerEntry[], active: boolean) {
  const [probes, setProbes] = useState<Record<string, ServerProbe>>({});
  useEffect(() => {
    if (!active) {
      return;
    }
    let cancelled = false;
    const probeAll = () => {
      for (const server of servers) {
        void probeServer(server).then((probe) => {
          if (!cancelled) {
            setProbes((previous) => ({ ...previous, [server.alias]: probe }));
          }
        });
      }
    };
    probeAll();
    const id = window.setInterval(probeAll, RECHECK_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [servers, active]);
  return probes;
}
