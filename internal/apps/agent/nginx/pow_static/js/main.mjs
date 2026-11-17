/*
@licstart  The following is the entire license notice for the
JavaScript code in this page.

Copyright (c) 2025 Xe Iaso <xe.iaso@techaro.lol>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

Includes code from https://github.com/aws/aws-sdk-js-crypto-helpers which is
used under the terms of the Apache 2 license.

@licend  The above is the entire license notice
for the JavaScript code in this page.
*/
import { updateState } from './widget.mjs';

const staticPrefix = '/.within.website/x/cmd/anubis/static/';

export function solveChallenge(config, onProgress, signal) {
  return new Promise((resolve, reject) => {
    const workers = [];
    const threads = config.algorithm === 'slow' ? 1 : Math.min(8, Math.max(1, Math.floor((navigator.hardwareConcurrency || 2) / 2)));
    const implementation = window.isSecureContext && !/Firefox|Goanna/.test(navigator.userAgent) ? 'webcrypto' : 'purejs';
    let finished = false;
    const cleanup = () => {
      workers.forEach((worker) => worker.terminate());
      signal?.removeEventListener('abort', abort);
    };
    const finish = (callback, value) => {
      if (finished) return;
      finished = true;
      cleanup();
      callback(value);
    };
    const abort = () => finish(reject, new Error('Verification timed out. Reload to try again.'));
    if (signal?.aborted) return abort();
    signal?.addEventListener('abort', abort, { once: true });
    try {
      for (let nonce = 0; nonce < threads; nonce++) {
        const worker = new Worker(`${staticPrefix}js/worker/sha256-${implementation}.mjs`);
        workers.push(worker);
        worker.onmessage = ({ data }) => typeof data === 'number' ? onProgress(data) : finish(resolve, data);
        worker.onerror = () => finish(reject, new Error('Verification worker failed. Reload to try again.'));
        worker.postMessage({ data: config.randomData, difficulty: config.difficulty, nonce, threads });
      }
    } catch (error) {
      finish(reject, error);
    }
  });
}

export async function startVerification(config) {
  const controller = new AbortController();
  const stop = () => controller.abort();
  window.addEventListener('pagehide', stop, { once: true });
  const timeout = setTimeout(stop, (config.challengeTTL || 300) * 1000);
  let text = {};
  const message = (key, fallback) => text[key] || fallback;
  try {
    const localeResponse = await fetch(`${staticPrefix}locales/${config.lang}.json`);
    if (localeResponse.ok) text = await localeResponse.json();
    if (!window.Worker) throw new Error(message('js_web_workers_error', 'Web Workers are required.'));
    if (!navigator.cookieEnabled) throw new Error(message('js_cookies_error', 'Cookies are required.'));
    updateState({ status: 'working', progress: 0, message: message('js_calculating', 'Verifying your browser…'), retryLabel: message('try_again', 'Try again') });
    const started = performance.now();
    const result = await solveChallenge(config, (hashes) => {
      const probability = Math.pow(1 - Math.pow(16, -config.difficulty), hashes);
      updateState({ status: 'working', progress: Math.min(99, (1 - probability * probability) * 100),
        hashes, elapsed: performance.now() - started,
        message: message('js_calculating', 'Verifying your browser…') });
    }, controller.signal);
    updateState({ status: 'verifying', message: message('js_verifying', 'Checking the result…') });
    const params = new URLSearchParams({ id: config.id, response: result.hash, nonce: String(result.nonce) });
    const response = await fetch(`/.within.website/x/cmd/anubis/api/pass-challenge?${params}`, {
      credentials: 'same-origin', cache: 'no-store', signal: controller.signal,
    });
    const payload = await response.json();
    if (!response.ok || !payload.ok) throw new Error(message('js_verification_failed', 'Verification failed. Reload to try again.'));
    clearTimeout(timeout);
    updateState({ status: 'success', progress: 100, message: message('js_success', 'Success!') });
    setTimeout(() => window.location.reload(), 650);
  } catch (error) {
    updateState({ status: 'error', message: error.message || message('js_calculation_error', 'Verification failed.') });
  } finally {
    clearTimeout(timeout);
    window.removeEventListener('pagehide', stop);
    controller.abort();
  }
}

const boot = document.getElementById('openflare-pow-config');
if (boot) {
  try { await startVerification(JSON.parse(boot.textContent)); }
  catch { updateState({ status: 'error', message: 'Invalid verification configuration.' }); }
}
