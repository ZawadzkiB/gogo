/* gogo viewer — the interactive viewport controller (no DOM, no framework).
 *
 * Ported from xplan's web/src/components/useCanvasViewport.ts, minus React:
 * a plain controller object that owns the model state
 *   { pos: {id:{x,y}}, view: {panX,panY,zoom} }
 * and mutates it in response to caller-supplied pointer coordinates. The caller
 * (render.js) wires DOM events to these methods and redraws after each call.
 *
 * Math preserved verbatim from xplan:
 *   - node drag: delta / zoom, with CLICK_SLOP separating a click from a drag
 *   - zoom toward cursor: pan = c - (c - pan) * k  (k = newZoom / oldZoom)
 *   - fit: zoom clamped to <= 1, then center the bounds in the viewport
 *
 * Attaches to window.gogoViewer (plain <script>, no import/export — file://).
 */
(function () {
  "use strict";
  var ns = (window.gogoViewer = window.gogoViewer || {});
  var geometry = ns.geometry;

  var ZOOM_MIN = 0.25;
  var ZOOM_MAX = 2.5;
  var CLICK_SLOP = 4;

  function createViewport() {
    var state = { pos: {}, view: { panX: 0, panY: 0, zoom: 1 } };
    var drag = null;

    return {
      state: state,
      constants: { ZOOM_MIN: ZOOM_MIN, ZOOM_MAX: ZOOM_MAX, CLICK_SLOP: CLICK_SLOP },

      // Seed / replace all node positions ({id:{x,y}}).
      setPositions: function (pos) {
        state.pos = pos || {};
      },
      positionOf: function (id) {
        return state.pos[id] || null;
      },

      // --- drag lifecycle (coords are clientX/clientY) ---
      startNodeDrag: function (id, clientX, clientY) {
        var p = state.pos[id] || { x: 0, y: 0 };
        drag = { kind: "node", id: id, startX: clientX, startY: clientY, originX: p.x, originY: p.y, moved: false };
      },
      startPan: function (clientX, clientY) {
        drag = { kind: "pan", startX: clientX, startY: clientY, originPanX: state.view.panX, originPanY: state.view.panY };
      },
      isDragging: function () {
        return !!drag;
      },
      // Apply a pointer move to the active drag. Returns a small descriptor of
      // what changed (or null if nothing is being dragged) so the caller can
      // decide what to redraw.
      onMove: function (clientX, clientY) {
        if (!drag) return null;
        if (drag.kind === "pan") {
          state.view.panX = drag.originPanX + (clientX - drag.startX);
          state.view.panY = drag.originPanY + (clientY - drag.startY);
          return { kind: "pan" };
        }
        var dx = (clientX - drag.startX) / state.view.zoom;
        var dy = (clientY - drag.startY) / state.view.zoom;
        if (!drag.moved && Math.hypot(clientX - drag.startX, clientY - drag.startY) > CLICK_SLOP) {
          drag.moved = true;
        }
        state.pos[drag.id] = { x: drag.originX + dx, y: drag.originY + dy };
        return { kind: "node", id: drag.id, moved: drag.moved };
      },
      // End the active drag; returns the finished drag (so the caller can persist
      // when a node actually moved) or null.
      endDrag: function () {
        var d = drag;
        drag = null;
        return d;
      },

      // Zoom by `factor` keeping the point (cx,cy) — in viewport pixels — fixed.
      zoomAt: function (factor, cx, cy) {
        var v = state.view;
        var zoom = geometry.clamp(v.zoom * factor, ZOOM_MIN, ZOOM_MAX);
        var k = zoom / v.zoom;
        v.panX = cx - (cx - v.panX) * k;
        v.panY = cy - (cy - v.panY) * k;
        v.zoom = zoom;
      },
      setPan: function (panX, panY) {
        state.view.panX = panX;
        state.view.panY = panY;
      },
      // Fit `bounds` ({x,y,w,h}) into `size` ({w,h}); zoom clamped to <= 1 so we
      // never magnify a small diagram, then center it.
      fit: function (bounds, size) {
        if (!size || !size.w || !size.h) return;
        var zoom = geometry.clamp(Math.min(size.w / bounds.w, size.h / bounds.h), ZOOM_MIN, 1);
        state.view.panX = (size.w - bounds.w * zoom) / 2 - bounds.x * zoom;
        state.view.panY = (size.h - bounds.h * zoom) / 2 - bounds.y * zoom;
        state.view.zoom = zoom;
      },

      worldTransform: function () {
        var v = state.view;
        return "translate(" + v.panX + "px, " + v.panY + "px) scale(" + v.zoom + ")";
      }
    };
  }

  ns.createViewport = createViewport;
})();
