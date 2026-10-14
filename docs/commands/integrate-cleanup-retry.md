# 통합 영수증과 cleanup-only 복구

통합의 push가 끝난 뒤 회수가 중단되면 기존 실행의 영수증을 조회하고
cleanup-only 경로로 남은 자원을 처리한다. readiness, gate, merge와 target
push는 다시 실행하지 않는다. 호출자는 최신 실행 세대와 복구 권한을
확인해야 하며, TaskManager가 그 lifecycle CAS와 durable 실행 기록을 소유한다.

```sh
gz-git integrate receipt --execution-id run-42 --generation g7 --json
gz-git integrate cleanup --execution-id run-42 --generation g7 --json
```

두 명령은 해당 저장소에서 실행한다. 실제 runtime이 발급한 실행 ID와 세대를
사용한다. 예시의 이름을 다른 실행에 재사용하지 않는다.

## 버전과 호환성

새 durable 영수증의 `schema_version`은 **2**다. 저장 위치는 기존 Git common
directory의 `gz-integrate/v1/receipts`를 유지한다. 저장 위치 이름과 문서
스키마는 별개다. run·cleanup의 machine JSON과 worktree 등록 marker는 v1을
유지한다. `receipt --json`은 저장된 영수증의 스키마를 그대로 보여 준다.

v1 영수증은 조회만 지원한다. cleanup, 실행 재사용과 쓰기는 거부하며 현재
설정으로 과거 보호 정보를 보완하거나 자동 변환하지 않는다. 구버전 reader는
새 v2 영수증을 처리하지 못한다. 복구에는 v2를 지원하는 검증된 실행 파일이
필요하다. 이 공개 호환 변경으로 제품 minor 버전은 0.10.0이다.

## 불변 정보와 삭제 조건

영수증 조회 키와 `execution_id`, 저장소 common directory가 일치해야 한다.
실행 세대·소유자, task branch·worktree·OID, target ref·기존 OID, remote·실제
push endpoint, captured provenance와 보호 branch 이름도 불변이다. target
잠금 뒤 재조회에서 바뀐 정보가 있으면 삭제 전에 거부한다.

최초 실행은 상대 filesystem 주소를 실행 디렉터리 기준의 물리 주소로 확정한다.
복구는 저장된 endpoint를 사용하며 현재 디렉터리에서 상대 주소를 재해석하지
않는다. symbolic ref, 이동·잠긴·미등록 worktree, caller 소유 nested/bare
저장소나 sibling lease, 미확인 Git 상태는 보존한다. local ref는 captured
expected-old CAS, remote ref는 정확한 captured OID의 lease로 삭제한다.

## 상태와 실패

| 상태              | 복구 동작                                                                   |
| ----------------- | --------------------------------------------------------------------------- |
| intent            | 실제 remote containment를 증명한다. push 전이면 삭제하지 않는다.            |
| pushed / partial  | 모든 잔여 자원의 정체성을 먼저 확인하고, 남은 자원만 회수한다.              |
| complete          | 현재 부재가 확인되면 무변경 성공한다. 다시 나타난 자원은 재삭제하지 않는다. |
| v1·손상·정보 부족 | 원인을 표시하고 변경 전에 거부한다.                                         |

삭제 명령의 성공이나 저장된 removal flag만으로 완료하지 않는다. 마지막 변경
뒤 worktree·local ref·remote ref의 부재를 새로 조회한다. 조회 오류는 미확인으로
남긴다. cleanup의 exit 0은 완료, 3은 부분 결과, 2는 사전 거부 또는 실행 오류다.

영수증 쓰기는 rename 뒤 directory sync에서도 실패할 수 있다. 오류가 났다고
이전 문서가 그대로 남았다고 가정하지 않는다. 다음 호출은 실제 영수증과 Git을
다시 읽어 push를 반복하지 않는다. 로컬 잠금이 외부 Git writer까지 통제하지는
못하므로 변경 직전에 재확인하고 불확실하면 자원을 보존한다.
