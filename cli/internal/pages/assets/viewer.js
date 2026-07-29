/* gogo interactive viewer - orchestrator.
 *
 * Renders every <figure class="diagram"> on the page with very-nice-mermaid,
 * offline over file://. Loads as a CLASSIC script (ES-module src and fetch are
 * both blocked on file://) after vnm-browser.js, which puts the renderer on
 * `window.vnm`.
 *
 * Three tiers, in order, per figure:
 *   1. PREBUILT LAYOUT - window.GOGO_LAYOUTS[key], computed at authoring time by
 *      assets/vnm/layout.mjs. Works for ALL five gogo kinds and is the only tier
 *      that can render sequence / class / state, because routing raw DSL for
 *      those needs mermaid's detectType (a dynamic import that file:// blocks).
 *   2. INLINE DSL - flowchart family only, parsed in-browser by vnm's own
 *      dependency-free parser. Covers the common case with zero authoring deps.
 *   3. STATIC SVG - a <svg> pre-rendered beside the .mmd, shown as-is.
 * Anything left over renders an inline .err box; the summary always stays
 * readable. A figure never silently blanks.
 */
(function () {
  "use strict";

  var LAYOUTS = window.GOGO_LAYOUTS || {};
  var SEEDS = window.GOGO_LAYOUT || {}; // committed node positions (sidecar)
  var THEME = window.GOGO_THEME || "arch-light";
  var handles = {};

  function fail(figure, message) {
    var box = document.createElement("div");
    box.className = "err";
    box.textContent = message;
    var host = figure.querySelector(".diagram-host");
    if (host) host.replaceWith(box); else figure.appendChild(box);
  }

  /** Rehydrate a layout from layout.mjs. `model` carries Map/Set values that only
   *  deserializeModel can restore - plain JSON leaves them as objects and the
   *  renderer dies on "model.classDefs is not iterable". */
  function rehydrate(entry) {
    if (entry && entry._bare) return window.vnm.deserializeModel(entry._bare);
    if (entry && entry._model) {
      var copy = {};
      for (var k in entry) if (k !== "_model") copy[k] = entry[k];
      copy.model = window.vnm.deserializeModel(entry.model);
      return copy;
    }
    return entry;
  }

  function isFlowchart(dsl) {
    return /^\s*(flowchart|graph)\b/.test(dsl);
  }

  /** True for a half-width Before|After cell (a solo row is full width). */
  function isCompareCell(figure) {
    return (figure.classList.contains("compare-before") ||
            figure.classList.contains("compare-after")) &&
           !figure.classList.contains("compare-solo");
  }

  /** gogo diagram sources escape angle brackets (`A["/gogo:done &lt;slug&gt;"]`)
   *  because the same DSL also ships as a ```mermaid fence, and mermaid draws
   *  labels as HTML. vnm draws SVG <text>, which would show the entity
   *  literally - decode so both renderings match. Ampersand LAST so an escaped
   *  `&amp;lt;` survives as the literal `&lt;`. Mirrors layout.mjs. */
  function decodeEntities(s) {
    return s.replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&");
  }

  function mountOne(figure) {
    var key = figure.getAttribute("data-diagram") || "";
    var pre = figure.querySelector("pre.mermaid");
    var dsl = pre ? decodeEntities(pre.textContent) : "";
    var svg = figure.querySelector("svg");

    var host = document.createElement("div");
    host.className = "diagram-host";
    if (pre) pre.replaceWith(host); else figure.appendChild(host);

    var opts = {
      theme: THEME,
      // vnm persists dragged positions to localStorage under this key, so simply
      // reopening the page keeps the arrangement (no manual step).
      persist: "gogo-view:layout:" + key,
      // A compare-row cell is half-width, where the minimap covers a large
      // fraction of an already-cramped canvas and helps nobody - drop it there
      // and let ⛶ expand carry the "see it properly" job. Solo/full-width rows
      // keep it.
      minimap: !isCompareCell(figure),
      // Tighter than vnm's 60px default: gogo diagrams are typically wide and
      // short, so a large pad wastes most of the canvas on first paint.
      fitPadding: 24,
    };

    var entry = Object.prototype.hasOwnProperty.call(LAYOUTS, key) ? LAYOUTS[key] : null;

    // The authoring step tried this diagram and the parse failed - show that,
    // rather than a misleading "install very-nice-mermaid" hint.
    if (entry && entry._error) {
      fail(figure, "Could not parse \"" + key + "\":\n" + entry._error +
        "\n\nThis diagram's Mermaid source is invalid (mermaid.js rejects it too) - fix the .mmd.");
      return;
    }

    try {
      if (entry) {
        handles[key] = window.vnm.mount(host, rehydrate(entry), opts);
      } else if (dsl && isFlowchart(dsl)) {
        handles[key] = window.vnm.mount(host, dsl, opts);
      } else if (svg) {
        host.className = "diagram-host static";
        host.appendChild(svg);
        return; // static: no handle, no expand control needed
      } else {
        fail(figure, "No renderable diagram for \"" + key + "\".\n" +
          "This kind needs a prebuilt layout - re-run the authoring step with very-nice-mermaid installed:\n" +
          "  npm i -g very-nice-mermaid");
        return;
      }
    } catch (e) {
      fail(figure, "Could not render \"" + key + "\":\n" + (e && e.message ? e.message : String(e)));
      return;
    }

    // A committed sidecar wins on first open; local drags persist thereafter.
    if (SEEDS[key] && handles[key] && handles[key].importLayout) {
      try { handles[key].importLayout(SEEDS[key]); } catch (e) { /* seed is advisory */ }
    }
    addExpandControl(figure, host, key);
  }

  /* ---- expand-to-modal: move the host into a near-fullscreen overlay ---- */
  var scrim, panel, body, origin;

  function ensureModal() {
    if (scrim) return;
    scrim = document.createElement("div");
    scrim.className = "gogo-modal-scrim";
    scrim.hidden = true;
    panel = document.createElement("div");
    panel.className = "gogo-modal";
    var close = document.createElement("button");
    close.className = "gogo-modal-close";
    close.textContent = "✕";
    close.title = "close (Esc)";
    close.addEventListener("click", closeModal);
    body = document.createElement("div");
    body.className = "gogo-modal-body";
    panel.appendChild(close);
    panel.appendChild(body);
    scrim.appendChild(panel);
    scrim.addEventListener("click", function (e) { if (e.target === scrim) closeModal(); });
    document.addEventListener("keydown", function (e) { if (e.key === "Escape") closeModal(); });
    document.body.appendChild(scrim);
  }

  function refit(key) {
    var h = handles[key];
    if (h && h.fit) { try { h.fit(); } catch (e) { /* fit is best-effort */ } }
  }

  function closeModal() {
    if (!scrim || scrim.hidden || !origin) return;
    origin.parent.insertBefore(origin.host, origin.next);
    scrim.hidden = true;
    var key = origin.key;
    origin = null;
    setTimeout(function () { refit(key); }, 0);
  }

  function addExpandControl(figure, host, key) {
    ensureModal();
    var btn = document.createElement("button");
    btn.className = "gogo-expand";
    btn.textContent = "⛶";
    btn.title = "expand - open this diagram near-fullscreen (Esc / ✕ / click outside closes)";
    btn.addEventListener("click", function () {
      closeModal(); // only one at a time
      origin = { host: host, parent: host.parentNode, next: host.nextSibling, key: key };
      body.appendChild(host);
      scrim.hidden = false;
      setTimeout(function () { refit(key); }, 0);
    });
    figure.appendChild(btn);
    figure.style.position = figure.style.position || "relative";
  }

  function start() {
    if (!window.vnm || !window.vnm.mount) {
      var figures = document.querySelectorAll("figure.diagram");
      for (var i = 0; i < figures.length; i++) {
        fail(figures[i], "The very-nice-mermaid runtime did not load.\n" +
          "Expected vnm-browser.js beside this page - re-run /gogo:view to restore it.");
      }
      return;
    }
    var figures = document.querySelectorAll("figure.diagram");
    for (var i = 0; i < figures.length; i++) mountOne(figures[i]);
    // Exposed so a diagram's arrangement can be exported/committed as a sidecar.
    window.gogoViewer = {
      handles: handles,
      exportLayouts: function () {
        var out = {};
        for (var k in handles) {
          if (handles[k] && handles[k].exportLayout) {
            try { out[k] = handles[k].exportLayout(); } catch (e) { /* skip */ }
          }
        }
        return out;
      },
    };
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
