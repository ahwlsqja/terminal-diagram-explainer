---
name: terminal-diagram-explainer
description: 비자명한 소프트웨어 아키텍처·데이터 흐름·API·Worker 호출 순서·상태 전이·장애 원인을 한 줄 결론, bounded 터미널 Flowchart 또는 Sequence Diagram, 단계별 해설로 설명한다. 관계·분기·경계·시간 순서가 여러 개인 개발 설명과 코드 변경의 런타임 의미를 전달할 때 사용하며 단순한 한 단계 답변이나 text-only 요청에는 사용하지 않는다.
---

# Terminal Diagram Explainer

복잡한 소프트웨어 동작을 서버·코어 개발자가 한 번에 파악할 수 있는 설명으로 바꾼다. 이 Skill은 **표현 계층**일 뿐이며 프로젝트 규칙이나 구현 권한을 바꾸지 않는다.

## 경계

- 이 Skill 때문에 프로젝트의 `AGENTS.md`, SDLC, workflow, repo-local Skill 또는 source를 생성·수정하지 않는다.
- 사용자의 별도 구현 요청이 없다면 읽고 설명만 한다.
- repo-local 규칙, security boundary, source of truth가 항상 우선이다.
- secret, credential, PII, 내부 hostname을 다이어그램 label에 노출하지 않는다.
- 관계·분기·경계가 3개 미만이거나 도식이 이해를 높이지 않으면 일반 텍스트로 답한다.

## 사실 확인

현재 구현을 설명할 때는 가장 가까운 `AGENTS.md`와 repo의 SSoT 라우팅을 따른다. 운영 사실은 code/config/workflow/API의 실제 호출 경로로 확인하고, 확인되지 않은 컴포넌트나 데이터 흐름은 사실처럼 그리지 않는다.

## 기본 출력

1. **한 줄 결론**: 무엇이 왜 그렇게 동작하는지 먼저 말한다.
2. **터미널 도식 한 개**: 한 가지 핵심 이야기만 5~12 nodes 또는 최대 6 participants로 표현한다.
3. **읽는 순서**: 도식 label과 연결한 3~7단계 설명을 쓴다.
4. **개발 핵심**: 관련 있는 항목만 고른다.
   - source of truth와 data ownership
   - sync/async boundary와 state transition
   - validation, retry, idempotency, ordering
   - failure path와 observability
   - trust/tenant/security boundary
5. **구체 예시**: 입력 하나가 각 경계를 통과해 어떤 결과가 되는지 보여준다.
6. repo 분석 결과라면 중요한 주장을 실제 file:line에 연결한다.

## 도식 선택

- 처리·데이터 파이프라인: `flowchart LR`
- 분기·의사결정·장애 처리: `flowchart TD`
- API request/response, 서비스 간 호출 순서, fan-out, self-call: `sequenceDiagram`
- 정상 흐름은 `-->`, 실패·비동기·보조 흐름은 `-.->`를 사용한다.
- decision은 `ID{label}`, data store/view는 `ID[(label)]`로 표시한다.
- ownership, service, data, trust boundary가 설명의 핵심이면 `subgraph ID[label] ... end`로 묶는다. Node ID와 subgraph ID는 전체 graph에서 유일해야 한다.
- cycle과 self-loop를 지원한다. Feedback edge는 외곽 route로 그리고 label은 도식 아래 `feedback:` legend에 표시된다.
- 중간 rank를 건너뛰는 edge도 외곽 route를 사용하며 label은 `routed:` legend에 표시될 수 있다.
- cross-subgraph edge는 frame-safe 외곽 route를 사용하며 label은 `routed:` legend에 표시될 수 있다.
- Sequence participant는 먼저 명시 선언하고, request는 `->>`, return은 `-->>`, fan-out은 연속 message로 표시한다.
- 재시도·반복 구간은 `loop label ... end`, 상호 배타적 결과는 `alt label ... else label ... end`, 선택적 호출은 `opt label ... end`로 묶는다.
- 각 fragment branch에는 실제 message를 넣고, frame label은 조건·반복 이유를 짧게 쓴다.
- Sequence는 호출 시간 순서가 핵심일 때만 사용한다. Ownership·분기·데이터 이동이 핵심이면 Flowchart를 유지한다.
- 현재 버전은 class/style/click, Sequence activation/note/`par`, ER 문법을 지원하지 않는다.

문법이 필요하면 [references/grammar.md](references/grammar.md)를 읽는다. 설명 관점과 예시가 필요하면 [references/developer-lenses.md](references/developer-lenses.md)를 읽는다.

## 렌더링

Mermaid subset source를 만든 뒤 이 Skill 디렉터리의 `scripts/render.sh`에 stdin으로 전달한다.

```bash
printf '%s\n' "$diagram_source" | scripts/render.sh
```

- 성공한 renderer 출력을 `text` code fence에 넣는다.
- Mermaid source는 사용자가 재사용을 요청했을 때만 함께 보여준다.
- 실패하면 오류가 가리키는 문법·limit을 줄여 한 번만 재시도한다.
- 두 번째 실패 시 작은 수동 Unicode 도식으로 fallback하고 실패를 한 문장으로 밝힌다.
- renderer는 프로젝트 파일을 만들거나 자동 다운로드·업데이트하지 않는다.
