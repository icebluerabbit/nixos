/*
  Homepage polish — kept deliberately minimal and side-effect-free.
  Only tags <body> with the current daypart so custom.css can shift the accent
  colour through the day. No DOM rewrites, no network calls.
*/
(function () {
  function daypart() {
    var h = new Date().getHours();
    if (h < 6) return "night";
    if (h < 12) return "morning";
    if (h < 18) return "afternoon";
    return "evening";
  }
  function apply() {
    if (document.body) document.body.dataset.daypart = daypart();
  }
  apply();
  setInterval(apply, 60 * 1000);
})();
