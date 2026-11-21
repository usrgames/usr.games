// usr.games' one script. On the front page the listing is the menu: the row you point at, or
// reach with the arrow keys, is the game whose screen shows beside it. Without this script the
// rows are still links and the first screen shows.
"use strict";

(() => {
  const rows = [...document.querySelectorAll(".list .row")];
  const screens = [...document.querySelectorAll(".preview .att")];
  if (!rows.length || rows.length !== screens.length) return;

  let on = 0;
  const pick = (i) => {
    on = i;
    rows.forEach((row, k) => row.classList.toggle("on", k === i));
    screens.forEach((screen, k) => { screen.hidden = k !== i; });
  };
  rows.forEach((row, i) => {
    row.addEventListener("mouseenter", () => pick(i));
    row.addEventListener("focus", () => pick(i));
  });

  // Up and down move through the listing from anywhere on the page that is not a place to type.
  // Return then follows the row's own link, with no help from here.
  document.addEventListener("keydown", (e) => {
    const step = { ArrowUp: -1, ArrowDown: 1 }[e.key];
    if (!step || e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.target instanceof Element && e.target.closest("input, textarea, select, [contenteditable]")) return;
    e.preventDefault();
    rows[(on + step + rows.length) % rows.length].focus();
  });

  pick(0);
})();
