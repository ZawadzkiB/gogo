/* gogo interactive diagram viewer — vanilla JS, zero deps, offline (file://).
 *
 * Renders every <pre class="mermaid"> with the vendored mermaid runtime, then
 * wraps each resulting SVG in a pan / zoom / drag canvas viewport with its own
 * floating reset / fit / zoom controls. Whole-diagram interaction only:
 * per-node repositioning is intentionally out of scope (decision D3).
 *
 * State per diagram is a CSS transform on a wrapper "canvas": translate(tx,ty)
 * + scale(s). Drag pans, wheel zooms toward the cursor, buttons reset/fit/zoom.
 */
(function () {
  "use strict";
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

  // Wrap one rendered diagram's <svg> in a pan/zoom/drag viewport.
  function setupViewport(figure) {
    var svg = figure.querySelector("svg");
    var pre = figure.querySelector("pre.mermaid");
    if (!svg) return;

    // Natural size from the viewBox (mermaid always sets one); strip the
    // responsive max-width style so the canvas transform fully controls scale.
    var vb = svg.viewBox && svg.viewBox.baseVal;
    var box = svg.getBoundingClientRect();
    var nW = (vb && vb.width) || box.width || 600;
    var nH = (vb && vb.height) || box.height || 400;
    svg.removeAttribute("style");
    svg.setAttribute("width", nW);
    svg.setAttribute("height", nH);

    var viewport = document.createElement("div");
    viewport.className = "viewport";
    var canvas = document.createElement("div");
    canvas.className = "canvas";
    canvas.appendChild(svg);           // moves svg out of <pre> into the canvas
    viewport.appendChild(canvas);
    if (pre) pre.replaceWith(viewport); else figure.appendChild(viewport);

    var s = 1, tx = 0, ty = 0;
    function apply() {
      canvas.style.transform = "translate(" + tx + "px," + ty + "px) scale(" + s + ")";
    }
    function zoomBy(factor, cx, cy) {
      var r = viewport.getBoundingClientRect();
      if (cx == null) { cx = r.width / 2; cy = r.height / 2; }
      var ns = clamp(s * factor, MIN, MAX);
      tx = cx - ((cx - tx) / s) * ns;  // keep the point under the cursor fixed
      ty = cy - ((cy - ty) / s) * ns;
      s = ns; apply();
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

    // drag-to-pan (pointer events cover mouse + touch)
    var drag = null;
    viewport.addEventListener("pointerdown", function (e) {
      if (e.target.closest(".controls")) return;  // let control buttons click cleanly
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

    // wheel-to-zoom toward the cursor
    viewport.addEventListener("wheel", function (e) {
      e.preventDefault();
      var r = viewport.getBoundingClientRect();
      zoomBy(Math.exp(-e.deltaY * 0.0015), e.clientX - r.left, e.clientY - r.top);
    }, { passive: false });

    // floating per-diagram controls
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

    fit();  // open fitted to the viewport
  }

  ready(function () {
    if (!window.mermaid) {
      fail("Could not load the vendored mermaid runtime. Open the .mmd sources directly, or re-run /gogo:view.");
      return;
    }
    window.mermaid.initialize({ startOnLoad: false, theme: "default", securityLevel: "loose" });
    // suppressErrors: a single malformed diagram renders its own inline error
    // (mermaid's bomb svg) instead of rejecting the batch and blanking the good
    // ones; the global catch is reserved for a catastrophic runtime failure.
    Promise.resolve(window.mermaid.run({ querySelector: "pre.mermaid", suppressErrors: true }))
      .then(function () { document.querySelectorAll(".diagram").forEach(setupViewport); })
      .catch(function (err) { fail("mermaid render failed: " + (err && err.message ? err.message : err)); });
  });
})();
