/* gogo viewer — the rich renderer for a parsed flowchart model.
 *
 * Given a { nodes, edges } model (from mermaid-parse.js) it renders, inside the
 * diagram's figure:
 *   - token-styled node CARDS (HTML divs, absolutely positioned in a transformed
 *     "world"), colored by their classDef role via CSS vars — visibly not
 *     mermaid's default look;
 *   - an OWNED SVG edge layer, redrawn from geometry.routeEdge every frame so
 *     dragging a node re-routes its edges live (mermaid's baked SVG is hidden);
 *   - a MINIMAP (inverse-transform, click / drag to recenter);
 *   - per-diagram controls: zoom -, zoom +, fit, reset-layout.
 *
 * Interaction is delegated to the ported viewport controller (viewport.js).
 * Layout persistence (decision D7=A, within the file:// no-write-back constraint):
 *   - dragged positions auto-save (debounced) to localStorage under
 *     "gogo-view:layout:<name>" — a re-opened page keeps the arrangement with zero
 *     manual steps (localStorage works over file://);
 *   - an "export" control downloads the portable <name>.layout.json (the D6 shape,
 *     the full window.gogoViewer.layouts map) via a Blob download — no network;
 *   - seed order on load: injected window.GOGO_LAYOUT[name] (the committed sidecar
 *     the gogo-view skill reads) -> localStorage -> mermaid's parsed positions.
 * Also mirrors the layout to window.gogoViewer.layouts[name] and notifies the
 * optional onPersist(name, positions) hook. No label editing (decision D2=A).
 *
 * Attaches to window.gogoViewer (plain <script>, no import/export — file://).
 */
(function () {
  "use strict";
  var ns = (window.gogoViewer = window.gogoViewer || {});
  var SVGNS = "http://www.w3.org/2000/svg";
  var MM_W = 168, MM_H = 104, MM_PAD = 6;
  var PERSIST_MS = 400;
  var LS_PREFIX = "gogo-view:layout:";  // localStorage key namespace (D7=A)

  function svgEl(name, attrs) {
    var el = document.createElementNS(SVGNS, name);
    if (attrs) for (var k in attrs) if (attrs.hasOwnProperty(k)) el.setAttribute(k, attrs[k]);
    return el;
  }

  // --- localStorage persistence (D7=A). localStorage works over file://, so a
  // re-opened page keeps dragged positions with zero manual steps. Every access is
  // wrapped: a browser that blocks storage on file:// (or private mode / quota)
  // degrades to in-memory only, never throwing. Keyed by the diagram name. ---
  function lsKey(name) { return LS_PREFIX + (name || "diagram"); }
  function readLocalLayout(name) {
    try {
      var raw = window.localStorage.getItem(lsKey(name));
      var obj = raw ? JSON.parse(raw) : null;
      return obj && typeof obj === "object" ? obj : null;
    } catch (e) { return null; }
  }
  function writeLocalLayout(name, positions) {
    try { window.localStorage.setItem(lsKey(name), JSON.stringify(positions)); } catch (e) { /* in-memory only */ }
  }
  function clearLocalLayout(name) {
    try { window.localStorage.removeItem(lsKey(name)); } catch (e) { /* no-op */ }
  }

  function render(figure, mermaidSvg, model, opts) {
    opts = opts || {};
    var geometry = ns.geometry;
    var vp = ns.createViewport();

    // Original (mermaid) positions — the reset-layout baseline.
    var orig = {};
    var seed = {};
    // Seed order (D7=A): committed sidecar (opts.layout, from window.GOGO_LAYOUT)
    // wins first-open -> then localStorage (this browser's saved drags) -> then
    // mermaid's parsed positions.
    var localLayout = readLocalLayout(opts.name);
    model.nodes.forEach(function (n) {
      orig[n.id] = { x: n.x, y: n.y };
      var saved = (opts.layout && opts.layout[n.id]) || (localLayout && localLayout[n.id]);
      seed[n.id] = saved ? { x: saved.x, y: saved.y } : { x: n.x, y: n.y };
    });
    vp.setPositions(seed);

    // Hide mermaid's baked SVG — we own the render now (it stays in the DOM,
    // already parsed, so nothing re-lays-out).
    if (mermaidSvg) mermaidSvg.style.display = "none";

    // --- DOM scaffold ---
    var viewport = document.createElement("div");
    viewport.className = "viewport gogo-rich";

    var world = document.createElement("div");
    world.className = "gogo-world";

    var edgeSvg = svgEl("svg", { "class": "gogo-edges", overflow: "visible" });
    edgeSvg.style.position = "absolute";
    edgeSvg.style.left = "0";
    edgeSvg.style.top = "0";
    edgeSvg.style.overflow = "visible";
    var defs = svgEl("defs");
    var marker = svgEl("marker", {
      id: "gogo-arrow-" + (opts.name || "d"), viewBox: "0 0 10 10", refX: "9", refY: "5",
      markerWidth: "7", markerHeight: "7", orient: "auto-start-reverse"
    });
    marker.appendChild(svgEl("path", { d: "M 0 0 L 10 5 L 0 10 z", "class": "gogo-arrow-head" }));
    defs.appendChild(marker);
    edgeSvg.appendChild(defs);
    world.appendChild(edgeSvg);

    // Node cards.
    var cardEls = {};
    model.nodes.forEach(function (n) {
      var card = document.createElement("div");
      card.className = "gogo-node";
      (n.classes || []).forEach(function (c) { card.classList.add("role-" + c); });
      card.setAttribute("data-id", n.id);
      card.style.minWidth = n.w + "px";
      card.textContent = n.label || n.id; // textContent — no label editing (D2=A)
      card.addEventListener("pointerdown", function (e) {
        if (e.button !== 0) return;
        e.stopPropagation(); // don't start a background pan
        vp.startNodeDrag(n.id, e.clientX, e.clientY);
      });
      world.appendChild(card);
      cardEls[n.id] = card;
    });

    // Edge paths + optional labels (owned layer).
    var pathEls = [];
    var edgeLabelEls = [];
    model.edges.forEach(function (e) {
      var path = svgEl("path", { "class": "gogo-edge", "marker-end": "url(#gogo-arrow-" + (opts.name || "d") + ")" });
      edgeSvg.appendChild(path);
      pathEls.push(path);
      if (e.label) {
        var t = svgEl("text", { "class": "gogo-edge-label", "text-anchor": "middle", dy: "-2" });
        t.textContent = e.label;
        edgeSvg.appendChild(t);
        edgeLabelEls.push(t);
      } else {
        edgeLabelEls.push(null);
      }
    });

    viewport.appendChild(world);

    // --- controls ---
    var ctl = document.createElement("div");
    ctl.className = "controls";
    [
      ["−", "zoom out", function () { vp.zoomAt(1 / 1.2, sizeOf().w / 2, sizeOf().h / 2); draw(); }],
      ["+", "zoom in", function () { vp.zoomAt(1.2, sizeOf().w / 2, sizeOf().h / 2); draw(); }],
      ["fit", "fit to view", function () { fitAll(); draw(); }],
      ["reset", "reset layout to mermaid's positions", resetLayout],
      ["export", "download layout.json — save as <name>.layout.json to persist / share", exportLayout]
    ].forEach(function (b) {
      var btn = document.createElement("button");
      btn.type = "button";
      btn.textContent = b[0];
      btn.title = b[1];
      btn.addEventListener("click", b[2]);
      ctl.appendChild(btn);
    });
    viewport.appendChild(ctl);

    // --- minimap ---
    var minimap = document.createElement("div");
    minimap.className = "gogo-minimap";
    var mmNodes = {};
    model.nodes.forEach(function (n) {
      var d = document.createElement("div");
      d.className = "gogo-mm-node";
      (n.classes || []).forEach(function (c) { d.classList.add("role-" + c); });
      minimap.appendChild(d);
      mmNodes[n.id] = d;
    });
    var mmView = document.createElement("div");
    mmView.className = "gogo-mm-view";
    minimap.appendChild(mmView);
    viewport.appendChild(minimap);

    // Mount.
    figure.appendChild(viewport);

    // --- geometry helpers over live boxes ---
    function boxOf(id) {
      var p = vp.state.pos[id];
      var el = cardEls[id];
      if (!p || !el) return null;
      // offsetWidth/Height are layout px (transform-independent) = world size.
      var n = null;
      for (var i = 0; i < model.nodes.length; i++) if (model.nodes[i].id === id) { n = model.nodes[i]; break; }
      return { x: p.x, y: p.y, w: el.offsetWidth || (n && n.w) || 40, h: el.offsetHeight || (n && n.h) || 24 };
    }
    function allBoxes() {
      var out = [];
      model.nodes.forEach(function (n) { var b = boxOf(n.id); if (b) out.push(b); });
      return out;
    }
    function sizeOf() {
      var r = viewport.getBoundingClientRect();
      return { w: r.width, h: r.height };
    }
    function fitAll() {
      vp.fit(geometry.contentBounds(allBoxes()), sizeOf());
    }

    // --- draw (transform world, place cards, re-route edges, refresh minimap) ---
    function draw() {
      world.style.transform = vp.worldTransform();
      model.nodes.forEach(function (n) {
        var p = vp.state.pos[n.id];
        var el = cardEls[n.id];
        el.style.left = p.x + "px";
        el.style.top = p.y + "px";
      });
      model.edges.forEach(function (e, i) {
        var a = boxOf(e.fromId), b = boxOf(e.toId);
        var path = pathEls[i];
        if (!a || !b) { path.setAttribute("d", ""); return; }
        var r = geometry.routeEdge(a, b);
        path.setAttribute("d", r.d);
        var lbl = edgeLabelEls[i];
        if (lbl) { lbl.setAttribute("x", r.mid.x); lbl.setAttribute("y", r.mid.y); }
      });
      drawMinimap();
    }

    function drawMinimap() {
      var b = geometry.contentBounds(allBoxes());
      var scale = Math.min((MM_W - MM_PAD * 2) / b.w, (MM_H - MM_PAD * 2) / b.h);
      var offX = MM_PAD + ((MM_W - MM_PAD * 2) - b.w * scale) / 2;
      var offY = MM_PAD + ((MM_H - MM_PAD * 2) - b.h * scale) / 2;
      model.nodes.forEach(function (n) {
        var box = boxOf(n.id);
        var d = mmNodes[n.id];
        if (!box) { d.style.display = "none"; return; }
        d.style.display = "block";
        d.style.left = (offX + (box.x - b.x) * scale) + "px";
        d.style.top = (offY + (box.y - b.y) * scale) + "px";
        d.style.width = Math.max(2, box.w * scale) + "px";
        d.style.height = Math.max(2, box.h * scale) + "px";
      });
      var sz = sizeOf();
      var v = vp.state.view;
      if (sz.w && v.zoom) {
        var viewWorldX = -v.panX / v.zoom;
        var viewWorldY = -v.panY / v.zoom;
        mmView.style.left = (offX + (viewWorldX - b.x) * scale) + "px";
        mmView.style.top = (offY + (viewWorldY - b.y) * scale) + "px";
        mmView.style.width = (sz.w / v.zoom) * scale + "px";
        mmView.style.height = (sz.h / v.zoom) * scale + "px";
      }
    }

    // Recenter so a clicked minimap point becomes the viewport center.
    function minimapPanTo(clientX, clientY) {
      var rect = minimap.getBoundingClientRect();
      var sz = sizeOf();
      if (!sz.w) return;
      var b = geometry.contentBounds(allBoxes());
      var scale = Math.min((MM_W - MM_PAD * 2) / b.w, (MM_H - MM_PAD * 2) / b.h);
      var offX = MM_PAD + ((MM_W - MM_PAD * 2) - b.w * scale) / 2;
      var offY = MM_PAD + ((MM_H - MM_PAD * 2) - b.h * scale) / 2;
      var worldX = b.x + (clientX - rect.left - offX) / scale;
      var worldY = b.y + (clientY - rect.top - offY) / scale;
      var v = vp.state.view;
      vp.setPan(sz.w / 2 - worldX * v.zoom, sz.h / 2 - worldY * v.zoom);
      draw();
    }

    // --- persistence (D7=A: localStorage auto-save + portable sidecar export) ---
    var nameKey = opts.name || "diagram";
    // Current rounded positions ({id:{x,y}}) — the persisted / exported shape.
    function snapshot() {
      var out = {};
      model.nodes.forEach(function (n) {
        var p = vp.state.pos[n.id];
        out[n.id] = { x: Math.round(p.x), y: Math.round(p.y) };
      });
      return out;
    }
    // Mirror the layout into the shared namespace so "export" always sees every
    // rendered diagram (even ones never dragged); notify the optional onPersist
    // hook on real persistence events (drag / reset), not on the initial seed.
    function mirror(positions, notify) {
      ns.layouts = ns.layouts || {};
      ns.layouts[nameKey] = positions;
      if (notify && typeof opts.onPersist === "function") opts.onPersist(nameKey, positions);
    }
    mirror(snapshot(), false);

    var persistTimer = null;
    function persist() {
      if (persistTimer) clearTimeout(persistTimer);
      persistTimer = setTimeout(function () {
        var out = snapshot();
        writeLocalLayout(nameKey, out);  // auto-persist (seamless reopen, file://-safe)
        mirror(out, true);
      }, PERSIST_MS);
    }

    // Download the portable sidecar (D6 shape) so the arrangement can be committed
    // / shared. A Blob object URL triggers a download entirely offline (no network,
    // no fetch). Ships the full window.gogoViewer.layouts map — the exact object
    // gogo-view re-seeds as window.GOGO_LAYOUT — ready to save as
    // .gogo/resources/view/<name>.layout.json.
    function exportLayout() {
      mirror(snapshot(), false);
      ns.layouts = ns.layouts || {};
      var data = JSON.stringify(ns.layouts, null, 2);
      try {
        var blob = new Blob([data], { type: "application/json" });
        var url = URL.createObjectURL(blob);
        var a = document.createElement("a");
        a.href = url;
        a.download = "layout.json";
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        setTimeout(function () { URL.revokeObjectURL(url); }, 0);
      } catch (e) {
        // No Blob/URL download available — log so the JSON is still recoverable.
        if (window.console && console.log) console.log("gogo layout — save as <name>.layout.json:\n" + data);
      }
    }

    function resetLayout() {
      model.nodes.forEach(function (n) { vp.state.pos[n.id] = { x: orig[n.id].x, y: orig[n.id].y }; });
      fitAll();
      draw();
      clearLocalLayout(nameKey);  // reset also drops this diagram's saved localStorage entry (D7=A)
      mirror(snapshot(), true);
    }

    // --- wiring: background pan, wheel zoom, window-level move/up, minimap ---
    viewport.addEventListener("pointerdown", function (e) {
      if (e.button !== 0) return;
      if (e.target.closest(".controls") || e.target.closest(".gogo-minimap") || e.target.closest(".gogo-node")) return;
      vp.startPan(e.clientX, e.clientY);
      viewport.classList.add("grabbing");
    });
    window.addEventListener("pointermove", function (e) {
      if (!vp.isDragging()) return;
      vp.onMove(e.clientX, e.clientY);
      draw();
    });
    function endPointer() {
      var d = vp.endDrag();
      viewport.classList.remove("grabbing");
      if (d && d.kind === "node" && d.moved) persist();
    }
    window.addEventListener("pointerup", endPointer);
    // REV-003: a cancelled touch/pen pointer (scroll/gesture takeover) must also
    // end the drag — otherwise the next bare pointermove keeps moving the node.
    window.addEventListener("pointercancel", endPointer);
    viewport.addEventListener("wheel", function (e) {
      e.preventDefault();
      var r = viewport.getBoundingClientRect();
      vp.zoomAt(Math.exp(-e.deltaY * 0.0015), e.clientX - r.left, e.clientY - r.top);
      draw();
    }, { passive: false });

    var mmDragging = false;
    minimap.addEventListener("pointerdown", function (e) {
      e.stopPropagation();
      mmDragging = true;
      minimapPanTo(e.clientX, e.clientY);
    });
    minimap.addEventListener("pointermove", function (e) {
      if (mmDragging) minimapPanTo(e.clientX, e.clientY);
    });
    var mmStop = function () { mmDragging = false; };
    minimap.addEventListener("pointerup", mmStop);
    minimap.addEventListener("pointerleave", mmStop);

    // Redraw on container resize so fit/minimap stay correct.
    if (window.ResizeObserver) {
      try { new ResizeObserver(function () { draw(); }).observe(viewport); } catch (e) { /* no-op */ }
    } else {
      window.addEventListener("resize", draw);
    }

    // Initial fit once the cards have laid out (so offsetWidth/Height are real).
    requestAnimationFrame(function () {
      fitAll();
      draw();
    });

    return { viewport: viewport, redraw: draw, resetLayout: resetLayout };
  }

  ns.render = render;
})();
