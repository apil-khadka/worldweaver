import { useCallback, useEffect, useState } from "react";
import { fetchWorlds, isFull, playUrl, type WorldSummary } from "../api";
import Icon from "./Icon";

type LoadState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; worlds: WorldSummary[] };

const SKELETON_COUNT = 3;

export default function LiveWorlds() {
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const [reloadKey, setReloadKey] = useState(0);

  const retry = useCallback(() => {
    setState({ status: "loading" });
    setReloadKey((k) => k + 1);
  }, []);

  useEffect(() => {
    const controller = new AbortController();

    fetchWorlds(controller.signal)
      .then((worlds) => {
        if (controller.signal.aborted) return;
        setState({ status: "ready", worlds });
      })
      .catch((err: unknown) => {
        // An abort is a normal unmount / StrictMode remount, not a failure.
        if (controller.signal.aborted) return;
        setState({
          status: "error",
          message: err instanceof Error ? err.message : "Unknown error",
        });
      });

    return () => controller.abort();
  }, [reloadKey]);

  return (
    <section className="worlds" id="live-worlds">
      <div className="container">
        <div className="worlds__head">
          <div>
            <p className="section-eyebrow">Live right now</p>
            <h2 className="section-title">Join a world in progress</h2>
            <p className="section-lede">
              Every world below is a running simulation with its own terrain and
              its own history. Drop into one and you inherit whatever the last
              group built.
            </p>
          </div>

          <button
            type="button"
            className="button button--ghost button--sm worlds__refresh"
            onClick={retry}
            disabled={state.status === "loading"}
          >
            <Icon name="refresh" size={16} />
            {state.status === "loading" ? "Checking…" : "Refresh"}
          </button>
        </div>

        {/* One live region covers all three states so a screen reader is told
            what happened without the page ever going blank. */}
        <div aria-live="polite" aria-busy={state.status === "loading"}>
          {state.status === "loading" && <LoadingState />}
          {state.status === "error" && (
            <ErrorState message={state.message} onRetry={retry} />
          )}
          {state.status === "ready" &&
            (state.worlds.length === 0 ? (
              <EmptyState />
            ) : (
              <WorldGrid worlds={state.worlds} />
            ))}
        </div>
      </div>
    </section>
  );
}

function LoadingState() {
  return (
    <>
      <p className="worlds__status-text">Looking for running worlds…</p>
      <ul className="worlds__grid" role="list">
        {Array.from({ length: SKELETON_COUNT }).map((_, i) => (
          <li key={i} className="world-card world-card--skeleton" aria-hidden="true">
            <span className="skeleton skeleton--title" />
            <span className="skeleton skeleton--line" />
            <span className="skeleton skeleton--line skeleton--short" />
          </li>
        ))}
      </ul>
    </>
  );
}

function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div className="worlds__notice worlds__notice--error" role="group">
      <p className="worlds__notice-title">
        <Icon name="alert" size={20} />
        The world server is not answering
      </p>
      <p className="worlds__notice-body">
        The live list could not be loaded, so this is all we can show for now.
        If you are running WorldWeaver locally, start the Go server and try
        again.
      </p>
      <p className="worlds__notice-detail">Reported: {message}</p>
      <div className="worlds__notice-actions">
        <button type="button" className="button button--primary button--sm" onClick={onRetry}>
          <Icon name="refresh" size={16} />
          Try again
        </button>
        <a className="button button--ghost button--sm" href="/play">
          Open the client anyway
        </a>
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="worlds__notice" role="group">
      <p className="worlds__notice-title">
        <Icon name="globe" size={20} />
        No worlds are running yet
      </p>
      <p className="worlds__notice-body">
        Nobody has a world open at the moment. Create one from the lobby, pick a
        size, and it will show up here for everyone else to join.
      </p>
      <div className="worlds__notice-actions">
        <a className="button button--primary button--sm" href="/play">
          Create the first world
          <Icon name="arrow" size={16} />
        </a>
      </div>
    </div>
  );
}

function WorldGrid({ worlds }: { worlds: WorldSummary[] }) {
  return (
    <ul className="worlds__grid" role="list">
      {worlds.map((world) => (
        <WorldCard key={world.id} world={world} />
      ))}
    </ul>
  );
}

function WorldCard({ world }: { world: WorldSummary }) {
  const full = isFull(world);
  const free = Math.max(0, world.maxPlayers - world.playerCount);
  const fillPct =
    world.maxPlayers > 0
      ? Math.min(100, Math.round((world.playerCount / world.maxPlayers) * 100))
      : 0;

  return (
    <li className={`world-card${full ? " world-card--full" : ""}`}>
      <a className="world-card__link" href={playUrl(world.id)}>
        <span className="world-card__header">
          {/* Values below are server-provided strings rendered as text nodes. */}
          <span className="world-card__name">{world.name}</span>
          <span
            className={`world-card__badge${full ? " world-card__badge--full" : ""}`}
          >
            {full ? "Full" : `${free} free`}
          </span>
        </span>

        <span className="world-card__players">
          <Icon name="users" size={16} />
          <span>
            {world.playerCount} of {world.maxPlayers} players
          </span>
        </span>

        <span className="world-card__meter" aria-hidden="true">
          <span className="world-card__meter-fill" style={{ width: `${fillPct}%` }} />
        </span>

        <dl className="world-card__meta">
          <div className="world-card__meta-row">
            <dt>Size</dt>
            <dd>
              {world.width} × {world.height}
              {world.size !== "" ? ` · ${world.size}` : ""}
            </dd>
          </div>
          <div className="world-card__meta-row">
            <dt>Seed</dt>
            <dd>{world.seed}</dd>
          </div>
          <div className="world-card__meta-row">
            <dt>Opened by</dt>
            <dd>{world.creatorName}</dd>
          </div>
        </dl>

        <span className="world-card__cta">
          {full ? "Watch for a free slot" : "Join this world"}
          <Icon name="arrow" size={16} />
        </span>
      </a>
    </li>
  );
}
