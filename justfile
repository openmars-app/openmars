# Open Mars task runner. Run `just` to list every recipe.
# Backend, frontend and gateway recipes live in modules: `just backend <recipe>`, `just frontend <recipe>`, `just proxy <recipe>`.

set shell := ["bash", "-euo", "pipefail", "-c"]

mod backend
mod frontend
mod proxy

[private]
default:
    @just --list --unsorted --list-submodules

[group('setup')]
[doc('Install Go modules and frontend and gateway dependencies')]
deps: backend::deps frontend::deps proxy::deps

[group('dev')]
[doc('Build once and run the app on :3001 without hot reload, serving the built frontend from the backend')]
run: frontend::build
    {{ just_executable() }} backend serve

[group('dev')]
[doc('Run the backend (:3001, hot reload) and frontend (:3000) together; --proxy also runs the gateway (:4000) in front of them. Ctrl-C or any one exiting stops all')]
[arg("proxy", long="proxy", value="true")]
dev proxy="false":
    #!/usr/bin/env bash
    set -uo pipefail
    # Each server runs in its own process group so shutdown reaches its children
    # (air's rebuilt binary, vite under bun). A server still running five seconds
    # after SIGTERM is killed.
    groups=()
    stop() {
        trap - EXIT INT TERM
        for group in "${groups[@]}"; do kill -TERM -- "-$group" 2>/dev/null; done
        for _ in $(seq 50); do
            alive=false
            for group in "${groups[@]}"; do kill -0 -- "-$group" 2>/dev/null && alive=true; done
            $alive || return
            sleep 0.1
        done
        for group in "${groups[@]}"; do kill -KILL -- "-$group" 2>/dev/null; done
    }
    trap stop EXIT
    trap 'exit 130' INT TERM
    setsid {{ just_executable() }} backend dev &
    groups+=($!)
    if [ "{{ proxy }}" = "true" ]; then
        export OPENMARS_DEV_ORIGIN=http://localhost:3000
    fi
    setsid {{ just_executable() }} frontend dev &
    groups+=($!)
    if [ "{{ proxy }}" = "true" ]; then
        OPENMARS_SERVERS="${OPENMARS_SERVERS:-local=http://localhost:3000}" setsid {{ just_executable() }} proxy dev &
        groups+=($!)
    fi
    wait -n

[group('dev')]
[doc('Stop stray dev servers: anything on :3000/:3001/:4000 plus air and vite processes from this repo')]
kill:
    #!/usr/bin/env bash
    set -uo pipefail
    pids=$( (lsof -t -i:3000 -i:3001 -i:4000 -sTCP:LISTEN 2>/dev/null; pgrep -f "{{ justfile_directory() }}.*(air|vite)" 2>/dev/null) | sort -u)
    if [ -z "$pids" ]; then
        echo "No dev servers running."
        exit 0
    fi
    echo "Stopping: $(echo $pids)"
    kill -TERM $pids 2>/dev/null
    sleep 2
    remaining=$(for pid in $pids; do kill -0 "$pid" 2>/dev/null && echo "$pid"; done)
    if [ -n "$remaining" ]; then
        kill -KILL $remaining 2>/dev/null
    fi

[group('check')]
[doc('Fail if any backend, frontend or gateway source is not formatted')]
format-check: backend::format-check frontend::format-check proxy::format-check

[group('check')]
[doc('Lint backend, frontend and gateway')]
lint: backend::lint frontend::lint proxy::lint

[group('check')]
[doc('Type-check the frontend and gateway')]
typecheck: frontend::typecheck proxy::typecheck

[group('check')]
[doc('Run backend and gateway tests')]
test: backend::test proxy::test

[group('check')]
[doc('Fail if the committed generated TypeScript types are out of date')]
generate-check: backend::generate-check

[group('check')]
[doc('Run every check CI runs, without modifying files')]
check: backend::check frontend::check proxy::check

[group('fix')]
[doc('Format backend, frontend and gateway source')]
format: backend::format frontend::format proxy::format

[group('fix')]
[doc('Format, regenerate types, then run every check CI runs')]
prepare-for-commit:
    {{ just_executable() }} format
    {{ just_executable() }} generate
    {{ just_executable() }} check

[group('codegen')]
[doc('Generate frontend TypeScript types from the backend DTOs')]
generate: backend::generate

[group('build')]
[doc('Build the backend binary, the frontend bundle and the gateway')]
build: backend::build frontend::build proxy::build

[group('build')]
[doc('Remove backend, frontend and gateway build output')]
clean: backend::clean frontend::clean proxy::clean

[group('release')]
[doc('Check that changelog/<tag>/ has a valid CHANGELOG.md and CHANGELOG-USER.md')]
changelog-check tag:
    cd backend && go run ./cmd/changelog check ../changelog {{ tag }}

[group('release')]
[doc('Tag the current main as <tag> and push it, which starts the release workflow. Needs both changelog files for <tag> on main')]
release tag:
    #!/usr/bin/env bash
    set -euo pipefail
    tag="{{ tag }}"
    fail() { echo "release: $*" >&2; exit 1; }
    [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "$tag is not vMAJOR.MINOR.PATCH"
    [ "$(git branch --show-current)" = "main" ] || fail "releases are tagged from main"
    [ -z "$(git status --porcelain)" ] || fail "the working tree has uncommitted changes"
    git fetch --quiet origin main --tags
    [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || fail "main is not at origin/main; pull or push first"
    ! git rev-parse --quiet --verify "refs/tags/$tag" >/dev/null || fail "tag $tag already exists"
    {{ just_executable() }} changelog-check "$tag"
    git tag -a "$tag" -m "Open Mars $tag"
    git push origin "$tag"

[group('deploy')]
[doc('Build and deploy to the Raspberry Pi (see scripts/deploy-pi.sh)')]
deploy-pi:
    ./scripts/deploy-pi.sh
