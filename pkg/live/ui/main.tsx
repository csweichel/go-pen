declare const React: any;
declare const ReactDOM: any;

type BuildEventType = "" | "ready" | "building" | "error" | "log";
type ArgType = "int" | "float" | "bool";

interface BuildEvent {
  type: BuildEventType;
  file?: string;
  msg?: string;
  log?: string;
}

interface OptimState {
  llo: boolean;
  vpype: boolean;
  vpypeAvailable: boolean;
}

interface ArgSpec {
  name: string;
  description?: string;
  type: ArgType;
  default?: string;
  min?: number;
  max?: number;
}

interface ArgState {
  specs: ArgSpec[];
  values: Record<string, string>;
  raw: string;
}

interface InteractiveTool {
  id: string;
  label: string;
  description?: string;
}

interface InteractiveCanvas {
  width: number;
  height: number;
}

interface InteractiveRegion {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  label?: string;
  hint?: string;
  stroke?: string;
  fill?: string;
  textColor?: string;
}

interface InteractiveManifest {
  canvas: InteractiveCanvas;
  tools: InteractiveTool[];
  regions: InteractiveRegion[];
  defaultTool?: string;
}

interface InteractiveState {
  manifest: InteractiveManifest;
}

interface ServerState {
  sketch: string;
  event: BuildEvent;
  logs: string[];
  optim: OptimState;
  debug: boolean;
  args: ArgState;
  interactive: InteractiveState;
}

const EMPTY_ARG_STATE: ArgState = {
  specs: [],
  values: {},
  raw: "",
};

const EMPTY_INTERACTIVE_STATE: InteractiveState = {
  manifest: {
    canvas: { width: 0, height: 0 },
    tools: [],
    regions: [],
    defaultTool: "",
  },
};

const EMPTY_SERVER_STATE: ServerState = {
  sketch: "",
  event: { type: "" },
  logs: [],
  optim: {
    llo: false,
    vpype: false,
    vpypeAvailable: false,
  },
  debug: false,
  args: EMPTY_ARG_STATE,
  interactive: EMPTY_INTERACTIVE_STATE,
};

function classNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

function normalizeArgsState(state?: Partial<ArgState> | null): ArgState {
  return {
    specs: Array.isArray(state?.specs) ? state!.specs! : [],
    values: state?.values ? { ...state.values } : {},
    raw: state?.raw ?? "",
  };
}

function normalizeInteractiveState(state?: Partial<InteractiveState> | null): InteractiveState {
  const manifest = state?.manifest ?? {};
  return {
    manifest: {
      canvas: {
        width: manifest.canvas?.width ?? 0,
        height: manifest.canvas?.height ?? 0,
      },
      tools: Array.isArray(manifest.tools) ? manifest.tools : [],
      regions: Array.isArray(manifest.regions) ? manifest.regions : [],
      defaultTool: manifest.defaultTool ?? "",
    },
  };
}

function normalizeServerState(state?: Partial<ServerState> | null): ServerState {
  return {
    sketch: state?.sketch ?? "",
    event: state?.event ?? { type: "" },
    logs: Array.isArray(state?.logs) ? state!.logs! : [],
    optim: {
      llo: !!state?.optim?.llo,
      vpype: !!state?.optim?.vpype,
      vpypeAvailable: !!state?.optim?.vpypeAvailable,
    },
    debug: !!state?.debug,
    args: normalizeArgsState(state?.args),
    interactive: normalizeInteractiveState(state?.interactive),
  };
}

function argStep(spec: ArgSpec) {
  return spec.type === "int" ? "1" : "0.01";
}

function argMeta(spec: ArgSpec) {
  if (spec.type === "bool") {
    return "bool";
  }

  const bounds =
    spec.min != null || spec.max != null
      ? `${spec.min != null ? spec.min : "-inf"} .. ${spec.max != null ? spec.max : "inf"}`
      : "";

  return bounds ? `${spec.type}  ${bounds}` : spec.type;
}

function sliderValue(spec: ArgSpec, value: string) {
  if (value !== "") {
    return value;
  }
  if (spec.default) {
    return spec.default;
  }
  if (spec.min != null) {
    return String(spec.min);
  }
  return "0";
}

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function requestMaybeJSON<T>(url: string): Promise<T | null> {
  const res = await fetch(url);
  if (!res.ok) {
    return null;
  }
  return res.json();
}

function filenameFromDisposition(disposition: string) {
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch (_err) {
      return utf8Match[1];
    }
  }

  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  return plainMatch?.[1] ?? "";
}

function StatusPill(props: { eventType: BuildEventType; connected: boolean }) {
  const { eventType, connected } = props;

  let tone = "border-white/10 bg-white/5 text-zinc-300";
  let label = "idle";

  if (!connected) {
    tone = "border-fault/30 bg-fault/10 text-fault";
    label = "disconnected";
  } else if (eventType === "ready") {
    tone = "border-ready/35 bg-ready/10 text-ready";
    label = "ready";
  } else if (eventType === "building" || eventType === "log") {
    tone = "border-build/35 bg-build/10 text-build";
    label = "building";
  } else if (eventType === "error") {
    tone = "border-fault/35 bg-fault/10 text-fault";
    label = "error";
  }

  return (
    <span
      className={classNames(
        "inline-flex min-w-[7.5rem] items-center justify-center rounded-full border px-3 py-1 text-[11px] font-medium uppercase tracking-[0.24em]",
        tone,
      )}
    >
      {label}
    </span>
  );
}

function ToggleChip(props: {
  label: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <label
      className={classNames(
        "inline-flex cursor-pointer items-center gap-2 rounded-full border px-3 py-2 text-xs tracking-[0.2em] uppercase transition",
        props.disabled
          ? "cursor-not-allowed border-white/10 bg-white/[0.03] text-zinc-500"
          : props.checked
            ? "border-signal/40 bg-signal/10 text-signal"
            : "border-white/10 bg-white/[0.03] text-zinc-300 hover:border-white/20 hover:bg-white/[0.05]",
      )}
    >
      <input
        className="hidden"
        type="checkbox"
        checked={props.checked}
        disabled={props.disabled}
        onChange={(ev) => props.onChange(ev.currentTarget.checked)}
      />
      <span
        className={classNames(
          "h-2.5 w-2.5 rounded-full",
          props.checked ? "bg-current" : "bg-zinc-600",
        )}
      />
      <span>{props.label}</span>
    </label>
  );
}

function ArgFieldCard(props: {
  spec: ArgSpec;
  value: string;
  onChange: (value: string) => void;
}) {
  const { spec, value, onChange } = props;

  if (spec.type === "bool") {
    const checked = value === "true";
    return (
      <div className="rounded-[24px] border border-white/10 bg-white/[0.03] p-4 shadow-[0_20px_80px_-50px_rgba(0,0,0,0.85)]">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="text-sm font-semibold text-white">{spec.name}</div>
            {spec.description ? (
              <p className="text-xs leading-5 text-zinc-400">{spec.description}</p>
            ) : null}
          </div>
          <label className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-black/20 px-3 py-2 text-xs uppercase tracking-[0.2em] text-zinc-200">
            <input
              type="checkbox"
              checked={checked}
              onChange={(ev) => onChange(ev.currentTarget.checked ? "true" : "false")}
            />
            <span>{checked ? "on" : "off"}</span>
          </label>
        </div>
      </div>
    );
  }

  const sliderReady = spec.min != null && spec.max != null;
  const currentValue = sliderValue(spec, value);

  return (
    <div className="rounded-[24px] border border-white/10 bg-white/[0.03] p-4 shadow-[0_20px_80px_-50px_rgba(0,0,0,0.85)]">
      <div className="mb-3 flex items-start justify-between gap-4">
        <div className="space-y-1">
          <div className="text-sm font-semibold text-white">{spec.name}</div>
          {spec.description ? (
            <p className="text-xs leading-5 text-zinc-400">{spec.description}</p>
          ) : null}
        </div>
        <div className="rounded-full border border-white/10 bg-black/20 px-2.5 py-1 text-[10px] uppercase tracking-[0.2em] text-zinc-500">
          {argMeta(spec)}
        </div>
      </div>

      <div className="flex items-center gap-3">
        <input
          type="number"
          value={currentValue}
          step={argStep(spec)}
          min={spec.min}
          max={spec.max}
          onChange={(ev) => onChange(ev.currentTarget.value)}
          className="w-28 rounded-2xl border border-white/10 bg-black/30 px-3 py-2 text-sm text-white outline-none transition focus:border-signal/50 focus:bg-black/40"
        />

        {sliderReady ? (
          <input
            type="range"
            min={spec.min}
            max={spec.max}
            step={argStep(spec)}
            value={currentValue}
            onChange={(ev) => onChange(ev.currentTarget.value)}
            className="h-2 w-full cursor-pointer accent-[#74d3ff]"
          />
        ) : null}
      </div>
    </div>
  );
}

function InteractiveToolButton(props: {
  tool: InteractiveTool;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={props.onSelect}
      title={props.tool.description || props.tool.label}
      className={classNames(
        "rounded-[20px] border px-3 py-3 text-left transition",
        props.selected
          ? "border-signal/45 bg-signal/12 text-signal shadow-[0_0_0_1px_rgba(116,211,255,0.15)]"
          : "border-white/10 bg-white/[0.03] text-zinc-200 hover:border-white/20 hover:bg-white/[0.06]",
      )}
    >
      <div className="text-sm font-semibold">{props.tool.label}</div>
      {props.tool.description ? (
        <div className="mt-1 text-xs leading-5 text-zinc-400">{props.tool.description}</div>
      ) : null}
    </button>
  );
}

function PreviewOverlay(props: {
  manifest: InteractiveManifest;
  visible: boolean;
  onRegionClick: (regionID: string) => void;
}) {
  const { manifest, visible, onRegionClick } = props;

  if (!visible || !manifest.canvas.width || !manifest.canvas.height || manifest.regions.length === 0) {
    return null;
  }

  return (
    <svg
      viewBox={`0 0 ${manifest.canvas.width} ${manifest.canvas.height}`}
      className="pointer-events-auto absolute inset-0 h-full w-full"
      aria-hidden="true"
    >
      {manifest.regions.map((region) => (
        <g key={region.id}>
          <rect
            x={region.x}
            y={region.y}
            width={region.width}
            height={region.height}
            rx="6"
            ry="6"
            fill={region.fill || "rgba(116, 211, 255, 0.05)"}
            stroke={region.stroke || "rgba(116, 211, 255, 0.32)"}
            strokeWidth="1.5"
            className="cursor-pointer transition hover:stroke-[#f5c24f]"
            onClick={() => onRegionClick(region.id)}
          >
            {region.hint ? <title>{region.hint}</title> : null}
          </rect>
          {region.label ? (
            <text
              x={region.x + region.width / 2}
              y={region.y + region.height / 2}
              fill={region.textColor || "#74d3ff"}
              textAnchor="middle"
              dominantBaseline="central"
              style={{
                fontFamily: '"IBM Plex Mono", monospace',
                fontSize: "11px",
                letterSpacing: "0.08em",
                pointerEvents: "none",
              }}
            >
              {region.label}
            </text>
          ) : null}
        </g>
      ))}
    </svg>
  );
}

function App() {
  const [serverState, setServerState] = React.useState<ServerState>(EMPTY_SERVER_STATE);
  const [gallerySketches, setGallerySketches] = React.useState<string[]>([]);
  const [draftValues, setDraftValues] = React.useState<Record<string, string>>({});
  const [draftRaw, setDraftRaw] = React.useState("");
  const [selectedTool, setSelectedTool] = React.useState("");
  const [previewSrc, setPreviewSrc] = React.useState("");
  const [imageLoaded, setImageLoaded] = React.useState(false);
  const [zoom, setZoom] = React.useState(100);
  const [gcodeFlavor, setGcodeFlavor] = React.useState("vanilla");
  const [exporting, setExporting] = React.useState(false);
  const [connected, setConnected] = React.useState(true);
  const deferredLogs = React.useDeferredValue(serverState.logs);

  const displayEventType =
    serverState.event.type === "log" ? "building" : serverState.event.type;
  const hasArgs =
    serverState.args.specs.length > 0 || serverState.args.raw.trim().length > 0;
  const hasInteractive =
    serverState.interactive.manifest.tools.length > 0 ||
    serverState.interactive.manifest.regions.length > 0;
  const showSidebar = hasArgs || hasInteractive;

  function syncPreview(event: BuildEvent) {
    if (event.type === "ready" && event.file) {
      setImageLoaded(false);
      setPreviewSrc(`/out/${event.file}?t=${Date.now()}`);
    }
  }

  function applyFullState(next: Partial<ServerState> | null | undefined) {
    const normalized = normalizeServerState(next);
    React.startTransition(() => {
      setServerState(normalized);
    });
    syncPreview(normalized.event);
  }

  async function refreshArgsState() {
    try {
      const next = await requestMaybeJSON<ArgState>("/api/args");
      if (!next) {
        return;
      }
      const normalized = normalizeArgsState(next);
      React.startTransition(() => {
        setServerState((current) => ({
          ...current,
          args: normalized,
        }));
      });
    } catch (_err) {
      // Ignore transient refresh failures while builds are in flight.
    }
  }

  async function refreshInteractiveState() {
    try {
      const next = await requestMaybeJSON<InteractiveState>("/api/interactive");
      if (!next) {
        return;
      }
      const normalized = normalizeInteractiveState(next);
      React.startTransition(() => {
        setServerState((current) => ({
          ...current,
          interactive: normalized,
        }));
      });
    } catch (_err) {
      // Ignore transient refresh failures while builds are in flight.
    }
  }

  async function refreshServerState() {
    const next = await requestJSON<ServerState>("/api/state");
    applyFullState(next);
  }

  React.useEffect(() => {
    let disposed = false;
    let events: EventSource | null = null;

    function handleEvent(event: BuildEvent) {
      setConnected(true);

      if (event.type === "building") {
        React.startTransition(() => {
          setServerState((current) => ({
            ...current,
            event,
            logs: [],
          }));
        });
        void refreshArgsState();
        return;
      }

      if (event.type === "ready") {
        React.startTransition(() => {
          setServerState((current) => ({
            ...current,
            event,
          }));
        });
        syncPreview(event);
        void refreshArgsState();
        void refreshInteractiveState();
        return;
      }

      if (event.type === "error") {
        React.startTransition(() => {
          setServerState((current) => ({
            ...current,
            event,
          }));
        });
        return;
      }

      if (event.type === "log") {
        React.startTransition(() => {
          setServerState((current) => ({
            ...current,
            event: current.event.type === "building" ? current.event : { type: "building" },
            logs: [...current.logs, event.log || ""].slice(-300),
          }));
        });
      }
    }

    async function boot() {
      try {
        const [sketches, state] = await Promise.all([
          requestMaybeJSON<string[]>("/api/sketches"),
          requestJSON<ServerState>("/api/state"),
        ]);

        if (disposed) {
          return;
        }

        setGallerySketches(Array.isArray(sketches) ? sketches : []);
        applyFullState(state);
        setConnected(true);

        events = new EventSource("/events");
        events.onmessage = (message) => {
          try {
            handleEvent(JSON.parse(message.data));
          } catch (_err) {
            // Ignore malformed event payloads.
          }
        };
        events.onerror = () => {
          setConnected(false);
        };

        if (Array.isArray(sketches) && sketches.length > 0 && !state.event.type) {
          const first = sketches[0];
          await fetch(`/api/select?name=${encodeURIComponent(first)}`);
        }
      } catch (err) {
        if (disposed) {
          return;
        }

        const msg = err instanceof Error ? err.message : String(err);
        setConnected(false);
        applyFullState({
          ...EMPTY_SERVER_STATE,
          event: {
            type: "error",
            msg: `Failed to connect: ${msg}`,
          },
        });
      }
    }

    void boot();

    return () => {
      disposed = true;
      if (events) {
        events.close();
      }
    };
  }, []);

  React.useEffect(() => {
    setDraftValues({ ...serverState.args.values });
    setDraftRaw(serverState.args.raw);
  }, [serverState.args]);

  React.useEffect(() => {
    const tools = serverState.interactive.manifest.tools;
    if (tools.length === 0) {
      if (selectedTool !== "") {
        setSelectedTool("");
      }
      return;
    }

    const stillValid = tools.some((tool) => tool.id === selectedTool);
    if (stillValid) {
      return;
    }

    setSelectedTool(serverState.interactive.manifest.defaultTool || tools[0].id);
  }, [serverState.interactive.manifest, selectedTool]);

  async function updateDebug(next: boolean) {
    React.startTransition(() => {
      setServerState((current) => ({
        ...current,
        debug: next,
      }));
    });

    try {
      await requestJSON("/api/debug", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ debug: next }),
      });
    } catch (err) {
      window.alert(`Failed to update debug mode: ${err instanceof Error ? err.message : String(err)}`);
      void refreshServerState();
    }
  }

  async function updateOptimisation(key: "llo" | "vpype", next: boolean) {
    React.startTransition(() => {
      setServerState((current) => ({
        ...current,
        optim: {
          ...current.optim,
          [key]: next,
        },
      }));
    });

    try {
      await requestJSON("/api/optimise", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          llo: key === "llo" ? next : serverState.optim.llo,
          vpype: key === "vpype" ? next : serverState.optim.vpype,
        }),
      });
    } catch (err) {
      window.alert(
        `Failed to update optimisation flags: ${err instanceof Error ? err.message : String(err)}`,
      );
      void refreshServerState();
    }
  }

  async function applyArgs() {
    try {
      const next = await requestJSON<ArgState>("/api/args", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          values: draftValues,
          raw: draftRaw,
        }),
      });

      React.startTransition(() => {
        setServerState((current) => ({
          ...current,
          event: { type: "building" },
          logs: [],
          args: normalizeArgsState(next),
        }));
      });
    } catch (err) {
      window.alert(`Failed to update args: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  async function resetArgs() {
    try {
      const next = await requestJSON<ArgState>("/api/args", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reset: true }),
      });

      React.startTransition(() => {
        setServerState((current) => ({
          ...current,
          event: { type: "building" },
          logs: [],
          args: normalizeArgsState(next),
        }));
      });
    } catch (err) {
      window.alert(`Failed to reset args: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  async function selectSketch(name: string) {
    React.startTransition(() => {
      setServerState((current) => ({
        ...current,
        sketch: name,
        event: { type: "building" },
        logs: [],
      }));
    });

    try {
      const res = await fetch(`/api/select?name=${encodeURIComponent(name)}`);
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "Sketch selection failed");
      }
    } catch (err) {
      window.alert(`Failed to select sketch: ${err instanceof Error ? err.message : String(err)}`);
      void refreshServerState();
    }
  }

  async function applyInteractive(regionID: string) {
    if (!selectedTool) {
      window.alert("Select an interactive tool first.");
      return;
    }

    try {
      const next = await requestJSON<InteractiveState>("/api/interactive", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tool: selectedTool,
          region: regionID,
        }),
      });

      React.startTransition(() => {
        setServerState((current) => ({
          ...current,
          event: { type: "building" },
          logs: [],
          interactive: normalizeInteractiveState(next),
        }));
      });
    } catch (err) {
      window.alert(
        `Failed to update interactive state: ${err instanceof Error ? err.message : String(err)}`,
      );
    }
  }

  async function downloadGCode() {
    if (exporting || displayEventType !== "ready") {
      return;
    }

    setExporting(true);

    try {
      const res = await fetch(`/api/export/gcode?flavor=${encodeURIComponent(gcodeFlavor)}`);
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "Export failed");
      }

      const blob = await res.blob();
      const disposition = res.headers.get("Content-Disposition") || "";
      const filename = filenameFromDisposition(disposition) || "sketch.gcode";
      const url = URL.createObjectURL(blob);

      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (err) {
      window.alert(`Failed to export G-code: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setExporting(false);
    }
  }

  return (
    <div className="min-h-full bg-[radial-gradient(circle_at_top,rgba(116,211,255,0.08),transparent_24%),radial-gradient(circle_at_bottom,rgba(255,154,87,0.08),transparent_26%)]">
      <div className="mx-auto flex min-h-full max-w-[1800px] flex-col px-4 py-4 lg:px-6">
        <header className="mb-4 rounded-[32px] border border-white/10 bg-black/20 px-5 py-4 shadow-[0_30px_120px_-60px_rgba(0,0,0,0.95)] backdrop-blur-xl">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
            <div className="space-y-1">
              <div className="text-[11px] uppercase tracking-[0.36em] text-zinc-500">
                go-pen
              </div>
              <div className="text-2xl font-semibold text-white">Live Preview Console</div>
              <p className="max-w-2xl text-sm leading-6 text-zinc-400">
                Tune sketch arguments, pin interactive decisions, inspect solver output, and export
                printer-specific G-code from the same live surface.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {gallerySketches.length > 0 ? (
                <select
                  value={serverState.sketch || gallerySketches[0]}
                  onChange={(ev) => void selectSketch(ev.currentTarget.value)}
                  className="rounded-full border border-white/10 bg-black/30 px-4 py-2 text-sm text-white outline-none transition hover:border-white/20 focus:border-signal/50"
                >
                  {gallerySketches.map((sketch) => (
                    <option key={sketch} value={sketch}>
                      {sketch}
                    </option>
                  ))}
                </select>
              ) : null}

              <ToggleChip
                label="debug"
                checked={serverState.debug}
                onChange={(next) => void updateDebug(next)}
              />
              <ToggleChip
                label="llo"
                checked={serverState.optim.llo}
                onChange={(next) => void updateOptimisation("llo", next)}
              />
              <ToggleChip
                label="vpype"
                checked={serverState.optim.vpype}
                disabled={!serverState.optim.vpypeAvailable}
                onChange={(next) => void updateOptimisation("vpype", next)}
              />

              <select
                value={gcodeFlavor}
                onChange={(ev) => setGcodeFlavor(ev.currentTarget.value)}
                className="rounded-full border border-white/10 bg-black/30 px-4 py-2 text-sm text-white outline-none transition hover:border-white/20 focus:border-ember/50"
              >
                <option value="vanilla">gcode: vanilla</option>
                <option value="mk4s">gcode: mk4s</option>
              </select>

              <button
                type="button"
                disabled={exporting || displayEventType !== "ready"}
                onClick={() => void downloadGCode()}
                className={classNames(
                  "rounded-full border px-4 py-2 text-sm font-medium transition",
                  exporting || displayEventType !== "ready"
                    ? "cursor-not-allowed border-white/10 bg-white/[0.04] text-zinc-500"
                    : "border-ember/40 bg-ember/12 text-ember hover:border-ember/60 hover:bg-ember/18",
                )}
              >
                {exporting ? "exporting..." : "download gcode"}
              </button>

              <div className="ml-1 flex items-center gap-3 rounded-full border border-white/10 bg-black/25 px-4 py-2">
                <label className="text-[11px] uppercase tracking-[0.26em] text-zinc-500">zoom</label>
                <input
                  type="range"
                  min="40"
                  max="180"
                  step="5"
                  value={zoom}
                  onChange={(ev) => setZoom(Number(ev.currentTarget.value))}
                  className="w-28 accent-[#74d3ff]"
                />
                <span className="w-12 text-right text-xs text-zinc-300">{zoom}%</span>
              </div>

              <StatusPill eventType={displayEventType} connected={connected} />
            </div>
          </div>
        </header>

        <div
          className={classNames(
            "grid flex-1 gap-4",
            showSidebar ? "xl:grid-cols-[minmax(0,1fr)_24rem]" : "grid-cols-1",
          )}
        >
          <section className="min-h-0 rounded-[36px] border border-white/10 bg-black/20 p-4 shadow-[0_40px_120px_-60px_rgba(0,0,0,0.95)] backdrop-blur-xl md:p-5">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3 border-b border-white/10 pb-4">
              <div>
                <div className="text-[11px] uppercase tracking-[0.3em] text-zinc-500">
                  preview
                </div>
                <div className="mt-1 text-sm text-zinc-300">
                  {displayEventType === "ready"
                    ? "Latest render with interactive overlay."
                    : displayEventType === "error"
                      ? "Build failed."
                      : "Waiting for the current build to complete."}
                </div>
              </div>

              <div className="rounded-full border border-white/10 bg-black/25 px-3 py-1.5 text-xs uppercase tracking-[0.22em] text-zinc-400">
                {serverState.interactive.manifest.regions.length} interactive fields
              </div>
            </div>

            <div className="flex h-full min-h-[420px] items-center justify-center overflow-hidden rounded-[28px] border border-white/8 bg-[radial-gradient(circle_at_top,rgba(116,211,255,0.09),transparent_22%),radial-gradient(circle_at_bottom,rgba(255,154,87,0.09),transparent_26%),linear-gradient(180deg,rgba(7,9,13,0.86),rgba(15,20,29,0.92))] p-4">
              {displayEventType === "error" ? (
                <div className="max-w-2xl rounded-[28px] border border-fault/20 bg-fault/8 p-6 text-left shadow-[0_30px_100px_-70px_rgba(255,127,127,0.8)]">
                  <div className="text-[11px] uppercase tracking-[0.28em] text-fault">error</div>
                  <h2 className="mt-3 text-2xl font-semibold text-white">Live build failed</h2>
                  <p className="mt-3 whitespace-pre-wrap text-sm leading-6 text-zinc-300">
                    {serverState.event.msg || "The server reported an unspecified error."}
                  </p>
                </div>
              ) : displayEventType === "building" ? (
                <div className="flex h-full w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-white/10 bg-[#05070b]">
                  <div className="flex items-center justify-between border-b border-white/10 px-5 py-4">
                    <div>
                      <div className="text-[11px] uppercase tracking-[0.28em] text-build">build log</div>
                      <div className="mt-1 text-sm text-zinc-400">
                        stdout, stderr, and pipeline steps stream here while the sketch rebuilds.
                      </div>
                    </div>
                    <div className="rounded-full border border-build/25 bg-build/10 px-3 py-1 text-xs uppercase tracking-[0.22em] text-build">
                      {deferredLogs.length} lines
                    </div>
                  </div>
                  <pre className="min-h-0 flex-1 overflow-auto px-5 py-4 text-sm leading-6 text-zinc-300">
                    {deferredLogs.length > 0
                      ? deferredLogs.join("\n")
                      : "Waiting for compiler output..."}
                  </pre>
                </div>
              ) : previewSrc ? (
                <div className="relative inline-flex max-h-full max-w-full items-center justify-center">
                  <img
                    src={previewSrc}
                    alt="go-pen preview"
                    onLoad={() => setImageLoaded(true)}
                    style={{ transform: `scale(${zoom / 100})`, transformOrigin: "center center" }}
                    className="max-h-[calc(100vh-18rem)] max-w-full rounded-[18px] bg-white shadow-[0_50px_120px_-60px_rgba(0,0,0,0.95)] transition-transform"
                  />
                  <PreviewOverlay
                    manifest={serverState.interactive.manifest}
                    visible={imageLoaded && hasInteractive}
                    onRegionClick={(regionID) => void applyInteractive(regionID)}
                  />
                </div>
              ) : (
                <div className="max-w-xl text-center">
                  <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-500">
                    standby
                  </div>
                  <h2 className="mt-3 text-2xl font-semibold text-white">Waiting for preview output</h2>
                  <p className="mt-3 text-sm leading-6 text-zinc-400">
                    Select a sketch or wait for the current build. Once a render is ready, the
                    preview and interactive overlay will appear here.
                  </p>
                </div>
              )}
            </div>
          </section>

          {showSidebar ? (
            <aside className="min-h-0 rounded-[36px] border border-white/10 bg-black/20 p-4 shadow-[0_40px_120px_-60px_rgba(0,0,0,0.95)] backdrop-blur-xl md:p-5">
              <div className="mb-4 border-b border-white/10 pb-4">
                <div className="text-[11px] uppercase tracking-[0.32em] text-zinc-500">controls</div>
                <div className="mt-2 text-sm leading-6 text-zinc-400">
                  Live argument editing and manual tile pinning share the same persisted profile.
                </div>
              </div>

              <div className="flex h-full flex-col gap-5 overflow-hidden">
                {hasInteractive ? (
                  <section className="space-y-3">
                    <div className="flex items-center justify-between">
                      <div className="text-sm font-semibold text-white">Interactive tools</div>
                      <div className="rounded-full border border-white/10 bg-black/25 px-2.5 py-1 text-[10px] uppercase tracking-[0.24em] text-zinc-500">
                        {serverState.interactive.manifest.tools.length} tools
                      </div>
                    </div>

                    <p className="text-xs leading-5 text-zinc-400">
                      Choose a tile override, then click a field in the preview overlay. The solver
                      keeps your pinned fields fixed and re-optimizes the remaining grid.
                    </p>

                    <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-1">
                      {serverState.interactive.manifest.tools.map((tool) => (
                        <InteractiveToolButton
                          key={tool.id}
                          tool={tool}
                          selected={tool.id === selectedTool}
                          onSelect={() => setSelectedTool(tool.id)}
                        />
                      ))}
                    </div>
                  </section>
                ) : null}

                {hasArgs ? (
                  <section className="flex min-h-0 flex-1 flex-col gap-4">
                    <div className="flex items-center justify-between">
                      <div className="text-sm font-semibold text-white">Arguments</div>
                      <div className="rounded-full border border-white/10 bg-black/25 px-2.5 py-1 text-[10px] uppercase tracking-[0.24em] text-zinc-500">
                        {serverState.args.specs.length} typed
                      </div>
                    </div>

                    <div className="min-h-0 space-y-3 overflow-auto pr-1">
                      {serverState.args.specs.map((spec) => (
                        <ArgFieldCard
                          key={spec.name}
                          spec={spec}
                          value={draftValues[spec.name] ?? ""}
                          onChange={(value) =>
                            setDraftValues((current) => ({
                              ...current,
                              [spec.name]: value,
                            }))
                          }
                        />
                      ))}

                      <div className="rounded-[24px] border border-white/10 bg-white/[0.03] p-4 shadow-[0_20px_80px_-50px_rgba(0,0,0,0.85)]">
                        <div className="mb-2 text-sm font-semibold text-white">extra args</div>
                        <p className="mb-3 text-xs leading-5 text-zinc-400">
                          Pass through additional `key=value` pairs that are not part of the typed
                          schema.
                        </p>
                        <textarea
                          value={draftRaw}
                          onChange={(ev) => setDraftRaw(ev.currentTarget.value)}
                          rows={4}
                          placeholder="variant=wave,scale=0.8"
                          className="w-full rounded-[20px] border border-white/10 bg-black/30 px-3 py-3 text-sm text-white outline-none transition focus:border-signal/50 focus:bg-black/40"
                        />
                      </div>
                    </div>

                    <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-1">
                      <button
                        type="button"
                        onClick={() => void applyArgs()}
                        className="rounded-full border border-signal/35 bg-signal/10 px-4 py-3 text-sm font-medium text-signal transition hover:border-signal/60 hover:bg-signal/16"
                      >
                        apply args
                      </button>
                      <button
                        type="button"
                        onClick={() => void resetArgs()}
                        className="rounded-full border border-white/10 bg-white/[0.03] px-4 py-3 text-sm font-medium text-zinc-200 transition hover:border-white/20 hover:bg-white/[0.06]"
                      >
                        reset
                      </button>
                    </div>
                  </section>
                ) : null}
              </div>
            </aside>
          ) : null}
        </div>
      </div>
    </div>
  );
}

const root = document.getElementById("root");
if (!root) {
  throw new Error("missing root element");
}

ReactDOM.createRoot(root).render(<App />);
