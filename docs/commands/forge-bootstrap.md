# `gz-git forge bootstrap`

GitLab의 비어 있는 프로젝트에 템플릿의 특정 커밋을 복사해 독립 저장소를 만든다.
템플릿 Git 이력은 복사하지 않는다. 기본 브랜치는 `master`, 통합 브랜치는
`develop`이며 두 브랜치는 같은 최초 커밋을 가리킨다.

## 계획과 적용

요청 JSON에는 다음 정보를 넣는다. 인증 토큰은 JSON에 넣지 않고 실행 환경의
`GITLAB_TOKEN`으로 제공한다. 아래 도메인과 경로는 운영자가 실제 값으로 바꾼다.

```json
{
  "namespace": "service-hub/singlepageweb",
  "project": "example-site",
  "remote": "ssh://git@gitlab.example:2224/service-hub/singlepageweb/example-site.git",
  "checkout": "/absolute/path/example-site",
  "default_branch": "master",
  "integration_branch": "develop",
  "template_url": "https://gitlab.example/service-hub/singlepageweb/singlepageweb-template.git",
  "template_sha": "0123456789abcdef0123456789abcdef01234567",
  "repo_id": "example-site",
  "worker_name": "example-site",
  "site": {
    "siteId": "example-site",
    "domain": "https://example.invalid",
    "displayName": "Example site",
    "description": "서비스 설명",
    "supportedLocales": ["en", "ko"],
    "defaultLocale": "ko",
    "indexingEnabled": false,
    "analyticsEnabled": false,
    "theme": {
      "light": { "accent": "#2563eb", "accentContrast": "#ffffff", "surface": "#ffffff", "text": "#172033" },
      "dark": { "accent": "#7dd3fc", "accentContrast": "#082f49", "surface": "#0f172a", "text": "#e2e8f0" }
    }
  }
}
```

예시 색상은 공통 브랜드 규칙이 아니다. 대상 제품에서 승인한 밝은·어두운 테마를 사용한다.

`site`는 해당 제품의 전체 사이트 설정이어야 한다. 도구는 이를
`site.config.json`으로 기록하지만 제품의 설정 스키마나 UI를 검증하지는 않는다.
일반 사이트 준비에서는 `repo_id`와 `worker_name`을 생략하면 `project` 값을 사용한다. 템플릿의
`.ce/task-runtime.yaml`에 있는 `repository-id: singlepageweb-template`와
`wrangler.jsonc`의 `name` 속성에 있는 `replace-with-worker-name`을 각각 대체한다.
현재 템플릿 계약은 단일 `repository-id` 줄과 주석 없는 표준 JSON 객체 형태의
Worker 설정을 지원한다. 이미 적용된 값도 속성의 전체 값이 정확히 같아야 한다.

```sh
gz-git forge bootstrap plan --input /absolute/path/request.json --output /absolute/path/plan.json
gz-git forge bootstrap apply --plan /absolute/path/plan.json --confirm 'sha256:실제-계획-다이제스트' --commit-model 'gpt-6.1-sol'
```

계획은 GitLab 프로젝트의 존재 여부와 ID, 비어 있는 상태를 관찰하고 요청과 함께
다이제스트에 묶는다. 적용 전에 계획과 실제 프로젝트 ID를 다시 확인한다. 기존
프로젝트에 내용이 있거나 체크아웃 경로가 점유되어 있으면 적용을 거부한다.
`--confirm`에는 계획 단계가 출력한 `PLAN_DIGEST`의 실제 값을 넣는다.
`--commit-model`에는 최초 커밋을 실행하는 실제 런타임의 버전이 포함된 모델
식별자를 넣는다. 문서의 예시를 다른 런타임에서 그대로 사용하지 않는다.

`git_lab_url`을 생략하면 원격 호스트에서 HTTPS API 원점을 결정하고 계획에
저장한다. HTTPS 원격의 포트는 유지하고 SSH 원격의 포트는 API 포트로 사용하지
않는다. 예를 들어 `ssh://git@gitlab.example:2224/...`의 API는
`https://gitlab.example`이다. API 주소가 다르면 `git_lab_url`에 승인된 HTTPS
원점을 명시한다. 인증 토큰을 `gitlab.com` 같은 암묵적 기본 호스트로 보내지
않는다. API 원점에는 경로, 사용자 정보, 쿼리와 프래그먼트를 허용하지 않는다.
Git 전송 인증은 Git 자체의 인증 설정을 사용하며 API 토큰을 Git URL에 삽입하지
않는다.

## 최초 템플릿 준비

아직 템플릿 저장소가 없는 최초 준비에는 `--seed-dir`로 검증된 로컬 소스 트리를
지정할 수 있다. 브랜드와 운영 설정이 없는 최초 템플릿에는 `site`를 생략하거나 빈 객체로 둔다.
이 경우 실제 루트 `site.config.json`을 만들지 않고 Worker 이름 자리표시자를 유지한다.
CE 저장소 ID와 시드 출처는 기록한다. 시드 자체에도 실제 루트 `site.config.json`이 없어야 한다.
일반 템플릿 URL·SHA 방식의 새 사이트 준비에는 비어 있지 않은 사이트 설정이 필요하다.
요청의 `template_url`과 `template_sha`를 비워 두고 최초
템플릿 프로젝트의 이름·설정·목적지 경로를 명시한다.

```sh
gz-git forge bootstrap plan --input /absolute/path/template-request.json --seed-dir /absolute/path/prepared-template --output /absolute/path/template-plan.json
```

이 경로는 기존 저장소의 CE 실행을 우회하는 일반 작업 경로가 아니다. 도구가
새 저장소의 최초 커밋과 두 브랜치를 만드는 초기 준비에만 사용한다. 이후의
소스 변경과 통합은 만들어진 저장소의 CE 선언과 실행 절차를 따른다.

시드의 내용과 실행 권한 비트는 계획 다이제스트에 묶인다. 적용 시 변경을 다시
검사하며 시드와 체크아웃 경로는 겹칠 수 없다. `.git`, `node_modules`, `dist`,
`.astro`, 테스트 결과, `.env`, `.npmrc` 같은 금지 항목과 심볼릭 링크를 거부한다.
공식 GitLab CLI는 `file://` 원격을 지원하지 않는다. 로컬 bare 저장소는 도구의
단위 테스트에서만 별도의 가짜 원격 어댑터와 함께 사용한다.

## 기록과 실패 후 재개

체크아웃 옆의 `<checkout>.bootstrap-receipt.json`에 프로젝트 ID, 계획
다이제스트, 진행 단계와 최초 커밋 SHA를 보관한다. 저장소의
`.bootstrap-provenance.json`에는 템플릿 URL·SHA 또는 시드 내용 다이제스트와
사이트 설정 다이제스트를 기록한다. 최초 커밋 SHA는 자기 참조가 생기지 않도록
커밋 안에 쓰지 않고 외부 영수증과 최종 결과에 기록한다. 검증된 전체 소스와 생성한 설정·출처 파일은
전역 또는 저장소 Git ignore 규칙 때문에 빠지지 않도록 최초 커밋에 포함한다.

설정 전의 파일·디렉터리 다이제스트와 각 설정 파일 교체 단계도 영수증에 기록한다.
각 쓰기 전에 변경 여부와 심볼릭 링크를 검사하고, 저장소 루트를 벗어나지 않는
임시 파일의 원자적 교체로 기존 하드 링크의 외부 대상을 보존한다.
다이제스트 비교와 파일 교체는 전체 디렉터리를 잠그는 트랜잭션이 아니다.
적용 중 외부 프로세스의 동시 수정은 지원하지 않는다.

원격 준비, 복사, 설정, 커밋과 푸시 단계가 실제 상태와 일치하는지 확인한 뒤
재개한다. 커밋을 준비한 파일이나 기존 커밋·원격 주소·원격 브랜치가 바뀌면
자동으로 덮어쓰지 않는다. 두 브랜치는 `--atomic`으로 함께 푸시하고 실제 원격
응답에서 두 SHA가 최초 커밋과 같고 태그·노트 같은 다른 ref가 없는지 확인한 뒤에만 `pushed`로 기록한다.

프로젝트 생성 전에 `provisioning` 단계를 기록한다. 생성 요청의 결과가
불확실한 경우 같은 요청을 자동 재시도하거나 프로젝트를 삭제하지 않는다.
운영자가 GitLab과 영수증을 확인하고 현재 프로젝트 ID를 관찰한 새 계획으로
다음 조치를 결정한다. 영수증을 임의로 삭제하거나 단계를 수정해 재시도하지
않는다. API 클라이언트는 자동 재시도와 다른 호스트로의 리다이렉트를 사용하지
않는다. 실패한 체크아웃과 영수증은 조사할 수 있도록 보존한다.

계획·출처·영수증에는 인증 토큰이나 비밀 설정을 넣지 않는다. 자격 증명이 있는
URL과 비밀 필드를 거부하고 API 응답 본문을 오류에 노출하지 않는다. 이 명령은
사이트 배포, 디자인 재게시, 인프라 등록 또는 기존 자식 저장소로의 변경 전파를
자동 수행하지 않는다. 생성 후에는 해당 제품의 검사·CE 실행·배포 계약으로
검증하고 전달한다.
