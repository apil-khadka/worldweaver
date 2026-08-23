/**
 * element-drawer.ts — searchable, categorised element picker
 *
 * # Why this replaces the palette strip
 *
 * Material selection was a horizontal row of hardcoded buttons with inline hex
 * colours. It held eleven entries, could not scroll usefully, and had no way to
 * describe what a material does. With 44 registered elements it was not a layout
 * that could be extended — it was the wrong shape.
 *
 * More importantly the strip duplicated the material list. The Go registry, the HTML
 * buttons and each renderer's colour table each carried their own copy, kept in step
 * by hand, which is why eight defined materials had no button at all and could never
 * be placed.
 *
 * This drawer is built entirely from `GET /api/elements`, so the server is the single
 * source of truth. Adding an element server-side makes it appear here with the right
 * colour, grouping, properties and reaction list, with no frontend edit.
 *
 * # Reactions are the point
 *
 * Selecting an element shows what it reacts with and the real chemical equation.
 * Without that the element set is 44 coloured powders and a player has no way to
 * discover that magnesium burns in carbon dioxide except by accident. The reaction
 * list is what turns the catalogue into something you can reason about.
 */

export interface ElementReaction {
  with: string;
  produces: string;
  equation: string;
  needsHeat: boolean;
  minTempC?: number;
  catalyst?: string;
  exothermic: boolean;
}

export interface ElementInfo {
  id: number;
  name: string;
  symbol?: string;
  atomic?: number;
  category: string;
  categoryId: number;
  phase: string;
  placeable: boolean;
  density: number;
  meltingPoint: number;
  boilingPoint: number;
  ignitionTemp?: number;
  flammability: number;
  conductivity: number;
  reactivity: number;
  colour: [number, number, number, number];
  flavour?: string;
  reactions?: ElementReaction[];
}

export interface ElementCategory {
  id: number;
  name: string;
  elements: ElementInfo[];
}

export interface ElementCatalogue {
  count: number;
  categories: ElementCategory[];
}

/** Called when the player picks an element. */
export type SelectHandler = (id: number, info: ElementInfo) => void;

export class ElementDrawer {
  private catalogue: ElementCatalogue | null = null;
  private flat: ElementInfo[] = [];
  private selectedID: number | null = null;
  private query = "";
  private open = false;

  private rootEl: HTMLElement | null = null;
  private listEl: HTMLElement | null = null;
  private searchEl: HTMLInputElement | null = null;
  private detailEl: HTMLElement | null = null;

  constructor(private readonly onSelect: SelectHandler) {}

  /**
   * Loads the catalogue and builds the drawer.
   *
   * Failure is non-fatal and deliberately quiet in the UI: the elements are an
   * addition to the existing forces and tools, so a player whose catalogue request
   * failed should still be able to play with what the legacy palette offers rather
   * than face a blocking error.
   */
  async load(): Promise<boolean> {
    try {
      const res = await fetch("/api/elements");
      if (!res.ok) {
        console.warn("[elements] catalogue request failed:", res.status);
        return false;
      }
      this.catalogue = (await res.json()) as ElementCatalogue;
      this.flat = this.catalogue.categories.flatMap((c) => c.elements);
      this.build();
      return true;
    } catch (e) {
      console.warn("[elements] catalogue unavailable:", e);
      return false;
    }
  }

  /** Every element, for callers that need the colour table. */
  all(): ElementInfo[] {
    return this.flat;
  }

  private build(): void {
    this.rootEl = document.getElementById("element-drawer");
    this.listEl = document.getElementById("element-list");
    this.searchEl = document.getElementById("element-search") as HTMLInputElement | null;
    this.detailEl = document.getElementById("element-detail");
    if (!this.rootEl || !this.listEl) return;

    this.searchEl?.addEventListener("input", () => {
      this.query = (this.searchEl?.value ?? "").trim().toLowerCase();
      this.renderList();
    });

    // Escape closes the drawer, and does so from inside the search box too — a
    // player who has just typed a query is exactly who wants to dismiss it.
    this.searchEl?.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        this.setOpen(false);
      }
    });

    document.getElementById("element-drawer-close")
      ?.addEventListener("click", () => this.setOpen(false));
    document.getElementById("element-drawer-toggle")
      ?.addEventListener("click", () => this.setOpen(!this.open));

    this.renderList();
  }

  /** Opens or closes the drawer, focusing search on open. */
  setOpen(open: boolean): void {
    this.open = open;
    this.rootEl?.classList.toggle("open", open);
    document.getElementById("element-drawer-toggle")
      ?.classList.toggle("active", open);
    if (open) {
      // Focusing search on open means the fastest path to any of 44 elements is to
      // open and type, rather than to open and scan.
      this.searchEl?.focus();
      this.searchEl?.select();
    }
  }

  isOpen(): boolean {
    return this.open;
  }

  /** Matches an element against the search query by name, symbol or category. */
  private matches(e: ElementInfo): boolean {
    if (this.query === "") return true;
    const q = this.query;
    return (
      e.name.toLowerCase().includes(q) ||
      (e.symbol?.toLowerCase() === q) ||
      (e.symbol?.toLowerCase().startsWith(q) ?? false) ||
      e.category.toLowerCase().includes(q) ||
      e.phase.toLowerCase().includes(q)
    );
  }

  private renderList(): void {
    if (!this.listEl || !this.catalogue) return;
    this.listEl.textContent = "";

    let shown = 0;

    for (const cat of this.catalogue.categories) {
      const hits = cat.elements.filter((e) => this.matches(e));
      if (hits.length === 0) continue;

      const section = document.createElement("div");
      section.className = "el-section";

      const heading = document.createElement("div");
      heading.className = "el-section-title";
      heading.textContent = cat.name;
      section.appendChild(heading);

      const grid = document.createElement("div");
      grid.className = "el-grid";

      for (const el of hits) {
        grid.appendChild(this.buildSwatch(el));
        shown++;
      }

      section.appendChild(grid);
      this.listEl.appendChild(section);
    }

    if (shown === 0) {
      const empty = document.createElement("div");
      empty.className = "el-empty";
      empty.textContent = `Nothing matches "${this.query}"`;
      this.listEl.appendChild(empty);
    }
  }

  private buildSwatch(el: ElementInfo): HTMLButtonElement {
    const btn = document.createElement("button");
    btn.className = "el-swatch";
    btn.dataset["material"] = String(el.id);
    btn.type = "button";
    if (el.id === this.selectedID) btn.classList.add("active");

    const [r, g, b] = el.colour;
    btn.style.setProperty("--c", `rgb(${r},${g},${b})`);

    const dot = document.createElement("span");
    dot.className = "el-dot";

    const label = document.createElement("span");
    label.className = "el-label";
    label.textContent = el.name;

    btn.appendChild(dot);
    btn.appendChild(label);

    if (el.symbol) {
      const sym = document.createElement("span");
      sym.className = "el-symbol";
      sym.textContent = el.symbol;
      btn.appendChild(sym);
    }

    // Hovering previews without committing, so a player can read what several
    // elements do before choosing one.
    btn.addEventListener("mouseenter", () => this.renderDetail(el));
    btn.addEventListener("click", () => this.select(el));

    return btn;
  }

  private select(el: ElementInfo): void {
    this.selectedID = el.id;
    this.listEl?.querySelectorAll(".el-swatch").forEach((b) => {
      b.classList.toggle("active", b.getAttribute("data-material") === String(el.id));
    });
    this.renderDetail(el);
    this.onSelect(el.id, el);
  }

  /** Renders the selected element's properties and reactions. */
  private renderDetail(el: ElementInfo): void {
    if (!this.detailEl) return;
    this.detailEl.textContent = "";

    const head = document.createElement("div");
    head.className = "el-detail-head";
    const [r, g, b] = el.colour;
    head.style.setProperty("--c", `rgb(${r},${g},${b})`);

    const title = document.createElement("div");
    title.className = "el-detail-title";
    title.textContent = el.name;
    head.appendChild(title);

    if (el.symbol) {
      const badge = document.createElement("span");
      badge.className = "el-detail-symbol";
      badge.textContent = el.atomic ? `${el.symbol} · ${el.atomic}` : el.symbol;
      head.appendChild(badge);
    }
    this.detailEl.appendChild(head);

    if (el.flavour) {
      const flav = document.createElement("div");
      flav.className = "el-detail-flavour";
      flav.textContent = el.flavour;
      this.detailEl.appendChild(flav);
    }

    // Properties. Temperatures are real values from the CRC Handbook, so they are
    // worth showing: "melts at 3422 °C" explains why tungsten survives lava without
    // the game having to say so.
    const props = document.createElement("dl");
    props.className = "el-props";
    const addProp = (k: string, v: string) => {
      const dt = document.createElement("dt");
      dt.textContent = k;
      const dd = document.createElement("dd");
      dd.textContent = v;
      props.appendChild(dt);
      props.appendChild(dd);
    };

    addProp("State", el.phase);
    addProp("Density", `${el.density.toLocaleString()} kg/m³`);
    if (el.meltingPoint > -30000 && el.meltingPoint < 30000) {
      addProp("Melts", `${el.meltingPoint} °C`);
    }
    if (el.boilingPoint > -30000 && el.boilingPoint < 30000) {
      addProp("Boils", `${el.boilingPoint} °C`);
    }
    if (el.ignitionTemp !== undefined) {
      addProp("Ignites", `${el.ignitionTemp} °C`);
    }
    this.detailEl.appendChild(props);

    // Reactions — the reason the drawer exists rather than a plain colour picker.
    const rx = el.reactions ?? [];
    const rxTitle = document.createElement("div");
    rxTitle.className = "el-rx-title";
    rxTitle.textContent = rx.length > 0
      ? `Reactions (${rx.length})`
      : "No known reactions";
    this.detailEl.appendChild(rxTitle);

    if (rx.length === 0) return;

    const list = document.createElement("ul");
    list.className = "el-rx-list";
    for (const r of rx) {
      const li = document.createElement("li");
      li.className = "el-rx";
      if (r.exothermic) li.classList.add("el-rx--exo");

      const pair = document.createElement("div");
      pair.className = "el-rx-pair";
      pair.textContent = `+ ${r.with} → ${r.produces}`;
      li.appendChild(pair);

      const eq = document.createElement("div");
      eq.className = "el-rx-eq";
      eq.textContent = r.equation;
      li.appendChild(eq);

      const conds: string[] = [];
      if (r.minTempC !== undefined) conds.push(`needs ${r.minTempC} °C`);
      if (r.catalyst) conds.push(`needs ${r.catalyst}`);
      if (r.exothermic) conds.push("releases heat");
      if (conds.length > 0) {
        const cond = document.createElement("div");
        cond.className = "el-rx-cond";
        cond.textContent = conds.join(" · ");
        li.appendChild(cond);
      }

      list.appendChild(li);
    }
    this.detailEl.appendChild(list);
  }
}
