/* Order wizard: step navigation, client-side validation, unit conversion,
   live price estimate and the review summary. The server validates again. */
(function () {
  "use strict";

  var form = document.getElementById("order-form");
  if (!form) return;

  var catalog = { designs: [], fabrics: [] };
  var catalogEl = document.getElementById("order-catalog");
  if (catalogEl) {
    try { catalog = JSON.parse(catalogEl.textContent); } catch (e) { catalog = { designs: [], fabrics: [] }; }
  }
  var designs = {};
  var fabrics = {};
  catalog.designs.forEach(function (d) { designs[d.slug] = d; });
  catalog.fabrics.forEach(function (f) { fabrics[f.slug] = f; });

  var steps = Array.prototype.slice.call(form.querySelectorAll(".wizard-step"));
  var progressItems = Array.prototype.slice.call(document.querySelectorAll("#wizard-progress li"));
  var groups = Array.prototype.slice.call(form.querySelectorAll(".measure-group"));
  var backBtn = document.getElementById("wizard-back");
  var nextBtn = document.getElementById("wizard-next");
  var submitBtn = document.getElementById("wizard-submit");
  var estimateLine = document.getElementById("estimate-line");
  var customFields = document.getElementById("custom-fields");
  var addressField = document.getElementById("address-field");
  var current = 1;

  // ---------------------------------------------------------------- helpers
  function checked(name) {
    return form.querySelector('input[name="' + name + '"]:checked');
  }
  function unit() {
    var u = checked("unit");
    return u ? u.value : "cm";
  }
  function designValue() {
    var d = checked("design");
    return d ? d.value : "";
  }
  function fabricValue() {
    var f = checked("fabric");
    return f ? f.value : "";
  }
  function category() {
    if (designValue() === "custom") {
      var sel = document.getElementById("custom-category");
      return sel ? sel.value : "";
    }
    var d = designs[designValue()];
    return d ? d.category : "";
  }
  function money(n) {
    try {
      return "KES " + new Intl.NumberFormat("en-KE").format(Math.round(n));
    } catch (e) {
      return "KES " + Math.round(n);
    }
  }
  function setError(field, message) {
    var el = form.querySelector('[data-error-for="' + field + '"]');
    if (el) el.textContent = message || "";
  }

  function show(step, focus) {
    current = Math.max(1, Math.min(steps.length, step));
    steps.forEach(function (s) {
      s.hidden = Number(s.getAttribute("data-step")) !== current;
    });
    progressItems.forEach(function (li) {
      var n = Number(li.getAttribute("data-step"));
      li.classList.toggle("is-active", n === current);
      li.classList.toggle("is-done", n < current);
      if (n === current) { li.setAttribute("aria-current", "step"); } else { li.removeAttribute("aria-current"); }
    });
    backBtn.hidden = current === 1;
    nextBtn.hidden = current === steps.length;
    submitBtn.hidden = current !== steps.length;
    if (focus) {
      var top = document.getElementById("order-form").getBoundingClientRect().top + window.scrollY - 110;
      window.scrollTo({ top: top, behavior: "smooth" });
    }
  }

  // -------------------------------------------------------- design handling
  function syncDesign() {
    var isCustom = designValue() === "custom";
    customFields.hidden = !isCustom;
    var cat = category();
    groups.forEach(function (g) {
      g.hidden = g.getAttribute("data-category") !== cat;
    });
    var yardage = document.getElementById("yardage");
    var d = designs[designValue()];
    if (yardage && !yardage.dataset.touched) {
      yardage.value = isCustom ? "4" : (d ? String(d.yards) : "4");
    }
    updateEstimate();
  }

  form.addEventListener("change", function (e) {
    var name = e.target.name;
    if (name === "design") syncDesign();
    if (name === "fabric") updateEstimate();
    if (name === "category") syncDesign();
    if (name === "delivery") syncDelivery();
    if (name === "unit") convertUnits(e.target.value);
  });

  function syncDelivery() {
    var delivery = checked("delivery");
    var isDelivery = delivery && delivery.value === "delivery";
    addressField.hidden = !isDelivery;
  }

  // --------------------------------------------------------- unit handling
  function convertUnits(to) {
    var from = to === "cm" ? "in" : "cm";
    form.querySelectorAll(".measure-field input").forEach(function (input) {
      if (input.dataset.age === "1") return;
      var v = parseFloat(input.value);
      if (!isFinite(v) || v === 0) return;
      var cm = from === "in" ? v * 2.54 : v;
      var out = to === "in" ? cm / 2.54 : cm;
      input.value = String(Math.round(out * 10) / 10);
    });
    form.querySelectorAll("[data-unit-label]").forEach(function (el) {
      if (el.closest(".measure-field").querySelector("[data-age]")) return;
      el.textContent = to === "cm" ? "cm" : "in";
    });
    updateEstimate();
  }

  // -------------------------------------------------------------- estimate
  function currentEstimate() {
    var d = designs[designValue()];
    var yards = parseFloat(document.getElementById("yardage").value) || 0;
    var f = fabrics[fabricValue()];
    var base = d ? d.price : 0;
    var fabricCost = (f && fabricValue() !== "own") ? f.price * yards : 0;
    return {
      base: base,
      fabricCost: fabricCost,
      total: base + fabricCost,
      days: d ? d.days : (designValue() === "custom" ? 14 : 0),
      unit: f ? f.unit : "yard",
      yards: yards,
      custom: designValue() === "custom"
    };
  }

  function updateEstimate() {
    if (!estimateLine) return;
    var est = currentEstimate();
    if (!est.custom && !est.base) { estimateLine.textContent = ""; return; }
    if (est.custom) {
      estimateLine.textContent = "Custom designs are quoted after we see your idea — usually within one working day.";
      return;
    }
    var parts = ["Design from " + money(est.base)];
    if (est.fabricCost > 0) {
      parts.push("fabric " + money(est.fabricCost) + " (" + est.yards + " " + est.unit + ")");
    }
    parts.push("estimated total " + money(est.total));
    estimateLine.textContent = parts.join(" · ");
  }

  var yardageInput = document.getElementById("yardage");
  if (yardageInput) {
    yardageInput.addEventListener("input", function () {
      yardageInput.dataset.touched = "1";
      updateEstimate();
    });
  }

  // ------------------------------------------------------------ validation
  function digits(s) { return (s || "").replace(/\D/g, ""); }

  function validate(step) {
    var ok = true;
    var firstInvalid = null;

    function fail(el, field, msg) {
      ok = false;
      setError(field, msg);
      if (!firstInvalid) firstInvalid = el;
    }

    if (step === 1) {
      if (!designValue()) { ok = false; setError("design", "Choose a design, or pick “My own idea”."); }
      if (designValue() === "custom") {
        var cat = document.getElementById("custom-category");
        if (!cat.value) fail(cat, "category", "Tell us who the outfit is for.");
        var desc = document.getElementById("custom-description");
        var file = document.getElementById("inspiration");
        if (desc.value.trim().length < 10 && !(file.files && file.files.length)) {
          fail(desc, "custom_description", "Describe your idea, or attach an inspiration photo.");
        }
      }
    }

    if (step === 2) {
      if (!fabricValue()) { ok = false; setError("fabric", "Choose a fabric, or bring your own."); }
      var y = parseFloat(yardageInput.value);
      if (!isFinite(y) || y < 0.5 || y > 40) {
        fail(yardageInput, "yardage", "Enter a quantity between 0.5 and 40.");
      }
    }

    if (step === 3) {
      var activeGroup = groups.filter(function (g) { return !g.hidden; })[0];
      if (!activeGroup) {
        ok = false;
        setError("design", "Choose a design first so we know which measurements to ask for.");
      } else {
        var u = unit();
        activeGroup.querySelectorAll(".measure-field input").forEach(function (input) {
          var key = input.name;
          var raw = input.value.trim();
          setError(key, "");
          if (!raw) {
            fail(input, key, "This one is needed for a good fit.");
            return;
          }
          var n = parseFloat(raw);
          var min = parseFloat(input.dataset.min);
          var max = parseFloat(input.dataset.max);
          if (!isFinite(n) || n <= 0) {
            fail(input, key, "Use numbers only, e.g. 92.");
            return;
          }
          if (input.dataset.age === "1") {
            if (n < min || n > max) fail(input, key, "Enter an age between " + min + " and " + max + " years.");
            return;
          }
          var cm = u === "in" ? n * 2.54 : n;
          if (cm < min || cm > max) {
            fail(input, key, u === "in"
              ? "That looks unusual — expected roughly " + (min / 2.54).toFixed(0) + "–" + (max / 2.54).toFixed(0) + " inches."
              : "That looks unusual — expected " + min + "–" + max + " cm.");
          }
        });
      }
    }

    if (step === 4) {
      var name = document.getElementById("customer-name");
      if (name.value.trim().length < 2) fail(name, "name", "Tell us your name.");
      var phone = document.getElementById("customer-phone");
      if (digits(phone.value).length < 9) fail(phone, "phone", "Enter a reachable phone number, e.g. 0712 345 678.");
      var email = document.getElementById("customer-email");
      if (email.value.trim() && !/^[^@\s]+@[^@\s]+\.[^@\s]{2,}$/.test(email.value.trim())) {
        fail(email, "email", "That email looks incomplete.");
      }
      var date = document.getElementById("customer-date");
      if (!date.value) {
        fail(date, "preferred_date", "Pick the date you would like it ready.");
      } else if (date.min && date.value < date.min) {
        fail(date, "preferred_date", "We need at least three days. Choose a later date.");
      }
      if (!checked("delivery")) {
        ok = false;
        setError("delivery", "Choose pickup or delivery.");
      }
      if (checked("delivery") && checked("delivery").value === "delivery") {
        var addr = document.getElementById("customer-address");
        if (addr.value.trim().length < 8) fail(addr, "address", "Add the delivery address, including the estate or town.");
      }
    }

    if (step === 5) {
      if (!form.querySelector('input[name="terms"]').checked) {
        ok = false;
        setError("terms", "Please accept the terms so we can start sewing.");
      }
    }

    if (!ok && firstInvalid && firstInvalid.focus) firstInvalid.focus();
    return ok;
  }

  // ----------------------------------------------------------- review step
  function fillReview() {
    var est = currentEstimate();
    var d = designs[designValue()];
    var f = fabrics[fabricValue()];
    var cat = category();
    var count = 0;
    var group = groups.filter(function (g) { return g.getAttribute("data-category") === cat; })[0];
    if (group) {
      group.querySelectorAll(".measure-field input").forEach(function (input) {
        if (input.value.trim()) count++;
      });
    }
    var set = function (key, value) {
      var el = form.querySelector('[data-review="' + key + '"]');
      if (el) el.textContent = value;
    };
    set("design", designValue() === "custom"
      ? "Custom design — " + (document.getElementById("custom-description").value.trim() || "your own idea")
      : (d ? d.name : "—"));
    set("fabric", fabricValue() === "own" ? "Your own fabric" : (f ? f.name : "—"));
    set("quantity", f ? est.yards + " " + f.unit + "s (estimated)" : est.yards + " (estimated)");
    set("measurements", count + (count === 1 ? " measurement" : " measurements") + " in " + unit());
    set("date", document.getElementById("customer-date").value || "—");
    var delivery = checked("delivery");
    var address = document.getElementById("customer-address").value.trim();
    set("delivery", (delivery && delivery.value === "delivery" ? "Delivery" : "Pickup") +
      (delivery && delivery.value === "delivery" && address ? " — " + address : ""));
    set("price", est.custom ? "Quoted after review" : money(est.total) + " (estimate)");
    set("days", est.days ? est.days + " days" : "—");
  }

  nextBtn.addEventListener("click", function () {
    if (!validate(current)) return;
    if (current === 4) fillReview();
    show(current + 1, true);
  });
  backBtn.addEventListener("click", function () { show(current - 1, true); });

  Array.prototype.forEach.call(form.querySelectorAll(".js-edit-step"), function (btn) {
    btn.addEventListener("click", function () { show(Number(btn.dataset.goto) || 1, true); });
  });

  // ------------------------------------------------------------------ tips
  Array.prototype.forEach.call(form.querySelectorAll(".tip"), function (tip) {
    tip.addEventListener("click", function () {
      var text = tip.closest(".measure-field").querySelector(".tip-text");
      if (!text) return;
      var open = text.hidden;
      text.hidden = !open;
      tip.setAttribute("aria-expanded", open ? "true" : "false");
      tip.classList.toggle("is-open", open);
    });
  });

  // ------------------------------------------- remember measurements (local)
  var STORE_KEY = "gojias.measurements.v1";
  var remember = document.getElementById("remember-measurements");

  function savedMeasurements() {
    try { return JSON.parse(localStorage.getItem(STORE_KEY) || "null"); } catch (e) { return null; }
  }

  var saved = savedMeasurements();
  if (saved && saved.values) {
    var unitRadio = form.querySelector('input[name="unit"][value="' + (saved.unit || "cm") + '"]');
    if (unitRadio) unitRadio.checked = true;
    Object.keys(saved.values).forEach(function (key) {
      var input = form.querySelector('input[name="measure_' + key + '"]');
      if (input && !input.value) input.value = saved.values[key];
    });
    if (remember) remember.checked = true;
  }

  form.addEventListener("submit", function (e) {
    // Re-check every step so a customer cannot skip ahead via the review.
    for (var s = 1; s <= steps.length; s++) {
      if (!validate(s)) {
        e.preventDefault();
        show(s, true);
        return;
      }
    }
    if (remember) {
      if (remember.checked) {
        var values = {};
        var active = groups.filter(function (g) { return !g.hidden; })[0];
        if (active) {
          active.querySelectorAll(".measure-field input").forEach(function (input) {
            if (input.value.trim()) values[input.name.replace("measure_", "")] = input.value.trim();
          });
        }
        try {
          localStorage.setItem(STORE_KEY, JSON.stringify({ unit: unit(), values: values }));
        } catch (err) { /* storage may be unavailable */ }
      } else {
        try { localStorage.removeItem(STORE_KEY); } catch (err) {}
      }
    }
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = "Sending…";
    }
  });

  // -------------------------------------------------------------- start up
  var errorStep = document.getElementById("wizard-progress");
  var startAt = 1;
  if (errorStep && errorStep.dataset.errorStep) {
    var n = Number(errorStep.dataset.errorStep);
    if (n >= 1 && n <= steps.length) startAt = n;
  }
  syncDesign();
  syncDelivery();
  show(startAt);
  updateEstimate();
})();
