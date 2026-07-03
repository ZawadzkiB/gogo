/* gogo interactive diagram viewer — orchestrator. Vanilla JS, zero deps, offline
 * (file://). Loaded as a plain <script> after geometry.js / viewport.js /
 * mermaid-parse.js / render.js (no ES modules — file:// blocks import & fetch).
 *
 * Per diagram:
 *   1. mermaid (vendored) renders the .mmd to an SVG (layout only).
 *   2. mermaid-parse.js tries to read that SVG into a {nodes,edges} model.
 *   3. flowchart-family (model returned)  -> RICH render via render.js:
 *        token-styled node cards + an owned edge layer that re-routes on drag +
 *        a minimap + fit/zoom/reset-layout, positions persisted (decision D1/D3).
 *      other kinds (model === null)       -> FALLBACK to the 0.5.0 pan/zoom/drag
 *        canvas (below), unchanged — never a blank page (FR1/FR6).
 *
 * If the vendored mermaid runtime is missing, every diagram degrades to an inline
 * error and the summary still reads.
 */
(function () {
  "use strict";
  var ns = (window.gogoViewer = window.gogoViewer || {});
  var MIN = 0.1, MAX = 8;
  var clamp = function (v, lo, hi) { return Math.max(lo, Math.min(hi, v)); };

  function ready(fn) {
    if (document.readyState !== "loading") fn();
    else document.addEventListener("DOMContentLoaded", fn);
  }

  function fail(msg) {
    document.querySelectorAll("pre.mermaid").forEach(function (pre) {
      var d = document.createElement("div");
      d.className = "err";
      d.textContent = msg;
      pre.replaceWith(d);
    });
  }

  // -------------------------------------------------------------------------
  // FALLBACK renderer — the 0.5.0 pan/zoom/drag canvas, kept intact for
  // non-flowchart kinds (sequence / class / stateDiagram) and any parse failure.
  // Whole-diagram interaction only: mermaid's SVG moved around as one image.
  // -------------------------------------------------------------------------
  function fallbackViewport(figure, svg) {
    if (!svg) return;

    var vb = svg.viewBox && svg.viewBox.baseVal;
    var box = svg.getBoundingClientRect();
    var nW = (vb && vb.width) || box.width || 600;
    var nH = (vb && vb.height) || box.height || 400;
    svg.removeAttribute("style");
    svg.setAttribute("width", nW);
    svg.setAttribute("height", nH);
    svg.style.display = "block";

    var viewport = document.createElement("div");
    viewport.className = "viewport";
    var canvas = document.createElement("div");
    canvas.className = "canvas";
    canvas.appendChild(svg);           // moves svg into the canvas
    viewport.appendChild(canvas);
    figure.appendChild(viewport);

    var s = 1, tx = 0, ty = 0;
    function apply() {
      canvas.style.transform = "translate(" + tx + "px," + ty + "px) scale(" + s + ")";
    }
    function zoomBy(factor, cx, cy) {
      var r = viewport.getBoundingClientRect();
      if (cx == null) { cx = r.width / 2; cy = r.height / 2; }
      var ns2 = clamp(s * factor, MIN, MAX);
      tx = cx - ((cx - tx) / s) * ns2;  // keep the point under the cursor fixed
      ty = cy - ((cy - ty) / s) * ns2;
      s = ns2; apply();
    }
    function fit() {
      var r = viewport.getBoundingClientRect(), pad = 24;
      s = clamp(Math.min((r.width - pad) / nW, (r.height - pad) / nH) || 1, MIN, MAX);
      tx = (r.width - nW * s) / 2;
      ty = (r.height - nH * s) / 2;
      apply();
    }
    function reset() {
      var r = viewport.getBoundingClientRect();
      s = 1; tx = Math.max((r.width - nW) / 2, 0); ty = 16; apply();
    }

    var drag = null;
    viewport.addEventListener("pointerdown", function (e) {
      if (e.target.closest(".controls")) return;
      drag = { x: e.clientX, y: e.clientY, tx: tx, ty: ty };
      try { viewport.setPointerCapture(e.pointerId); } catch (_) {}
      viewport.classList.add("grabbing");
    });
    viewport.addEventListener("pointermove", function (e) {
      if (!drag) return;
      tx = drag.tx + (e.clientX - drag.x);
      ty = drag.ty + (e.clientY - drag.y);
      apply();
    });
    function endDrag() { drag = null; viewport.classList.remove("grabbing"); }
    viewport.addEventListener("pointerup", endDrag);
    viewport.addEventListener("pointercancel", endDrag);

    viewport.addEventListener("wheel", function (e) {
      e.preventDefault();
      var r = viewport.getBoundingClientRect();
      zoomBy(Math.exp(-e.deltaY * 0.0015), e.clientX - r.left, e.clientY - r.top);
    }, { passive: false });

    var ctl = document.createElement("div");
    ctl.className = "controls";
    [["−", "zoom out", function () { zoomBy(1 / 1.2); }],
     ["+", "zoom in", function () { zoomBy(1.2); }],
     ["fit", "fit to view", fit],
     ["reset", "reset to 100%", reset]].forEach(function (b) {
      var btn = document.createElement("button");
      btn.type = "button"; btn.textContent = b[0]; btn.title = b[1];
      btn.addEventListener("click", b[2]);
      ctl.appendChild(btn);
    });
    viewport.appendChild(ctl);

    fit();
    // Expose the same shape as the rich renderer so the expand-to-modal control
    // (TEST-012) can move this .viewport node and refit it to the new size.
    return { viewport: viewport, refit: fit };
  }

  // -------------------------------------------------------------------------
  // Expand-to-modal (TEST-012) — a near-fullscreen overlay so a compare-row
  // diagram (two per row, each cramped) can be read big. Clicking a figure's
  // ⛶ expand control MOVES that figure's .viewport node into the overlay and
  // refits it; closing moves the node back to its row and refits to the original
  // size. Only one modal at a time. Pure offline DOM — no network, file://-safe.
  // -------------------------------------------------------------------------
  var modal = (function () {
    var scrim = null, body = null, current = null;

    function ensure() {
      if (scrim) return;
      scrim = document.createElement("div");
      scrim.className = "gogo-modal-scrim";
      scrim.hidden = true;

      var panel = document.createElement("div");
      panel.className = "gogo-modal";

      var close = document.createElement("button");
      close.type = "button";
      close.className = "gogo-modal-close";
      close.textContent = "✕";
      close.title = "close (Esc)";
      close.addEventListener("click", closeModal);

      body = document.createElement("div");
      body.className = "gogo-modal-body";

      panel.appendChild(close);
      panel.appendChild(body);
      scrim.appendChild(panel);
      // Scrim (backdrop) click closes; a click on the panel/diagram does not.
      scrim.addEventListener("click", function (e) { if (e.target === scrim) closeModal(); });
      document.body.appendChild(scrim);
    }

    // Refit after the overlay has laid out, so getBoundingClientRect() reflects
    // the new (modal or restored) size. A double rAF is robust across engines.
    function refitSoon(handle) {
      if (!handle || typeof handle.refit !== "function") return;
      requestAnimationFrame(function () {
        requestAnimationFrame(function () { try { handle.refit(); } catch (_) {} });
      });
    }

    function openModal(figure, handle) {
      if (!handle || !handle.viewport) return;
      ensure();
      if (current) closeModal();            // only one modal at a time
      current = { figure: figure, handle: handle, viewport: handle.viewport };
      body.appendChild(handle.viewport);    // MOVE the diagram node into the overlay
      scrim.hidden = false;
      document.addEventListener("keydown", onKey);
      refitSoon(handle);
    }

    function closeModal() {
      if (!current) return;
      var c = current;
      current = null;
      document.removeEventListener("keydown", onKey);
      scrim.hidden = true;
      c.figure.appendChild(c.viewport);     // move the node back to its row
      refitSoon(c.handle);
    }

    function onKey(e) {
      if (e.key === "Escape" || e.keyCode === 27) { e.preventDefault(); closeModal(); }
    }

    return { open: openModal, close: closeModal };
  })();

  // addExpandControl appends a ⛶ expand button to a figure's control strip and
  // wires it to the modal (TEST-012). Works for both renderer kinds — the rich
  // flowchart and the pan/zoom fallback each return { viewport, refit }.
  function addExpandControl(figure, handle) {
    if (!handle || !handle.viewport) return;
    var controls = handle.viewport.querySelector(".controls");
    if (!controls) return;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "gogo-expand";
    btn.textContent = "⛶";
    btn.title = "expand — open this diagram near-fullscreen (Esc / ✕ / click outside closes)";
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      modal.open(figure, handle);
    });
    controls.appendChild(btn);
  }

  // A short, stable per-diagram key for layout persistence: the figure's
  // data-diagram (the .mmd basename, set by gogo-view) else its index.
  function diagramName(figure, i) {
    return figure.getAttribute("data-diagram") || ("diagram-" + i);
  }

  ready(function () {
    if (!window.mermaid) {
      fail("Could not load the vendored mermaid runtime. Open the .mmd sources directly, or re-run /gogo:view.");
      return;
    }

    var figures = Array.prototype.slice.call(document.querySelectorAll("figure.diagram"));
    // Capture each diagram's source BEFORE mermaid.run replaces the <pre> with an
    // <svg> — the parser uses it for flowchart-family detection.
    var sources = figures.map(function (f) {
      var pre = f.querySelector("pre.mermaid");
      return pre ? pre.textContent : "";
    });

    window.mermaid.initialize({ startOnLoad: false, theme: "default", securityLevel: "loose" });
    // suppressErrors: a single malformed diagram renders its own inline error
    // instead of rejecting the batch; the global catch is for catastrophic failure.
    Promise.resolve(window.mermaid.run({ querySelector: "pre.mermaid", suppressErrors: true }))
      .then(function () {
        figures.forEach(function (figure, i) {
          var svg = figure.querySelector("svg");
          if (!svg) return; // mermaid emitted an inline error bomb — leave it visible
          // The <pre class="mermaid"> still holds mermaid's svg after mermaid.run.
          // Capture it now; we drop it once the svg has been taken over (below).
          var pre = figure.querySelector("pre.mermaid");

          var model = null;
          try {
            model = ns.parseMermaid(svg, sources[i]);
          } catch (e) {
            model = null; // any parse hiccup -> fall back, never a blank page
          }

          var handle = null;
          if (model && model.nodes && model.nodes.length) {
            var name = diagramName(figure, i);
            var layout = (window.GOGO_LAYOUT && window.GOGO_LAYOUT[name]) || null;
            try {
              handle = ns.render(figure, svg, model, { name: name, layout: layout });
            } catch (e) {
              handle = fallbackViewport(figure, svg); // rich render failed -> safe fallback
            }
          } else {
            handle = fallbackViewport(figure, svg);
          }

          // Every figure — rich or fallback — gets an expand-to-modal control
          // (TEST-012), wired to refit the same renderer at the new size.
          addExpandControl(figure, handle);

          // REV-001: remove the now-emptied <pre> (rich hides the svg in place;
          // fallback moved it into the canvas). Matches 0.5.0's pre.replaceWith so
          // no default <pre> margin leaves a stray gap above each diagram.
          if (pre && pre.parentNode) pre.parentNode.removeChild(pre);
        });
      })
      .catch(function (err) { fail("mermaid render failed: " + (err && err.message ? err.message : err)); });
  });
})();
