/**
 * Cap PoW (Proof-of-Work) 人机验证前端实现
 * 与后端 internal/util/cap 算法完全对应
 *
 * 求解在 Web Worker 中执行，不阻塞主线程 UI。
 */

// ——— Challenge / Redeem API types ———

export interface ChallengeResponse {
  challenge: { c: number; s: number; d: number };
  token: string;
  expires: number;
}

export interface RedeemResponse {
  success: boolean;
  token?: string;
  expires?: number;
  error?: string;
}

// ——— Worker 代码（内联 Blob，避免独立文件的打包配置问题）———
//
// 算法与 Go 后端 internal/util/cap/cap.go + prng.go 完全对应：
//   · FNV-1a 32-bit  (Math.imul 保证 32-bit 截断)
//   · xorshift32 PRNG → hex 字符串
//   · SubtleCrypto SHA-256 校验答案

const WORKER_SOURCE = /* js */ `
// FNV-1a 32-bit
function fnv1a(str) {
  let h = 2166136261 >>> 0;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h;
}
function fnv1aResume(state, str) {
  let h = state >>> 0;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h;
}

// xorshift32 PRNG → hex string
function prngFromHash(seed, len) {
  let s = seed >>> 0, r = '';
  while (r.length < len) {
    s ^= (s << 13) >>> 0;
    s ^= (s >>> 17) >>> 0;
    s ^= (s << 5)  >>> 0;
    s  = s >>> 0;
    r += s.toString(16).padStart(8, '0');
  }
  return r.slice(0, len);
}

// SHA-256 via SubtleCrypto (available in workers)
async function sha256Hex(input) {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(input));
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
}

// Solve one puzzle
async function solvePuzzle(token, index, size, difficulty) {
  const tFnv    = fnv1a(token);
  const saltSeed   = fnv1aResume(tFnv, String(index + 1));
  const targetSeed = fnv1aResume(saltSeed, 'd');
  const salt   = prngFromHash(saltSeed,   size);
  const target = prngFromHash(targetSeed, difficulty);

  for (let nonce = 0; nonce < 10_000_000; nonce++) {
    if ((await sha256Hex(salt + nonce)).startsWith(target)) return nonce;
  }
  throw new Error('无法在合理范围内求解第 ' + (index + 1) + ' 个难题');
}

// Entry point — receive task from main thread
self.onmessage = async ({ data }) => {
  const { token, count, size, difficulty } = data;
  try {
    const t0 = performance.now();
    const solutions = [];
    for (let i = 0; i < count; i++) {
      solutions.push(await solvePuzzle(token, i, size, difficulty));
    }
    const elapsed = ((performance.now() - t0) / 1000).toFixed(3);
    self.postMessage({ type: 'done', solutions, elapsed });
  } catch (err) {
    self.postMessage({ type: 'error', message: err.message });
  }
};
`;

// ——— 在 Web Worker 中求解，返回 Promise<number[]> ———

function solveInWorker(
  token: string,
  count: number,
  size: number,
  difficulty: number,
): Promise<{ solutions: number[]; elapsed: string }> {
  return new Promise((resolve, reject) => {
    const blob = new Blob([WORKER_SOURCE], { type: 'application/javascript' });
    const url = URL.createObjectURL(blob);
    const worker = new Worker(url);

    worker.onmessage = (e) => {
      URL.revokeObjectURL(url);
      worker.terminate();
      if (e.data.type === 'done') {
        resolve({ solutions: e.data.solutions, elapsed: e.data.elapsed });
      } else {
        reject(new Error(e.data.message));
      }
    };

    worker.onerror = (err) => {
      URL.revokeObjectURL(url);
      worker.terminate();
      reject(new Error(err.message || 'Worker 执行失败'));
    };

    worker.postMessage({ token, count, size, difficulty });
  });
}

// ——— Full Cap flow: get challenge → solve (Worker) → redeem → return X-Cap-Token ———

export async function getCapToken(scope = 'login'): Promise<string> {
  // 1. 获取难题
  const challengeRes = await fetch('/api/cap/challenge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ scope }),
  });
  if (!challengeRes.ok) {
    throw new Error('获取人机验证难题失败');
  }
  const challenge: ChallengeResponse = await challengeRes.json();
  const { c: count, s: size, d: difficulty } = challenge.challenge;

  console.groupCollapsed('[Cap] 人机验证 PoW 求解开始');
  console.log(`  难题数量 (count)    : ${count}`);
  console.log(`  验证难度 (difficulty): ${difficulty}`);
  console.log(`  盐值长度 (size)     : ${size}`);
  console.groupEnd();

  // 2. 在 Worker 中求解（不阻塞主线程）
  const t0 = performance.now();
  const { solutions, elapsed } = await solveInWorker(
    challenge.token,
    count,
    size,
    difficulty,
  );
  const wallTime = ((performance.now() - t0) / 1000).toFixed(3);

  console.groupCollapsed('[Cap] 人机验证 PoW 求解完成');
  console.log(`  难题数量 (count)    : ${count}`);
  console.log(`  验证难度 (difficulty): ${difficulty}`);
  console.log(`  Worker 耗时         : ${elapsed}s`);
  console.log(`  总耗时（含通信）     : ${wallTime}s`);
  console.log(`  Solutions           :`, solutions);
  console.groupEnd();

  // 3. 提交答案兑换一次性凭证
  const redeemRes = await fetch('/api/cap/redeem', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: challenge.token, solutions, scope }),
  });
  const redeemData: RedeemResponse = await redeemRes.json();
  if (!redeemData.success || !redeemData.token) {
    throw new Error(redeemData.error || '人机验证失败');
  }

  return redeemData.token;
}
