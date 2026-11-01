// Browser playground for vrk. Runs the real tools, compiled to WebAssembly
// (make playground), against a tiny shell: `echo`/`printf`/`cat <file>` as the
// first stage, then `| vrk <tool> ...` stages. Stages run one after another,
// each on the full output of the one before.
(function () {
    "use strict";

    // Keep in sync with the tools map in internal/playground/main/main.go.
    var PLAYGROUND_TOOLS = ["assert", "base", "chunk", "digest", "emit", "epoch", "jsonl", "jwt", "links", "mask", "moniker", "pct", "plain", "recase", "sip", "slug", "sse", "throttle", "tok", "urlinfo", "uuid", "validate"];

    // Files that `cat` can read. The real vrk reads your files; the playground has these.
    var FILES = {
        "README.md": "# Release notes\n\nWe moved the API to [v2](https://api.example.com/v2) and updated the [migration guide](https://docs.example.com/migrate).\n\nDeploy key for staging: sk-ant-api03-Xk9fQ2mLp7RtVw3ZbN8cYd4HjE6sUa1G\n\nQuestions go to support@example.com.\n",
        "users.jsonl": '{"name":"ana","age":31}\n{"name":"bo","age":"unknown"}\n{"name":"cy","age":27}\n',
        "stream.sse": 'data: {"choices":[{"delta":{"content":"Hello"}}]}\n\ndata: {"choices":[{"delta":{"content":", world"}}]}\n\ndata: [DONE]\n\n'
    };

    var enc = new TextEncoder();
    var dec = new TextDecoder();

    // ---- fs shim for wasm_exec.js: stdin from a buffer, stdout/stderr to arrays ----
    var io = { stdin: new Uint8Array(0), pos: 0, out: [], err: [] };
    function enosys() { var e = new Error("not implemented"); e.code = "ENOSYS"; return e; }
    function nope() { arguments[arguments.length - 1](enosys()); }
    globalThis.fs = {
        constants: { O_WRONLY: -1, O_RDWR: -1, O_CREAT: -1, O_TRUNC: -1, O_APPEND: -1, O_EXCL: -1, O_DIRECTORY: -1 },
        writeSync: function (fd, buf) { (fd === 1 ? io.out : io.err).push(buf.slice()); return buf.length; },
        write: function (fd, buf, offset, length, position, cb) {
            if (offset !== 0 || length !== buf.length || position !== null) { cb(enosys()); return; }
            cb(null, this.writeSync(fd, buf));
        },
        read: function (fd, buffer, offset, length, position, cb) {
            if (fd !== 0) { cb(enosys()); return; }
            var n = Math.min(length, io.stdin.length - io.pos);
            buffer.set(io.stdin.subarray(io.pos, io.pos + n), offset);
            io.pos += n;
            cb(null, n);
        },
        fsync: function (fd, cb) { cb(null); },
        chmod: nope, chown: nope, close: nope, fchmod: nope, fchown: nope, fstat: nope, ftruncate: nope,
        lchown: nope, link: nope, lstat: nope, mkdir: nope, open: nope, readdir: nope, readlink: nope,
        rename: nope, rmdir: nope, stat: nope, symlink: nope, truncate: nope, unlink: nope, utimes: nope
    };

    function concat(chunks) {
        var len = 0, i, off = 0;
        for (i = 0; i < chunks.length; i++) len += chunks[i].length;
        var all = new Uint8Array(len);
        for (i = 0; i < chunks.length; i++) { all.set(chunks[i], off); off += chunks[i].length; }
        return all;
    }

    // ---- wasm loading: fetched once, on first use ----
    var modulePromise = null;
    function loadModule() {
        if (modulePromise) return modulePromise;
        var script = new Promise(function (resolve, reject) {
            var s = document.createElement("script");
            s.src = "/play/wasm_exec.js";
            s.onload = resolve;
            s.onerror = function () { reject(new Error("could not load /play/wasm_exec.js")); };
            document.head.appendChild(s);
        });
        var wasm = fetch("/play/vrk.wasm").then(function (r) {
            if (!r.ok) throw new Error("could not load /play/vrk.wasm (" + r.status + ")");
            return WebAssembly.compileStreaming ? WebAssembly.compileStreaming(r) : r.arrayBuffer().then(WebAssembly.compile);
        });
        modulePromise = Promise.all([script, wasm]).then(function (v) { return v[1]; });
        modulePromise.catch(function () { modulePromise = null; });
        return modulePromise;
    }

    // One vrk process. A fresh instance per run, so no state leaks between runs.
    function runVrk(argv, stdin) {
        return loadModule().then(function (mod) {
            io = { stdin: stdin, pos: 0, out: [], err: [] };
            var go = new Go();
            var code = 0;
            go.argv = ["vrk"].concat(argv);
            go.env = { HOME: "/home/play" };
            go.exit = function (c) { code = c; };
            return WebAssembly.instantiate(mod, go.importObject).then(function (inst) {
                return go.run(inst);
            }).then(function () {
                return { out: concat(io.out), err: dec.decode(concat(io.err)), code: code };
            });
        });
    }

    // ---- tiny shell ----

    // Splits a command line into stages of words. Handles 'single', "double"
    // quotes, backslash escapes, line continuations and unquoted |. A line that
    // starts with | continues the one before, so pipes can be written one per line.
    function parse(line) {
        var stages = [[]], word = null, i = 0, c, q = null;
        line = line.replace(/\\\r?\n/g, " ").replace(/\r?\n(?=[ \t]*\|)/g, " ");
        function push() { if (word !== null) { stages[stages.length - 1].push(word); word = null; } }
        for (; i < line.length; i++) {
            c = line[i];
            if (q === "'") { if (c === "'") q = null; else word += c; continue; }
            if (q === '"') {
                if (c === '"') q = null;
                else if (c === "\\" && '"\\$`'.indexOf(line[i + 1]) >= 0) word += line[++i];
                else word += c;
                continue;
            }
            if (c === "'" || c === '"') { q = c; if (word === null) word = ""; continue; }
            if (c === "\\" && i + 1 < line.length) { word = (word || "") + line[++i]; continue; }
            if (c === "|") { push(); stages.push([]); continue; }
            if (/\s/.test(c)) { push(); continue; }
            if (c === "#" && word === null) break;
            word = (word || "") + c;
        }
        if (q) throw new Error("unclosed " + q + " quote");
        push();
        stages.forEach(function (s) { if (!s.length) throw new Error("empty pipeline stage"); });
        return stages;
    }

    function printfEscapes(s) {
        return s.replace(/\\(n|t|r|\\|"|')/g, function (_, e) {
            return { n: "\n", t: "\t", r: "\r", "\\": "\\", '"': '"', "'": "'" }[e];
        });
    }

    // The non-vrk commands the playground understands.
    var builtins = {
        echo: function (args) {
            var nl = true;
            if (args[0] === "-n") { nl = false; args = args.slice(1); }
            return { out: args.join(" ") + (nl ? "\n" : ""), code: 0 };
        },
        printf: function (args) {
            if (!args.length) return { err: "printf: usage: printf format [arguments]\n", code: 2 };
            var rest = args.slice(1);
            var out = printfEscapes(args[0]).replace(/%[sd%]/g, function (m) {
                return m === "%%" ? "%" : (rest.length ? rest.shift() : "");
            });
            return { out: out, code: 0 };
        },
        cat: function (args) {
            var out = "";
            for (var i = 0; i < args.length; i++) {
                if (!(args[i] in FILES)) {
                    return { err: "cat: " + args[i] + ": not in the playground (try " + Object.keys(FILES).join(", ") + ")\n", code: 1 };
                }
                out += FILES[args[i]];
            }
            return { out: out, code: 0 };
        }
    };

    function runStage(words, stdin) {
        var cmd = words[0], args = words.slice(1);
        if (cmd === "vrk") {
            if (!args.length || args[0] === "help" || args[0] === "--help") {
                return Promise.resolve({ out: enc.encode("tools in the playground:\n  " + PLAYGROUND_TOOLS.join(" ") + "\nrun 'vrk <tool> --help' for flags\n"), err: "", code: 0 });
            }
            return runVrk(args, stdin);
        }
        if (builtins[cmd]) {
            var r = builtins[cmd](args);
            return Promise.resolve({ out: enc.encode(r.out || ""), err: r.err || "", code: r.code });
        }
        return Promise.resolve({ out: new Uint8Array(0), err: cmd + ": not available here. The playground runs vrk, echo, printf and cat.\n", code: 127 });
    }

    // Runs a whole command line. Like bash without pipefail, the exit code is the last stage's.
    function runLine(line, stdinText) {
        var stages;
        try { stages = parse(line); } catch (e) { return Promise.resolve({ out: "", err: "parse error: " + e.message + "\n", code: 2 }); }
        var errs = "", trace = [];
        var p = Promise.resolve({ out: enc.encode(stdinText || ""), code: 0 });
        stages.forEach(function (words) {
            p = p.then(function (prev) {
                return runStage(words, prev.out).then(function (r) {
                    errs += r.err;
                    trace.push({ cmd: words.join(" "), code: r.code, bytes: r.out.length });
                    return r;
                });
            });
        });
        return p.then(function (r) { return { out: dec.decode(r.out), err: errs, code: r.code, trace: trace }; });
    }

    // ---- UI ----
    function init(root) {
        var input = root.querySelector("[data-pg-input]");
        var stdinBox = root.querySelector("[data-pg-stdin]");
        var output = root.querySelector("[data-pg-output]");
        var status = root.querySelector("[data-pg-status]");
        var runBtn = root.querySelector("[data-pg-run]");
        var does = root.querySelector("[data-pg-does]");
        var traceList = root.querySelector("[data-pg-trace]");
        var hl = root.querySelector("[data-pg-hl]");
        var recipes = root.querySelectorAll("[data-pg-recipe]");
        // A compact console shows one example and leaves the page URL alone.
        var compact = root.hasAttribute("data-pg-compact");
        var example = { cmd: input.value, does: does ? does.textContent : "" };
        var busy = false;

        // What each exit code means, shown next to it.
        var MEANING = { 0: "success", 1: "error or failed check", 2: "usage error", 127: "command not found" };

        // Colours the copy of the command that sits under the transparent textarea.
        // Prism loads after this script, so the first call may fall back to plain text.
        var grammar = null;
        function paint() {
            var v = input.value;
            if (!grammar && window.Prism && Prism.languages.bash) {
                // Bash plus `vrk <tool>`, first so it wins. Kept local so other code blocks on the page are unchanged.
                grammar = Object.assign({ vrk: { pattern: /\bvrk(?:[ \t]+[a-z]+\b)?/, alias: "function" } }, Prism.languages.bash);
            }
            if (grammar) hl.innerHTML = Prism.highlight(v, grammar, "bash");
            else hl.textContent = v;
            // A pre drops a trailing newline, so the textarea would be one line taller.
            if (/\n$/.test(v)) hl.appendChild(document.createTextNode(" "));
            hl.scrollLeft = input.scrollLeft;
        }

        function grow() {
            input.style.height = "auto";
            input.style.height = input.scrollHeight + "px";
            paint();
        }

        // Marks the recipe whose command is in the input, or none if the reader typed their own.
        function select(btn) {
            recipes.forEach(function (b) { b.setAttribute("aria-pressed", b === btn ? "true" : "false"); b.removeAttribute("data-edited"); });
            if (!does) return;
            if (btn) does.textContent = "# " + btn.dataset.does;
            else if (compact) does.textContent = input.value === example.cmd ? example.does : "# Edited. Press RUN to see what changes.";
            else does.textContent = "# Your own command. Pipe echo, printf or cat into vrk tools.";
        }

        function recipeFor(cmd) {
            for (var i = 0; i < recipes.length; i++) if (recipes[i].dataset.cmd === cmd) return recipes[i];
            return null;
        }

        function size(n) { return n < 1024 ? n + " B" : (n / 1024).toFixed(1) + " KB"; }

        // One chip per pipe stage: what ran, how much it wrote, and whether it failed.
        function renderTrace(trace) {
            traceList.textContent = "";
            if (!trace || trace.length < 2) return;
            trace.forEach(function (t) {
                var li = document.createElement("li");
                var chip = document.createElement("span");
                chip.className = "pg-stage";
                chip.dataset.code = t.code === 0 ? "ok" : "fail";
                chip.title = t.cmd;
                var c = document.createElement("span");
                c.className = "pg-stage-cmd";
                c.textContent = t.cmd;
                var m = document.createElement("span");
                m.className = "pg-stage-meta";
                m.textContent = t.code === 0 ? size(t.bytes) : "exit " + t.code;
                chip.appendChild(c);
                chip.appendChild(m);
                li.appendChild(chip);
                traceList.appendChild(li);
            });
        }

        function render(r) {
            if (traceList) renderTrace(r.trace);
            output.textContent = "";
            if (r.out) output.appendChild(document.createTextNode(r.out));
            if (r.err) {
                var e = document.createElement("span");
                e.className = "pg-stderr";
                e.textContent = r.err;
                output.appendChild(e);
            }
            if (!r.out && !r.err) {
                var none = document.createElement("span");
                none.className = "pg-muted";
                none.textContent = "(no output)";
                output.appendChild(none);
            }
            status.textContent = "exit " + r.code + (MEANING[r.code] ? " - " + MEANING[r.code] : "");
            status.dataset.code = r.code === 0 ? "ok" : "fail";
        }

        function run() {
            var line = input.value.replace(/\s+$/, "");
            if (!line || busy) return;
            busy = true;
            runBtn.disabled = true;
            status.dataset.code = "";
            status.textContent = modulePromise ? "running..." : "loading vrk (about 4 MB, once)...";
            runLine(line, stdinBox ? stdinBox.value : "").then(render, function (e) {
                render({ out: "", err: String(e.message || e) + "\n", code: 1 });
            }).then(function () {
                busy = false;
                runBtn.disabled = false;
                if (!compact) try { history.replaceState(null, "", location.pathname + "?cmd=" + encodeURIComponent(line) + (root.id ? "#" + root.id : "")); } catch (_) { /* ignore */ }
            });
        }

        runBtn.addEventListener("click", run);
        input.addEventListener("scroll", function () { hl.scrollLeft = input.scrollLeft; });
        document.addEventListener("DOMContentLoaded", paint);
        input.addEventListener("keydown", function (ev) {
            if (ev.key === "Enter" && !ev.shiftKey) { ev.preventDefault(); run(); }
        });
        // An edited recipe stays marked, so the reader still sees what they started from.
        input.addEventListener("input", function () {
            grow();
            var match = recipeFor(input.value);
            var picked = root.querySelector('[data-pg-recipe][aria-pressed="true"]');
            if (match || !picked) { select(match); return; }
            picked.setAttribute("data-edited", "");
            if (does) does.textContent = "# Edited from \"" + picked.querySelector(".pg-recipe-title").textContent + "\". Press RUN to see what changes.";
        });
        // Start the download when the reader shows intent, so the first run is fast.
        input.addEventListener("focus", function () { loadModule().catch(function () {}); }, { once: true });

        recipes.forEach(function (btn) {
            btn.addEventListener("click", function () {
                input.value = btn.dataset.cmd;
                grow();
                select(btn);
                run();
            });
        });

        var fileList = root.querySelector("[data-pg-files]");
        if (fileList) fileList.textContent = Object.keys(FILES).join("  ");

        var params = new URLSearchParams(location.search);
        if (!compact && params.get("cmd")) {
            input.value = params.get("cmd");
            select(recipeFor(input.value));
            root.scrollIntoView();
            run();
        }
        grow();
    }

    document.querySelectorAll("[data-playground]").forEach(init);

    // Exposed for docs pages: is this tool runnable in the browser?
    window.vrkPlayground = { tools: PLAYGROUND_TOOLS, run: runLine };
})();
