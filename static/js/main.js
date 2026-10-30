/* Progressive enhancement only: every page works without JavaScript. */

(function () {
  "use strict";

  // Mobile navigation toggle.
  var toggle = document.querySelector(".nav-toggle");
  var nav = document.getElementById("site-nav");
  if (toggle && nav) {
    toggle.addEventListener("click", function () {
      var open = nav.classList.toggle("is-open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
      toggle.setAttribute("aria-label", open ? "Close menu" : "Open menu");
    });
  }

  // Swap the main gallery image when a thumbnail is clicked.
  var main = document.getElementById("gallery-main");
  if (main) {
    var thumbs = document.querySelectorAll(".gallery-thumbs .thumb");
    Array.prototype.forEach.call(thumbs, function (thumb) {
      thumb.addEventListener("click", function () {
        var src = thumb.getAttribute("data-src");
        if (!src) return;
        main.src = src;
        Array.prototype.forEach.call(thumbs, function (t) {
          t.classList.remove("is-active");
        });
        thumb.classList.add("is-active");
      });
    });
  }

  // Filter forms submit on change so browsing feels immediate.
  var forms = document.querySelectorAll(".filters");
  Array.prototype.forEach.call(forms, function (form) {
    Array.prototype.forEach.call(form.querySelectorAll("select"), function (select) {
      select.addEventListener("change", function () { form.submit(); });
    });
  });

  // Add a shadow to the sticky header once the page has scrolled.
  var header = document.getElementById("site-header");
  if (header) {
    var onScroll = function () {
      header.classList.toggle("is-scrolled", window.scrollY > 8);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
  }
})();
