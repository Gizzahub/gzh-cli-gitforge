"""실제 설치·launcher·선택기를 검증하는 POSIX/Windows native smoke."""
import datetime
import importlib.metadata as metadata
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys
import tempfile

repo = Path(__file__).resolve().parents[1]
temporary = repo / 'tmp'
temporary.mkdir(exist_ok=True)
root = None
failure = None
suffix = '.exe' if os.name == 'nt' else ''
relative_python = 'Scripts/python.exe' if suffix else 'bin/python'
formatter = repo / ('bin/tools/mdformat' + suffix)
python = repo / 'bin/tools/python/mdformat' / relative_python


def require(ok, detail):
    if not ok:
        raise RuntimeError(detail)


def run(command, cwd=repo, env=None, expected=0):
    result = subprocess.run([str(arg) for arg in command], cwd=cwd, env=env,
                            capture_output=True, text=True, timeout=300)
    print(json.dumps({'command': [str(arg) for arg in command], 'exit': result.returncode}))
    if expected is not None:
        require(result.returncode == expected, (result.stdout, result.stderr))
    return result


def interrupted(number, frame):
    raise SystemExit(128 + number)


def remove_readonly(function, name, error):
    # Windows Git가 만든 읽기 전용 object만 확인한 뒤 같은 파일을 회수한다.
    path = Path(name)
    require(os.name == 'nt' and path.is_relative_to(root), (name, error))
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and not path.is_symlink(), (name, error))
    print('cleanup readonly file:', path.relative_to(root))
    path.chmod(info.st_mode | stat.S_IWRITE)
    function(name)


try:
    for name in ('SIGTERM', 'SIGHUP'):
        number = getattr(signal, name, None)
        if number is not None:
            signal.signal(number, interrupted)
    require(sys.version_info >= (3, 11), sys.version)
    require(Path(sys.executable).samefile(python), ('driver must use prepared tool Python', sys.executable))
    for line in (repo / '.make/mdformat-constraints.txt').read_text(encoding='utf-8').splitlines():
        if line.strip() and not line.lstrip().startswith('#'):
            name, version = line.strip().split('==')
            require(metadata.version(name) == version, (name, metadata.version(name), version))
    before = (python.stat().st_mtime_ns, formatter.stat().st_mtime_ns)
    env = dict(os.environ, UV_PYTHON='3.10')
    run(['make', 'install-mdformat'], env=env)
    require(before == (python.stat().st_mtime_ns, formatter.stat().st_mtime_ns),
            'healthy runtime was replaced by explicit older request')
    print('native runtime:', sys.platform, sys.version, 'launcher:', formatter)
    print(run([formatter, '--version']).stdout.strip())

    root = Path(tempfile.mkdtemp(prefix='mdformat-native-tests.', dir=temporary))
    (root / 'owner.json').write_text(json.dumps({
        'owner': 'native-test-runner', 'purpose': 'real installer and selector native smoke',
        'created_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'source': run(['git', 'rev-parse', 'HEAD']).stdout.strip(),
    }))
    work = root / 'repository'
    work.mkdir()
    git_env = {key: value for key, value in os.environ.items()
               if not key.startswith('GIT_') and key not in ('MAKEFLAGS', 'MAKEOVERRIDES', 'MFLAGS')}
    git_env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull)
    for args in (['init', '-q'], ['config', 'user.name', 'native fixture'],
                 ['config', 'user.email', 'native@example.invalid'],
                 ['config', 'commit.gpgsign', 'false']):
        run(['git', *args], cwd=work, env=git_env)
    # 기준 문서는 플랫폼 기본 줄바꿈 대신 formatter 계약의 LF 바이트로 만든다.
    (work / 'baseline.md').write_bytes(b'# Baseline\n')
    run(['git', 'add', '.'], cwd=work, env=git_env)
    run(['git', 'commit', '-qm', 'baseline', '--no-verify'], cwd=work, env=git_env)
    run(['git', 'branch', 'baseline'], cwd=work, env=git_env)
    changed = work / '-native space.md'
    changed.write_text('# Native\n\n\ntext\n', encoding='utf-8')
    selector = [python, '-I', repo / 'scripts/mdformat-files.py', 'changed',
                '--base', 'baseline', '--formatter', formatter]
    unformatted = changed.read_bytes()
    require(run([*selector, '--check'], cwd=work, env=git_env, expected=None).returncode != 0,
            'unformatted native input passed')
    require(changed.read_bytes() == unformatted, 'native check modified input')
    run(selector, cwd=work, env=git_env)
    run([*selector, '--check'], cwd=work, env=git_env)
    run(['git', 'add', '--', changed.name], cwd=work, env=git_env)
    formatted = changed.read_bytes()
    changed.write_text('# Different worktree\n', encoding='utf-8')
    mismatch = run([*selector, '--check'], cwd=work, env=git_env, expected=None)
    require(mismatch.returncode != 0 and 'index/worktree differ' in mismatch.stderr,
            'native staged/worktree mismatch passed')
    changed.write_bytes(formatted)

    # Windows의 core.symlinks=false checkout을 모든 플랫폼에서 재현한다.
    run(['git', 'config', 'core.symlinks', 'false'], cwd=work, env=git_env)
    blob = run(['git', 'hash-object', '-w', '--', 'baseline.md'], cwd=work, env=git_env).stdout.strip()
    run(['git', 'update-index', '--add', '--cacheinfo', '120000', blob, 'materialized-link.md'],
        cwd=work, env=git_env)
    run(['git', 'checkout-index', '-f', '--', 'materialized-link.md'], cwd=work, env=git_env)
    link = work / 'materialized-link.md'
    require(link.is_file() and not link.is_symlink(), 'symlink fixture was not materialized')
    # 제외되지 않으면 실제 formatter가 이 잘못된 Markdown을 거부한다.
    link.write_text('# Unformatted\n\n\ntext\n', encoding='utf-8')
    all_selector = [python, '-I', repo / 'scripts/mdformat-files.py', 'all',
                    '--formatter', formatter, '--check']
    run(all_selector, cwd=work, env=git_env)
    require(link.read_text(encoding='utf-8') == '# Unformatted\n\n\ntext\n',
            'materialized symlink was modified')

    # 같은 실제 installer를 빈 도구 디렉터리에서 재실행한다.
    fresh = root / 'fresh'
    fresh.mkdir()
    isolated_python = fresh / 'python/mdformat' / relative_python
    isolated_formatter = fresh / ('mdformat' + suffix)
    installer = ['make', '--no-print-directory', '-f', repo / '.make/tools.mk',
                 'install-mdformat', 'MDFORMAT_TOOL_DIR=' + (fresh / 'python').as_posix(),
                 'FORMAT_TOOLS_DIR=' + fresh.as_posix(),
                 'MDFORMAT=' + isolated_formatter.as_posix(),
                 'MDFORMAT_PYTHON=' + isolated_python.as_posix(),
                 'MDFORMAT_CONSTRAINTS=' + (repo / '.make/mdformat-constraints.txt').as_posix()]
    install_env = {key: value for key, value in git_env.items() if key != 'UV_PYTHON'}
    run(installer, env=install_env)
    run([isolated_python, '-I', '-c', 'import sys; sys.exit(sys.version_info < (3, 11))'])
    require(run([isolated_formatter, '--version']).stdout == run([formatter, '--version']).stdout,
            'fresh native launcher bundle differs')
    print('PASS native installer, runtime floor, reuse, launcher, formatter and selector:', sys.platform)
except BaseException as error:
    failure = error
finally:
    if root is not None:
        print('cleanup inventory:', root)
        for path in sorted(root.rglob('*')):
            print(' ', path.relative_to(root))
        try:
            require(root.parent == temporary and root.name.startswith('mdformat-native-tests.'), root)
            require(root.is_dir() and not root.is_symlink(), root)
            shutil.rmtree(root, onerror=remove_readonly)
        except BaseException as error:
            print('native fixture cleanup failed:', root, repr(error), file=sys.stderr)
            if failure is None:
                failure = error
if failure is not None:
    raise failure
