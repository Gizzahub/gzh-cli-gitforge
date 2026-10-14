# GitLab pipeline ref 재실행 경계 수정

실제 GitLab Template 초기화(project542·최초87fbe85) 뒤 서버가 같은 SHA의
`refs/pipelines/26880`, `26881`을 만들었다. 기존 전체 refs 검사는 이 정상 상태의
최초 검증·재개를 거절했다. [GitLab 공식 계약](https://docs.gitlab.com/ci/pipelines/#ref-specs-for-runners)의
서버 pipeline ref를 게시된 최초 상태에서만 좁게 허용한다.

두 필수 master/develop ref와 모든 허용 pipeline ref는 최초 SHA와 같아야 한다.
양수 정수 ID 외 ref, 태그·notes·추가 branch·다른 SHA·중복/잘못된 응답을 거절한다.
최초 빈 원격 검사와 pipeline-only/부분 branch 거절은 유지한다. 불확실한 push 뒤
committed 영수증과 pushed 영수증의 실제 상태가 일치하면 재푸시 없이 재개한다.
CI 성공이나 제품 배포를 이 ref 존재로 주장하지 않는다.

구현·별도 독립 리뷰 요구는 strong이며 실제 런타임은 root·strong_bootstrap·strong_review
모두 gpt-6.1-sol이다. 서로 다른 구현/리뷰 에이전트가 실제 diff를 대조했다.
실제 bare Git atomic push 직후 pipeline 생성·조회 실패·committed/pushed 재개를
검사했고, 독립 허용 목록·bare 회귀 -race exit0과 diffcheck exit0으로 PASS다.
구현자 전체 package -race exit0·coverage86.6%, 정확한 golangci-lint2.14.0
`make lint-check` exit0(0 issues)이다. 처음 PATH shim의 GitHub403과 오래된 primary
lint2.13.1/Go1.26 실행은 성공 근거에서 제외했다. pin·PATH·사용자 routing은 바꾸지 않았다.

root canonical `make quality-check`는 기존 CE parity drift로 exit2다.
`tmp/bootstrap-gitlab-system-refs-quality.log`에서 CLI 전체 exit0(52.458초·68.2%),
bootstrap exit0(11.057초·86.6%), uncached integrate 전체 exit0(553.848초·76.8%)를
확인했다. 기존 source 실패는 ISSUE-41이 소유하며 전체 PASS로 바꾸지 않는다.
추가 integration·E2E·critical coverage는 별도 필수 실행으로 확인한다.

VERSION·version.go는 0.9.4로 맞췄다. 기존 공개 동작의 버그 수정이므로 patch다.
source rebase·CE finish·stable 게시·cask 설치는 실제 영수증과 source refs 확인 후
따로 기록하며 임의 source 바이너리를 workstation에 설치하지 않는다.
