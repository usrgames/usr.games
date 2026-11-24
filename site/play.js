// usr.games' script for a game's page: Play starts the game right here, in place of its screen.
// It is the game's own release, built for a browser, run by glonk's player from site/play/glonk:
// glonk.js starts it and hands it the keys, and xterm.js draws it. Nothing of either loads until a
// visitor asks to play, by Play here or by arriving at #play from the front page. Without
// scripts, or without a keyboard and room, the page is the same with a note in place of Play.
"use strict";

(() => {
  const screen = document.getElementById("play");
  const page = screen?.closest(".page");
  if (!page || !screen.dataset.game) return;
  const { name, game, exec, player } = screen.dataset;

  // The games lay themselves out for the size they are given, which games.json sets for each: a
  // game whose drawing costs more for each cell plays smaller, and so faster. A cell is never
  // narrower than 5 pixels, a font of 10, nor wider than 12, a font of 24.
  const cols = Number(screen.dataset.cols), rows = Number(screen.dataset.rows), narrowest = 10, widest = 24;

  // xterm.js draws the game on a WebGL2 canvas. Without one it draws each cell with a style
  // attribute, which the site's content policy refuses, and the game comes out in one colour, so
  // a browser with none is not offered Play.
  let webgl;
  const hasWebGL = () => {
    if (webgl === undefined) {
      try {
        const gl = document.createElement("canvas").getContext("webgl2");
        webgl = !!gl;
        gl?.getExtension("WEBGL_lose_context")?.loseContext();
      } catch {
        webgl = false;
      }
    }
    return webgl;
  };

  const el = (tag, props, ...children) => {
    const e = Object.assign(document.createElement(tag), props);
    e.append(...children);
    return e;
  };

  // A keyboard, and room for the game: a fine pointer is the best a page can tell of the first.
  const canPlay = () =>
    typeof WebAssembly === "object" && "DecompressionStream" in window &&
    matchMedia(`(any-pointer: fine) and (min-width: ${cols * narrowest * 0.5 + 80}px)`).matches && hasWebGL();

  const button = el("button", { type: "button", className: "rev start", textContent: "Play" });
  const offer = el("p", { className: "offer" });
  const say = (text) => offer.replaceChildren(el("span", { className: "dim", textContent: text }));
  const show = () => {
    if (page.classList.contains("playing")) return;
    if (canPlay()) offer.replaceChildren(button);
    else say(`With a keyboard and a wider screen, ${name} plays right here. Or take it home.`);
  };
  screen.append(offer);
  show();
  addEventListener("resize", show);

  const loaded = {};
  const load = (tag, props) => (loaded[props.src ?? props.href] ??= new Promise((ok, fail) => {
    document.head.append(el(tag, { ...props, onload: ok, onerror: () => fail(new Error("it could not be loaded")) }));
  }));

  async function play() {
    if (page.classList.contains("playing") || !canPlay()) return;
    button.disabled = true;
    button.textContent = "Loading";
    const box = el("div", { className: "game", role: "application", ariaLabel: `${name}, playing. Tab leaves the game.` });
    const stop = el("button", { type: "button", className: "rev start", textContent: "Stop" });
    const bar = el("p", { className: "bar" }, stop, el("span", { className: "dim", textContent: "Tab leaves the game." }));

    const end = (how) => {
      box.remove();
      bar.remove();
      page.classList.remove("playing");
      if (location.hash === "#play") history.replaceState(null, "", location.pathname);
      button.disabled = false;
      button.textContent = "Play";
      show();
      if (how !== "quit") offer.append(el("span", { className: "dim", textContent: ` The game stopped: ${how}.` }));
      button.focus();
    };

    try {
      await load("link", { rel: "stylesheet", href: player + "xterm.css" });
      for (const src of [player + "xterm.js", player + "addon-webgl.js", exec, player + "glonk.js"]) {
        await load("script", { src });
      }
      page.classList.add("playing");
      page.prepend(box, bar);
      // A cell of VT323 is 0.4em wide and 1em tall. xterm.js 6 makes a cell the font's width in
      // device pixels, rounded down, plus the letter spacing, rounded, in device pixels too; this
      // spacing makes each cell half as wide as it is tall, so a game's half-block pixels come out
      // square. shortcut: measured in Chrome with xterm.js 6.0.0; measure again when
      // scripts/player.sh moves the player to another xterm.js.
      const dpr = devicePixelRatio || 1;
      let size = Math.max(narrowest, Math.min(widest, Math.floor(page.clientWidth / (cols * 0.5))));
      if ((size * dpr) % 2) size -= 1;
      const spacing = Math.max(0, (size * dpr) / 2 - Math.floor(0.4 * size * dpr));
      await document.fonts.load(`${size}px VT323`);
      const running = await glonkPlay(box, {
        game, cols, rows,
        terminal: {
          fontFamily: '"VT323", monospace', fontSize: size, letterSpacing: spacing, lineHeight: 1,
          theme: { background: "#030a05", foreground: "#45ff86" },
        },
      });
      stop.addEventListener("click", running.stop);
      end(await running.done);
    } catch (err) {
      end(err.message);
    }
  }

  button.addEventListener("click", play);
  if (location.hash === "#play") play();
})();
