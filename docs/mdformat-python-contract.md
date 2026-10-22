# mdformat Python 실행 계약

ISSUE-352의 기준 구현이다. 구현과 별도 독립 리뷰 요구 등급은 strong이다.
도구 묶음과 constraints의 버전은 변경하지 않는다.

## 실행 환경과 선택

`install-mdformat`은 재사용할 venv와 설치 후 venv 모두에서 실제 인터프리터가
Python 3.11 이상인지 검사한다. 정상적인 더 최신 venv는 재설치하지 않는다.
정확한 버전·플러그인·하위 의존성 검사도 그대로 적용하며 버전 명령 자체의
실패를 파서 성공으로 바꾸지 않는다.

설치가 필요할 때 `UV_PYTHON`이 설정되지 않았으면 uv에 `>=3.11`을 요청한다.
명시된 `UV_PYTHON`은 빈 값도 그대로 전달한다. 하한 미달 인터프리터로 설치되거나
설치 후 인터프리터가 실행되지 않으면 target은 실패한다. 명시적 요청이 있어도
이미 정상인 venv를 요청 버전으로 바꾸는 동작은 추가하지 않았다.

이 하한은 Python 3.11로 고정하거나 더 최신 런타임을 낮추는 선언이 아니다.
프로젝트 Python pin 파일도 추가하지 않는다. uv의 다운로드·offline 정책은
기존 환경 설정을 따른다. 워크스테이션 도구 pin과 전역 Python 패키지는 변경하지 않는다.

uv 도구 설치는 프로젝트의 `.python-version`과 `requires-python`을 기본적으로
따르지 않는다. 별도 선택이 필요하면 uv가 지원하는 `UV_PYTHON`으로 전달한다.
[uv 도구 Python 선택 문서](https://docs.astral.sh/uv/concepts/tools/#python-versions)를 따른다.

## 검증 범위

`make mdformat-install-test`는 먼저 실제 도구 환경을 준비하고 그 환경의 Python으로
검사한다. PATH의 별도 `python3`는 요구하지 않는다. 준비 단계에는 uv 다운로드가
필요할 수 있다. 이후 fixture는 네트워크 없이 실제 Make 설치 recipe를 격리된
모의 uv·인터프리터로 실행한다. 버전 판정 코드는 실제 Python에서
`sys.version_info`를 바꿔 평가한다.
하한 미달 재설치, 정상·더 최신 venv 재사용, 기본·명시 요청 전달, 설치 실패,
설치 후 잘못된 런타임·누락 인터프리터, 버전 producer 실패와 의존성 불일치를 검사한다.
Windows 경로 모양 검사는 POSIX fixture이며 실제 Windows 검증을 대신하지 않는다.

이 회귀 target은 기존 canonical `quality-check`의 선행 조건이다. 기존 검사와
프로파일·worker·timeout은 유지한다. `.make` 변경은 commit 선택기에서 full로
확대되며 `verify-full`을 요구한다. 모의 검사 통과를 native 설치·전체 게이트·
소비자 전파나 릴리스 성공으로 보고하지 않는다. 소비자 변경과 문서 백필은 별도다.
