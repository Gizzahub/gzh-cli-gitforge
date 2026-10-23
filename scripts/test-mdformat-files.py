"""실제 Git fixture로 변경 파일 선택과 실패 전파를 검사한다."""
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time

repo = Path(__file__).resolve().parents[1]
temporary = repo / 'tmp'
temporary.mkdir(exist_ok=True)
root = None
failure = None


def require(ok, detail):
    if not ok:
        raise RuntimeError(detail)


def interrupted(number, frame):
    raise SystemExit(128 + number)


try:
    for name in ('SIGTERM', 'SIGHUP'):
        number = getattr(signal, name, None)
        if number is not None:
            signal.signal(number, interrupted)
    root = Path(tempfile.mkdtemp(prefix='mdformat-selector-tests.', dir=temporary))
    (root / 'owner.json').write_text(json.dumps({
        'owner': 'test-runner', 'purpose': 'isolated Markdown selector regression',
        'created_epoch': time.time(), 'source': str(repo),
    }))
    work = root / 'repository'
    work.mkdir()
    env = dict(os.environ, GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull)
    # 외부 Git 명령을 포함한 fixture는 각각 잠정 30초 예산을 쓴다.
    def run(args, **kwargs):
        return subprocess.run(args, cwd=work, env=env, capture_output=True,
                              text=True, timeout=30, **kwargs)

    def git(*args):
        return run(['git', *args], check=True)

    git('init', '-q')
    git('config', 'user.name', 'fixture')
    git('config', 'user.email', 'fixture@example.invalid')
    git('config', 'commit.gpgsign', 'false')
    (work / '.gitignore').write_text('ignored/\n')
    (work / 'unchanged.md').write_text('old unformatted content\n')
    (work / 'deleted.md').write_text('delete me\n')
    (work / 'rename-old.md').write_text('rename me\n')
    (work / 'tracked-unstaged.md').write_text('baseline\n')
    (work / 'tracked-staged.md').write_text('baseline\n')
    (work / 'type-change.md').symlink_to(work / 'unchanged.md')
    git('add', '.')
    git('commit', '-qm', 'baseline', '--no-verify')
    git('branch', 'baseline')
    (work / 'committed.md').write_text('# Committed\n')
    (work / 'escape').mkdir()
    (work / 'escape/doc.md').write_text('# Owned before symlink\n')
    git('add', '.')
    git('commit', '-qm', 'task one', '--no-verify')
    (work / 'second.md').write_text('# Second\n')
    git('add', '.')
    git('commit', '-qm', 'task two', '--no-verify')
    git('mv', 'rename-old.md', 'rename-new.md')
    (work / 'deleted.md').unlink()
    weird = ['space name.md', 'line\nbreak.md', '-option.md']
    for name in weird + ['staged.md', 'unstaged.md']:
        (work / name).write_text('# Changed\n')
    git('add', 'staged.md')
    (work / 'tracked-unstaged.md').write_text('# Changed worktree\n')
    (work / 'tracked-staged.md').write_text('# Changed index\n')
    git('add', 'tracked-staged.md')
    (work / 'type-change.md').unlink()
    (work / 'type-change.md').write_text('# Now regular\n')
    (work / 'ignored').mkdir()
    (work / 'ignored/README.md').write_text('ignored tool venv\n')
    (work / 'docs').mkdir()
    (work / 'docs/nested.md').write_text('# Nested\n')
    (work / 'linked.md').symlink_to(work / 'unchanged.md')
    (work / 'escape/doc.md').unlink()
    (work / 'escape').rmdir()
    outside = root / 'outside'
    outside.mkdir()
    (outside / 'doc.md').write_text('external content\n')
    (work / 'escape').symlink_to(outside, target_is_directory=True)
    calls = root / 'calls.json'
    formatter = root / 'formatter'
    formatter.write_text('#!/bin/sh\nexec "$TEST_PYTHON" "$TEST_RECORDER" "$@"\n')
    formatter.chmod(0o700)
    recorder = root / 'record.py'
    recorder.write_text('import json,os,sys\nfrom pathlib import Path\n'
                        'Path(os.environ["TEST_CALLS"]).write_text(json.dumps(sys.argv[1:]))\n'
                        'sys.exit(int(os.environ.get("TEST_EXIT", "0")))\n')
    env.update(TEST_PYTHON=sys.executable, TEST_RECORDER=str(recorder), TEST_CALLS=str(calls))
    command = [sys.executable, '-I', str(repo / 'scripts/mdformat-files.py')]
    args = ['changed', '--base', 'baseline', '--formatter', str(formatter), '--check']
    result = run(command + args)
    require(result.returncode == 0, result.stderr)
    expected = {'./' + name for name in weird + [
        'committed.md', 'second.md', 'staged.md', 'unstaged.md', 'rename-new.md',
        'tracked-unstaged.md', 'tracked-staged.md', 'type-change.md', 'docs/nested.md']}
    observed = json.loads(calls.read_text())
    require(observed[0] == '--check' and set(observed[1:]) == expected, observed)
    before = (work / 'unchanged.md').read_bytes()
    env['TEST_EXIT'] = '42'
    require(run(command + args).returncode == 42, 'formatter failure hidden')
    env.pop('TEST_EXIT')
    calls.unlink()
    (work / 'tracked-staged.md').write_text('baseline\n')
    partial = run(command + args)
    require(partial.returncode != 0 and not calls.exists(), 'index/worktree mismatch passed')
    require('index/worktree differ' in partial.stderr, partial.stderr)
    (work / 'tracked-staged.md').write_text('# Changed index\n')
    (work / 'staged.md').unlink()
    missing_staged = run(command + args)
    require(missing_staged.returncode != 0 and not calls.exists(), 'staged then deleted file passed')
    require('index/worktree differ' in missing_staged.stderr, missing_staged.stderr)
    (work / 'staged.md').write_text('# Changed\n')
    bad = run(command + ['changed', '--base', 'missing-ref', '--formatter', str(formatter)])
    require(bad.returncode != 0 and not calls.exists(), 'Git failure hidden')
    result = run(command + ['all', '--formatter', str(formatter), '--check'])
    require(result.returncode == 0, result.stderr)
    all_files = set(json.loads(calls.read_text()))
    require({'./unchanged.md', './docs/nested.md'} <= all_files, 'all mode omitted owned files')
    require(not {'./linked.md', './ignored/README.md', './escape/doc.md'} & all_files,
            'all mode followed excluded files')
    require((work / 'unchanged.md').read_bytes() == before, 'check mutated source')
    # 빈 저장소의 빈 선택에서는 formatter를 호출하지 않는다.
    empty = root / 'empty'
    empty.mkdir()
    init = subprocess.run(['git', 'init', '-q', str(empty)], env=env, timeout=30)
    require(init.returncode == 0, 'empty fixture initialization')
    calls.unlink()
    result = subprocess.run(command + ['all', '--formatter', str(formatter), '--check'],
                            cwd=empty, env=env, capture_output=True, timeout=30)
    require(result.returncode == 0 and not calls.exists(), 'empty selection invoked formatter')
    print('PASS: commits, index, worktree, untracked, unusual names, rename/deletion, ignored/symlink, failures, all/empty selection')
except BaseException as error:
    failure = error
finally:
    if root is not None:
        print('cleanup inventory:', root)
        for path in sorted(root.rglob('*')):
            print(' ', path.relative_to(root))
        try:
            require(root.parent == temporary and root.name.startswith('mdformat-selector-tests.'), root)
            require(root.is_dir() and not root.is_symlink(), root)
            shutil.rmtree(root)
        except BaseException as cleanup_error:
            print('temporary fixture cleanup failed:', root, cleanup_error, file=sys.stderr)
            if failure is None:
                failure = cleanup_error
if failure is not None:
    raise failure
