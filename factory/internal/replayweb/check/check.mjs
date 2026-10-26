// Drives the replay page in a real browser, inside a frame on another origin,
// as VS Code's Simple Browser shows it. bin/check-visualise-run starts the
// servers and the browser's library; this only asks the questions.
//
//   node check.mjs <frame-url> <replay-url> --cut-short <url> --send-backs <url> --markup <url> [--chromium <path>]
//
// The frame page (frame.html) shows the address given in its query string in
// a 640 x 720 frame. Everything is asked of the page inside that frame.
import { createRequire } from 'node:module';
import path from 'node:path';

const require = createRequire(path.join(process.env.NODE_PLAYWRIGHT, 'package.json'));
const { chromium } = require('playwright');

// ---------- what was asked ----------

const args = process.argv.slice(2);
const flags = {};
const positional = [];
for (let i = 0; i < args.length; i++) {
  if (args[i].startsWith('--')) flags[args[i].slice(2)] = args[++i];
  else positional.push(args[i]);
}
const [frameUrl, demoUrl] = positional;
for (const [name, value] of Object.entries({ 'frame address': frameUrl, 'replay address': demoUrl, '--cut-short': flags['cut-short'], '--send-backs': flags['send-backs'], '--markup': flags.markup })) {
  if (!value) {
    console.error(`check.mjs: ${name} is missing`);
    process.exit(2);
  }
}

// ---------- small helpers ----------

function expect(ok, what) {
  if (!ok) throw new Error(what);
}

function equal(got, want, what) {
  expect(got === want, `${what}: got ${JSON.stringify(got)}, wanted ${JSON.stringify(want)}`);
}

const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

// eventually waits for fn to return something other than false, up to a few seconds.
async function eventually(fn, what, ms = 5000) {
  const until = Date.now() + ms;
  let last;
  for (;;) {
    last = await fn();
    if (last !== false && last !== undefined) return last;
    if (Date.now() > until) throw new Error(`${what}: did not happen within ${ms}ms`);
    await pause(100);
  }
}

// seconds is a time as the page writes it (40s, 6m 40s, 1h 5m 3s), in seconds.
function seconds(words) {
  const m = /^(?:(\d+)h )?(?:(\d+)m )?(\d+)s$/.exec(words);
  expect(m, `${JSON.stringify(words)} is not a time`);
  return Number(m[1] || 0) * 3600 + Number(m[2] || 0) * 60 + Number(m[3]);
}

// ---------- a page in a frame ----------

// open opens the replay at address in the frame, and keeps what is to be
// held against it: console errors, and requests to anywhere but the two
// local servers.
async function open(browser, address) {
  const page = await browser.newPage({ viewport: { width: 1000, height: 800 } });
  const problems = [];
  const frameOrigin = new URL(frameUrl).origin, replayOrigin = new URL(address).origin;
  page.on('console', m => { if (m.type() === 'error') problems.push(`console error: ${m.text()}`); });
  page.on('pageerror', e => problems.push(`uncaught error: ${e.message}`));
  page.on('requestfailed', r => problems.push(`request failed: ${r.url()} ${r.failure()?.errorText}`));
  page.on('request', r => {
    const url = new URL(r.url());
    if (url.protocol === 'data:' || url.protocol === 'about:' || url.protocol === 'blob:') return;
    if (url.origin !== frameOrigin && url.origin !== replayOrigin) problems.push(`request to another host: ${r.url()}`);
  });
  await page.goto(`${frameUrl}?src=${encodeURIComponent(address)}`);
  const f = page.frameLocator('#replay');
  await f.locator('g.node').first().waitFor({ timeout: 10000 });
  return {
    page,
    f,
    problems,
    meta: async () => (await f.locator('#meta').textContent()).trim(),
    note: async () => (await f.locator('#note').textContent()).trim(),
    clock: async () => seconds((await f.locator('#meta').textContent()).split(' of ')[0]),
    filled: async () => parseFloat(await f.locator('#fill').evaluate(e => e.style.width)),
    node: name => f.locator(`g.node[data-name="${name}"]`),
    // atEnd presses the far right of the timeline and drags a little past
    // it, as a hand would: one pixel short of the end is seconds short of it.
    atEnd: async () => {
      const box = await f.locator('#timeline').boundingBox();
      const y = box.y + box.height / 2;
      await page.mouse.move(box.x + box.width - 1, y);
      await page.mouse.down();
      await page.mouse.move(box.x + box.width + 6, y, { steps: 3 });
      await page.mouse.up();
    },
  };
}

// ---------- the checks ----------

const checks = [];
const check = (name, run) => checks.push({ name, run });

check('the demo run: nodes, the start, playing, hand-offs, the bubble and the end', async browser => {
  const p = await open(browser, demoUrl);
  const { f } = p;

  // One node per machine on the line, and the start and finish.
  equal(await f.locator('g.node.machine').count(), 9, 'machines drawn');
  equal(await f.locator('g.node.start').count(), 1, 'start pills');
  equal(await f.locator('g.node.finish').count(), 1, 'finish pills');
  equal(await f.locator('g.node').count(), 11, 'nodes drawn');

  // Paused at the start, with nothing yet said by a machine that hasn't begun.
  equal(await p.meta().then(m => m.split(' · ')[0]), '0s of 36m 41s', 'the time at the start');
  await p.node('doer').click();
  await eventually(async () => (await f.locator('#loglines').textContent()).trim() === 'nothing yet', 'a bubble saying nothing yet');
  await f.locator('#logclose').click();
  expect(await f.locator('#log').isHidden(), 'the bubble closes');

  // Playing at 60x for 6 seconds goes past 5 minutes.
  await f.locator('.speed[data-speed="60"]').click();
  equal(await f.locator('.speed[data-speed="60"]').getAttribute('aria-pressed'), 'true', '60x pressed');
  await f.locator('#play').click();
  equal(await f.locator('#play').getAttribute('aria-label'), 'pause', 'the play button while playing');
  await pause(6000);
  await f.locator('#play').click();
  equal(await f.locator('#play').getAttribute('aria-label'), 'play', 'the play button once paused');
  const played = await p.clock();
  expect(played > 5 * 60, `after 6 seconds at 60x the time is ${played}s, not past 5m`);

  // Next hand-off moves the time on.
  const before = await p.filled();
  await f.locator('#next').click();
  const after = await p.filled();
  expect(after > before, `next hand-off did not move on: ${before}% then ${after}%`);

  // The bubble: inside the frame, its tail on the doer.
  await p.node('doer').click();
  await f.locator('#log:not([hidden])').waitFor();
  const bubble = await f.locator('body').evaluate(() => {
    const log = document.getElementById('log');
    const r = log.getBoundingClientRect();
    const tail = getComputedStyle(log, '::after');
    const box = document.querySelector('g.node[data-name="doer"] rect').getBoundingClientRect();
    return {
      left: r.left, right: r.right, top: r.top, bottom: r.bottom,
      width: innerWidth, height: innerHeight,
      tailShown: tail.display !== 'none',
      tailCentre: r.left + parseFloat(tail.left) + parseFloat(tail.width) / 2,
      doerCentre: box.left + box.width / 2,
    };
  });
  expect(bubble.left >= -0.5 && bubble.top >= -0.5 && bubble.right <= bubble.width + 0.5 && bubble.bottom <= bubble.height + 0.5,
    `the bubble is not inside the frame: ${JSON.stringify(bubble)}`);
  expect(bubble.tailShown, 'the bubble has no tail');
  expect(Math.abs(bubble.tailCentre - bubble.doerCentre) <= 20,
    `the tail's centre is ${bubble.tailCentre}, the doer's ${bubble.doerCentre}`);
  await f.locator('#logclose').click();

  // The end: how it ended, the finish lit, and a blue tick for each send-back.
  await p.atEnd();
  equal(await p.note(), 'The run finished in 36m 41s, using 613.8k tokens.', 'the note at the end');
  equal(await f.locator('g.node.finish.lit').count(), 1, 'finish lit');
  equal(await f.locator('#track i.back').count(), 2, 'blue ticks');

  equal(p.problems.join('\n'), '', 'console errors and stray requests');
  await p.page.close();
});

check('a record that stops early says so', async browser => {
  const p = await open(browser, flags['cut-short']);
  await p.atEnd();
  equal(await p.note(), 'The record ends at 19m 47s, before the run finished.', 'the note at the end');
  equal(await p.f.locator('g.node.finish.lit').count(), 0, 'finish lit');
  equal(p.problems.join('\n'), '', 'console errors and stray requests');
  await p.page.close();
});

check('a run with three send-backs', async browser => {
  const p = await open(browser, flags['send-backs']);
  equal(await p.f.locator('#track i.back').count(), 3, 'blue ticks');
  await p.atEnd();
  equal(await p.f.locator('g.node.finish.lit').count(), 1, 'finish lit');
  expect((await p.note()).startsWith('The run finished in '), `the note at the end is ${JSON.stringify(await p.note())}`);
  equal(p.problems.join('\n'), '', 'console errors and stray requests');
  await p.page.close();
});

check('a machine\'s words are shown as text, not as markup', async browser => {
  const p = await open(browser, flags.markup);
  await p.atEnd();
  await p.node('plan_gate').click();
  await p.f.locator('#log:not([hidden])').waitFor();
  const lines = await p.f.locator('#loglines').textContent();
  expect(lines.includes('<b>bold</b>'), `the bubble says ${JSON.stringify(lines)}, not <b>bold</b>`);
  equal(await p.f.locator('#loglines b').count(), 0, 'bold elements made from the text');
  equal(p.problems.join('\n'), '', 'console errors and stray requests');
  await p.page.close();
});

// ---------- run them ----------

const browser = await chromium.launch(flags.chromium ? { executablePath: flags.chromium } : {});
let failed = 0;
try {
  for (const c of checks) {
    try {
      await c.run(browser);
      console.log(`ok   ${c.name}`);
    } catch (err) {
      failed++;
      console.log(`FAIL ${c.name}\n     ${String(err.message).split('\n').join('\n     ')}`);
    }
  }
} finally {
  await browser.close();
}
process.exit(failed ? 1 : 0);
