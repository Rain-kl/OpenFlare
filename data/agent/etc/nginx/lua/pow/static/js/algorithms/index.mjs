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
(()=>{var k=()=>navigator.hardwareConcurrency!==void 0?navigator.hardwareConcurrency:1;function n(c,b,w=5,e=null,g,u=Math.trunc(Math.max(k()/2,1))){console.debug("fast algo");let s="purejs";return window.isSecureContext&&(s="webcrypto"),(navigator.userAgent.includes("Firefox")||navigator.userAgent.includes("Goanna"))&&(console.log("Firefox detected, using pure-JS fallback"),s="purejs"),new Promise((p,l)=>{let m=`${c.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256-${s}.mjs?cacheBuster=${c.version}`,f=[],d=!1,a=()=>{console.log("PoW aborted"),i(),l(new DOMException("Aborted","AbortError"))},i=()=>{d||(d=!0,f.forEach(r=>r.terminate()),e?.removeEventListener("abort",a))};if(e!=null){if(e.aborted)return a();e.addEventListener("abort",a,{once:!0})}for(let r=0;r<u;r++){let t=new Worker(m);t.onmessage=o=>{typeof o.data=="number"?g?.(o.data):(i(),p(o.data))},t.onerror=o=>{i(),l(o)},t.postMessage({data:b,difficulty:w,nonce:r,threads:u}),f.push(t)}})}var P={fast:n,slow:n};})();
//# sourceMappingURL=index.mjs.map
