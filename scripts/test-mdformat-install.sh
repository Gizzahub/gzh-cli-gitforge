#!/bin/bash
# 실제 Make 계약을 네트워크 없는 설치 fixture로 검사한다.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
if [ "$#" -ne 1 ]; then
	echo "usage: $0 <prepared-mdformat-python>" >&2
	exit 2
fi
"$1" -I - "$repo_root" <<'PY'
import datetime
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile

repo = Path(sys.argv[1])
temporary = repo / 'tmp'
temporary.mkdir(exist_ok=True)
root = None
failure = None
passed = []

def require(condition, detail):
    if not condition:
        raise RuntimeError(detail)

def interrupted(number, frame):
    raise SystemExit(128 + number)

def write(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    path.chmod(0o700)

try:
    for name in ('SIGTERM', 'SIGHUP'):
        number = getattr(signal, name, None)
        if number is not None:
            signal.signal(number, interrupted)
    root = Path(tempfile.mkdtemp(prefix='mdformat-install-tests.', dir=temporary))
    (root / 'owner.json').write_text(json.dumps({
        'owner': os.environ.get('CE_TASK_ACTOR', 'test-runner'),
        'purpose': 'offline mdformat installer regression fixture',
        'created_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'source': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip(),
        'cleanup': 'only this newly-created temporary child',
    }))
    shim = root / 'python-shim.py'
    shim.write_text('''import importlib.metadata as metadata
import json, os, sys, types
from pathlib import Path
args = sys.argv[1:]
if args[:2] != ["-I", "-c"]: raise RuntimeError(args)
version = (Path(os.environ["TEST_CASE"]) / "runtime-version").read_text().strip()
pins = dict(line.split("==") for line in Path(os.environ["TEST_CONSTRAINTS"]).read_text(encoding="utf-8").splitlines() if line and not line.startswith(chr(35)))
metadata.distributions = lambda name: [types.SimpleNamespace(version="0.invalid" if os.environ.get("TEST_BAD_DEPS") else pins.get(name))]
sys.version_info = tuple(map(int, version.split(".")))
sys.argv = ["-c"] + args[3:]
# 실제 의존성 검사 식의 기본 locale decoding을 Windows CP1252처럼 재현한다.
if os.environ.get("TEST_DEFAULT_ENCODING"):
    import builtins
    original_open = builtins.open
    def locale_open(file, *args, **kwargs):
        kwargs.setdefault("encoding", os.environ["TEST_DEFAULT_ENCODING"])
        return original_open(file, *args, **kwargs)
    builtins.open = locale_open
exec(args[2])
''')
    uv_shim = root / 'uv-shim.py'
    uv_shim.write_text('''import json, os, sys
from pathlib import Path
case = Path(os.environ["TEST_CASE"])
seed = sys.argv[1:] == ["seed"]
if not seed:
    with (case / "uv-calls.jsonl").open("a") as log:
        log.write(json.dumps({"args":sys.argv[1:], "python":os.environ.get("UV_PYTHON")}) + "\\n")
    if sys.argv[1:3] != ["tool", "install"]: raise RuntimeError(sys.argv)
    if os.environ.get("TEST_UV_FAIL"):
        sys.exit(17)
exe = os.environ.get("TEST_GOEXE", "")
python = case / "bin/tools/python/mdformat" / ("Scripts/python.exe" if exe else "bin/python")
formatter = case / ("bin/tools/mdformat" + exe)
python.parent.mkdir(parents=True, exist_ok=True)
formatter.parent.mkdir(parents=True, exist_ok=True)
(case / "runtime-version").write_text(os.environ.get("TEST_INSTALL_RUNTIME", "3.14.5"))
if os.environ.get("TEST_NO_PYTHON"):
    if python.exists(): python.unlink()
else:
    python.write_text(chr(35) + '!/bin/sh\\nexec "$TEST_HOST_PYTHON" "$TEST_PY_SHIM" "$@"\\n')
    python.chmod(0o700)
formatter.write_text(chr(35) + '!/bin/sh\\nprintf "%s\\n" "${TEST_VERSION_OUTPUT:-mdformat 1.0.0 (mdformat-gfm 1.0.0, mdformat_frontmatter 2.1.2)}"\\nexit "${TEST_VERSION_EXIT:-0}"\\n')
formatter.chmod(0o700)
''')

    def case(name, existing=None, expected=True, **changes):
        work = root / name
        work.mkdir()
        constraints = work / '.make/mdformat-constraints.txt'
        constraints.parent.mkdir()
        shutil.copyfile(repo / '.make/mdformat-constraints.txt', constraints)
        if changes.get('TEST_DEFAULT_ENCODING'):
            constraints.write_text(constraints.read_text(encoding='utf-8') + '\n# UTF-8 검증\n',
                                   encoding='utf-8')
        env = {k:v for k,v in os.environ.items() if not k.startswith('TEST_')
               and k not in ('UV_PYTHON', 'MAKEFLAGS', 'MAKEOVERRIDES', 'MFLAGS')}
        env.update(TEST_CASE=str(work), TEST_HOST_PYTHON=sys.executable,
                   TEST_PY_SHIM=str(shim), TEST_UV_SHIM=str(uv_shim),
                   TEST_CONSTRAINTS=str(constraints), TEST_INSTALL_RUNTIME='3.14.5')
        env.update(changes)
        binary = work / 'mock-bin'
        binary.mkdir()
        write(binary / 'go', '#!/bin/sh\n[ "$*" = "env GOEXE" ] || exit 64\nprintf "%s" "${TEST_GOEXE:-}"\n')
        write(binary / 'uv', '#!/bin/sh\nexec "$TEST_HOST_PYTHON" "$TEST_UV_SHIM" "$@"\n')
        env['PATH'] = str(binary) + os.pathsep + env['PATH']
        if existing:
            seed = dict(env, TEST_INSTALL_RUNTIME=existing)
            subprocess.run([sys.executable, str(uv_shim), 'seed'], env=seed, check=True)
        result = subprocess.run(['make', '--no-print-directory', '-C', str(work),
                                 '-f', str(repo / '.make/tools.mk'), 'install-mdformat', 'GO=go'],
                                env=env, capture_output=True, text=True, timeout=30)
        require((result.returncode == 0) == expected, (name, result.stdout, result.stderr))
        calls = work / 'uv-calls.jsonl'
        observed = [json.loads(line) for line in calls.read_text().splitlines()] if calls.exists() else []
        passed.append(name)
        print('PASS', name, 'make_exit=' + str(result.returncode), 'installs=' + str(len(observed)))
        return work, observed, result

    for version in ('3.11.0', '3.14.5', '3.15.0'):
        _, calls, _ = case('reuse-' + version, existing=version)
        require(not calls, version)
    _, calls, _ = case('old-runtime', existing='3.10.12')
    require(len(calls) == 1 and calls[0]['python'] == '>=3.11', calls)
    _, calls, _ = case('fresh-default')
    require(len(calls) == 1 and calls[0]['python'] == '>=3.11', calls)
    request = '/Python Builds/newer/python3'
    _, calls, _ = case('explicit-request', UV_PYTHON=request)
    require(len(calls) == 1 and calls[0]['python'] == request, calls)
    _, calls, _ = case('explicit-empty-request', UV_PYTHON='')
    require(len(calls) == 1 and calls[0]['python'] == '', calls)
    _, calls, _ = case('reuse-with-request', existing='3.14.5', UV_PYTHON='3.10')
    require(not calls, calls)
    case('postinstall-too-old', expected=False, UV_PYTHON='3.10', TEST_INSTALL_RUNTIME='3.10.12')
    case('install-failure', expected=False, TEST_UV_FAIL='1')
    case('missing-interpreter', expected=False, TEST_NO_PYTHON='1')
    case('version-producer-failure', existing='3.14.5', expected=False, TEST_VERSION_EXIT='42')
    case('dependency-mismatch', existing='3.14.5', expected=False, TEST_BAD_DEPS='1')
    case('windows-path-shape', TEST_GOEXE='.exe')
    case('windows-default-encoding', TEST_DEFAULT_ENCODING='cp1252')
    print('mdformat installer fixtures: PASS (' + str(len(passed)) + ' cases; simulated runtimes, not native Windows)')
except BaseException as error:
    failure = error
finally:
    # 삭제 대상을 열거하고 이 실행이 만든 임시 자식만 회수한다.
    if root is not None:
        print('cleanup inventory:', root)
        for path in sorted(root.rglob('*')):
            print(' ', path.relative_to(root))
        try:
            require(root.parent == temporary and root.name.startswith('mdformat-install-tests.'), str(root))
            require(root.is_dir() and not root.is_symlink(), str(root))
            shutil.rmtree(root)
        except BaseException as cleanup_error:
            print('temporary fixture cleanup failed:', root, cleanup_error, file=sys.stderr)
            if failure is None:
                failure = cleanup_error
if failure is not None:
    raise failure
PY
