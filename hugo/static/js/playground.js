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
    // quotes, backslash escapes, line continuations and unquoted |.
    function parse(line) {
        var stages = [[]], word = null, i = 0, c, q = null;
        line = line.replace(/\\\r?\n/g, " ");
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
        var errs = "";
        var p = Promise.resolve({ out: enc.encode(stdinText || ""), code: 0 });
        stages.forEach(function (words) {
            p = p.then(function (prev) {
                return runStage(words, prev.out).then(function (r) { errs += r.err; return r; });
            });
        });
        return p.then(function (r) { return { out: dec.decode(r.out), err: errs, code: r.code }; });
    }

    // ---- UI ----
    function init(root) {
        var input = root.querySelector("[data-pg-input]");
        var stdinBox = root.querySelector("[data-pg-stdin]");
        var output = root.querySelector("[data-pg-output]");
        var status = root.querySelector("[data-pg-status]");
        var runBtn = root.querySelector("[data-pg-run]");
        var busy = false;

        function render(r) {
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
            status.textContent = "exit " + r.code;
            status.dataset.code = r.code === 0 ? "ok" : "fail";
        }

        function run() {
            var line = input.value.trim();
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
                try { history.replaceState(null, "", location.pathname + "?cmd=" + encodeURIComponent(line) + (root.id ? "#" + root.id : "")); } catch (_) { /* ignore */ }
            });
        }

        runBtn.addEventListener("click", run);
        input.addEventListener("keydown", function (ev) {
            if (ev.key === "Enter" && !ev.shiftKey) { ev.preventDefault(); run(); }
        });
        // Start the download when the reader shows intent, so the first run is fast.
        input.addEventListener("focus", function () { loadModule().catch(function () {}); }, { once: true });

        root.querySelectorAll("[data-pg-example]").forEach(function (btn) {
            btn.addEventListener("click", function () {
                input.value = btn.dataset.pgExample;
                run();
            });
        });

        var fileList = root.querySelector("[data-pg-files]");
        if (fileList) fileList.textContent = Object.keys(FILES).join("  ");

        var params = new URLSearchParams(location.search);
        if (params.get("cmd")) {
            input.value = params.get("cmd");
            root.scrollIntoView();
            run();
        }
    }

    document.querySelectorAll("[data-playground]").forEach(init);

    // Exposed for docs pages: is this tool runnable in the browser?
    window.vrkPlayground = { tools: PLAYGROUND_TOOLS, run: runLine };
})();
