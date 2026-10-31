// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

/*
Package tasksync prepares and explicitly applies a local-only rebased task candidate.

# Task sync의 clean candidate 준비

이 내부 API는 소유 writer가 고정한 입력으로 재배치 candidate를 준비하는 단계다.
운영 CLI에는 연결하지 않는다. PrepareClean은 원본을 보존하며, 별도 ApplyLocal만
명시적으로 주입한 소유권 lease 아래 task 원본 worktree·branch를 갱신한다. 원격 게시는 없다.
prepared candidate와 실제 task 동기화 완료를 구분한다.

기존 bulk worktree sync는 여러 worktree의 관계와 순서를 조정한다. 이 API는 한 task의
정확한 repository·worktree·branch·HEAD·source OID를 결속하므로 bulk sync를 감싸서
소유권 검증을 대신하지 않는다. 앱의 실행 owner·권한·영수증 검증은 caller의 책임이다.

## 지원 경계

원본에 충돌 상태를 만들기 전에 분리된 소유 scratch에서 clean replay를 시도한다.
결과는 준비된 candidate이며 원본 HEAD가 이동했다는 뜻이 아니다. 충돌이나 지원하지 않는
상태는 명시적으로 거절한다. 임의 파일 내용 선택·stash·fetch·push·trunk 통합을 하지 않는다.

공유 Git 상태에는 scratch worktree 관리 자료와 새 commit object가 생긴다.
반환 candidate는 소유된 private ref로 도달 가능하게 유지하며, caller가 결과를 수용하거나
취소할 때 정확한 ref/OID의 CAS로 해제한다. 다른 ref나 사용자 자료를 대신 정리하지 않는다.

`PrepareClean`의 `PreparedCandidate`는 신규 pin 생성이 확인된 결과다. caller가 소유하는
`CandidateDescriptor`를 보존하면 `Restore`로 저장소·ref·OID를 읽기 전용으로 대조하고
`Release`로 해제할 수 있다. 생성 결과가 불명확한 ref의 관측 자료를 owned descriptor로
승격하거나 해제 권한으로 사용하면 안 된다. 이 descriptor는 공개 wire나 소유권 인증이 아니다.

경로는 실제 canonical worktree root여야 한다. symlink alias, 진행 중 Git operation,
외부 helper를 실행할 수 있는 설정과 지원하지 않는 history는 거부한다. 실행할 Git은
기존 도구 경로에서 해석해 절대 실행 파일로 고정하며 시스템 Git으로 대체하지 않는다.

ApplyLocal은 plan/candidate 해시를 대조하고 writer가 발급한 하나의 lease를 적용 전체에
유지한다. 원래 HEAD를 별도 checkpoint ref에 보존한 뒤 detached checkout, branch CAS,
HEAD 부착을 수행하며 각 변경 전후 owner Record/Check를 요구한다. 중간 실패는 uncertain으로
반환하고 checkpoint·원래 HEAD·관측한 실제 HEAD를 보존한다. 되돌리거나 자동 정리하지 않는다.
lease 종료 실패도 성공으로 처리하지 않는다. Record는 호출자의 durable journal 책임이며,
lease는 기존 caller와 진행 중 작업까지 배제한 실제 권한·generation CAS에 연결해야 한다.
해당 연결과 구버전 caller의 우회 거절을 검증하기 전에는 운영 진입점을 활성화하지 않는다.
충돌 continue/cancel·사용자 편집의 durable 보존·원격 lease 게시도 후속 범위다.

## 실행 파일 연동 (co-process)

RunCoprocess는 숨김 명령 `gz-git task-sync-apply`의 본체다. capability는 정확히
`task-sync-apply-v1`이다. stdin·stdout은 줄 단위 JSON이고 메시지는 schema, type, plan_id,
step, sequence를 가진다. 수신은 strict decoder로 미지 필드·후속 자료·1 MiB 초과 줄을
거절한다. `begin`은 한 프로세스에 한 번이며, 제공자 요청의 sequence는 1부터 증가한다.
`ack`/`deny`는 그 요청의 plan_id·step·sequence를 그대로 되돌려야 한다. step은 기록된 마지막
단계이고 첫 기록 전에는 빈 문자열이다.

`deny`, stdin EOF, 응답 timeout, 복호화·대조 실패, 쓰기 실패는 lease 상실이다. 상실은 되돌릴
수 없고 이후 lease 호출은 입출력 없이 실패하므로 추가 Git 변경이 없다. checkpoint는 보존하고
결과는 uncertain이며, Acquire 단계의 실패만 변경 전이므로 refused다. 재실행 경로는 없다.
`begin` 실패는 종료 2(Git·lease 호출 없음), `result` 전달 실패는 종료 3이다. 그 외 판정은
종료 코드가 아니라 `result`로만 전달한다. 임의 sequence를 앞당긴 응답도 상실로 처리한다.

## CE 진행 단계와 증거

ApplyRecord.Step은 CE `syncStages`와 같은 문자열·같은 순서로 대응한다. 변환표를 두지 않는다.

| 순서 | Step | 직전 또는 직후의 Git 변경 |
|---|---|---|
| 0 | checkpoint-intent | checkpoint ref 생성 전 |
| 1 | checkpoint-created | checkpoint ref 생성 후 |
| 2 | checkout-intent | detached checkout 전 |
| 3 | checked-out | detached checkout 후 |
| 4 | branch-intent | task branch CAS 전 |
| 5 | branch-updated | task branch CAS 후 |
| 6 | attach-intent | HEAD 부착 전 |
| 7 | attached | HEAD 부착 후 |

ActualHEAD는 모든 기록에서 ApplyLocal이 Git으로 관측한 HEAD다. CE의 observeProgress가 이를
자체 관측과 대조한다. EvidenceDigest의 산출 주체는 제공자다. 입력을 직접 보는 쪽이 제공자이고
CE는 형식만 검증한 뒤 영수증에 보존한다. 값은 도메인 태그를 붙인 고정 구조체 JSON의 SHA-256이며,
record는 binding digest·plan·step·checkpoint·candidate·원래/실제 HEAD를, result는 여기에
source·outcome·reason·applied를 더해 결속한다. 위변조 방지 서명이 아니라 대조용 지문이다.

## 수용 조건

clean replay 결과와 source의 관계, 원본 branch/worktree 불변, 입력 drift·dirty·foreign
Git operation·경로/설정 위험의 사전 거절, 충돌 시 원본 보존을 실제 Git fixture로 검증한다.
candidate 보존과 소유 자원의 해제는 원본 task 정리와 구분한다. 결과 불확실성이나
정리 실패를 성공으로 숨기지 않는다.

검증은 저장소의 commit profile과 통합 전 full profile을 따른다. 좁은 fixture 성공은
운영 lifecycle 수용이나 설치·활성화의 증거가 아니다.
*/
package tasksync
