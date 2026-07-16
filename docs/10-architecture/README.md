# 아키텍처 (Architecture)

gzh-cli-gitforge의 상세 아키텍처 문서입니다. 개요와 진입점은 루트
[`ARCHITECTURE.md`](../../ARCHITECTURE.md)를 참고하세요.

## Quick Navigation

| 질문                                           | 이동처                                                                   |
| ---------------------------------------------- | ------------------------------------------------------------------------ |
| 전체 구조와 패키지 구성을 알고 싶다            | [10-overview.md](10-overview.md)                                         |
| 어떤 설계 원칙을 따르는지 알고 싶다            | [20-design-principles.md](20-design-principles.md)                       |
| 각 컴포넌트의 역할과 책임이 궁금하다           | [30-component-design.md](30-component-design.md)                         |
| 인터페이스/API 시그니처를 찾고 있다            | [40-interface-contracts.md](40-interface-contracts.md)                   |
| 데이터 흐름과 에러 처리 방식이 궁금하다        | [50-data-flow-and-error-handling.md](50-data-flow-and-error-handling.md) |
| 테스트 구조와 모킹 전략을 알고 싶다            | [60-testing-architecture.md](60-testing-architecture.md)                 |
| 성능 특성과 보안 모델이 궁금하다               | [70-performance-and-security.md](70-performance-and-security.md)         |
| 빌드·배포 방식을 알고 싶다                     | [80-deployment.md](80-deployment.md)                                     |
| 왜 이렇게 설계했는지, 앞으로의 계획이 궁금하다 | [90-design-decisions.md](90-design-decisions.md)                         |

## 목차

| 문서                                                                     | 원본 섹션 | 내용                                               |
| ------------------------------------------------------------------------ | --------- | -------------------------------------------------- |
| [10-overview.md](10-overview.md)                                         | §2        | Architectural Overview — 레이어 구성, 패키지 맵    |
| [20-design-principles.md](20-design-principles.md)                       | §3        | Design Principles — Library-First, 인터페이스 주도 |
| [30-component-design.md](30-component-design.md)                         | §4        | Component Design — 컴포넌트별 책임                 |
| [40-interface-contracts.md](40-interface-contracts.md)                   | §5        | Interface Contracts — 핵심 인터페이스 시그니처     |
| [50-data-flow-and-error-handling.md](50-data-flow-and-error-handling.md) | §6-7      | Data Flow, Error Handling Strategy                 |
| [60-testing-architecture.md](60-testing-architecture.md)                 | §8        | Testing Architecture — 모킹, 테스트 계층           |
| [70-performance-and-security.md](70-performance-and-security.md)         | §9-10     | Performance Considerations, Security Architecture  |
| [80-deployment.md](80-deployment.md)                                     | §11       | Deployment Architecture — 빌드·배포                |
| [90-design-decisions.md](90-design-decisions.md)                         | §12-13    | Design Decisions, Future Considerations            |

## 관련 문서

- 루트 [`ARCHITECTURE.md`](../../ARCHITECTURE.md) — 개요, Executive Summary, Appendix
- [`REQUIREMENTS.md`](../../REQUIREMENTS.md) — 요구사항
- [`docs/specs/`](../specs/) — 기능별 상세 스펙
