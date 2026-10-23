"""저장소 소유 Markdown을 선택하고 고정 formatter의 실패를 전파한다."""
import argparse
import os
from pathlib import Path
import stat
import subprocess


def git(root, *args):
    return subprocess.check_output(['git', *args], cwd=root)


def regular(root, name):
    # 부모 symlink도 제외해 저장소 밖 문서를 따라가지 않는다.
    path = root
    for part in Path(name).parts:
        path /= part
        try:
            mode = path.lstat().st_mode
        except FileNotFoundError:
            return False
        if stat.S_ISLNK(mode):
            return False
    return stat.S_ISREG(mode)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=('all', 'changed'))
    parser.add_argument('--base')
    parser.add_argument('--formatter', required=True)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    root = Path(os.fsdecode(git(None, 'rev-parse', '--show-toplevel')).rstrip('\n'))
    if args.mode == 'changed':
        if not args.base:
            parser.error('changed mode requires --base')
        base = os.fsdecode(git(root, 'merge-base', 'HEAD', args.base)).strip()
        staged = git(root, 'diff', '--cached', '--name-only', '-z', '--diff-filter=ACMRT',
                     'HEAD', '--', '*.md')
        working = git(root, 'diff', '--name-only', '-z', '--', '*.md')
        chunks = [git(root, 'diff', '--name-only', '-z', '--diff-filter=ACMRT',
                      base + '...HEAD', '--', '*.md'),
                  staged, working,
                  git(root, 'ls-files', '-z', '--others', '--exclude-standard', '--', '*.md')]
    else:
        chunks = [git(root, 'ls-files', '-z', '--cached', '--others',
                      '--exclude-standard', '--', '*.md')]
    names = sorted({os.fsdecode(name) for chunk in chunks
                    for name in chunk.split(b'\0') if name})
    # Git for Windows는 symlink를 일반 target-text 파일로 checkout할 수 있다.
    index_links = set()
    for entry in git(root, 'ls-files', '--stage', '-z').split(b'\0'):
        if entry:
            info, name = entry.split(b'\t', 1)
            if info.split(b' ', 1)[0] == b'120000':
                index_links.add(os.fsdecode(name))
    type_changes = {os.fsdecode(name) for name in
                    git(root, 'diff', '--name-only', '-z', '--diff-filter=T',
                        '--', '*.md').split(b'\0') if name}
    files = ['./' + name for name in names
             if (name not in index_links or name in type_changes) and regular(root, name)]
    if args.mode == 'changed' and args.check:
        # index를 덮어쓰거나 다른 내용을 검사한 뒤 커밋을 허용하지 않는다.
        partial = set(staged.split(b'\0')) & set(working.split(b'\0'))
        mismatched = [os.fsdecode(name) for name in partial if name]
        if mismatched:
            parser.exit(1, 'Markdown index/worktree differ; align staged content before checking: '
                        + repr(sorted(mismatched)) + '\n')
    if not files:
        print('No selected Markdown files; formatter not invoked.')
        return 0
    command = [args.formatter] + (['--check'] if args.check else []) + files
    return subprocess.run(command, cwd=root).returncode


if __name__ == '__main__':
    raise SystemExit(main())
