# 입력·실행 경계

이 도구는 Codex가 생성한 작은 설명용 Flowchart와 Sequence Diagram만 처리합니다. 임의 Mermaid 호환 renderer가 아닙니다.

## 불변식

- 입력은 256 KiB 이하입니다.
- 최대 2,048 lines, 48 nodes, 96 edges, 32 subgraphs, 중첩 깊이 8, label 96 cells입니다.
- Sequence Diagram은 최대 16 participants, 96 messages이며 participant와 message label은 최대 96 cells입니다.
- Structured Sequence fragment는 최대 32개, 중첩 깊이 8이며 전체 timeline은 256 steps입니다.
- Explicit activation은 최대 96개, participant별 LIFO depth 8입니다.
- 기본 canvas는 240×200 cells, hard cap은 512×512 cells이며 clipping 대신 오류를 반환합니다.
- SCC·feedback 분석은 32,768 고정 work-step budget 안에서 종료합니다.
- 직접 구성된 `Graph`도 renderer 진입점에서 parser의 custom `Limits`와 무관한 hard limit 48 nodes, 96 edges, 32 subgraphs, depth 8, endpoint, ID, label, parent forest, membership을 다시 검증합니다.
- 직접 구성된 Sequence `Diagram`도 renderer 진입점에서 participant/message count, ID·display label uniqueness, endpoint, message kind, label을 다시 검증합니다.
- Extended Sequence `Steps`도 renderer가 variant shape, fragment kind·label, branch cardinality, nesting, message count를 stack replay로 다시 검증합니다. `Messages`와 `Steps`는 nil 기준 상호 배타적입니다.
- invalid/unsupported syntax는 line/column이 있는 오류로 fail-closed 처리합니다.
- 입력 검증·파싱·렌더 실패는 stdout에 아무것도 기록하지 않습니다. OS·pipe·writer가 실제 쓰기 도중 실패한 경우에는 이미 전달된 byte를 회수할 수 없으므로 exit code 1과 stderr 진단으로 알립니다.
- label의 terminal control·bidi·format 문자는 렌더 전에 거부합니다.
- 네트워크, HTTP server, shell, subprocess, environment-driven behavior, runtime file write가 없습니다.
- `CGO_ENABLED=0`, `GOPROXY=off`에서 build/test할 수 있으며 `go list -m all`은 자기 모듈 하나만 출력해야 합니다.
- map은 lookup에만 사용하고 배치·출력 순서는 source-order slice로 결정합니다.
- Cycle feedback은 Tarjan SCC membership 안에서 source edge order대로 greedy 분류합니다. Feedback set은 inclusion-minimal이지만 minimum-cardinality라고 주장하지 않습니다.
- Flat feedback route는 LR 아래 gutter와 TD 오른쪽 gutter를 사용합니다. Scoped feedback route는 frame-safe 예약 corridor와 전역 하단 perimeter를 사용하며 label은 최대 96행의 bounded legend로 분리합니다.
- 중간 rank를 건너뛰는 forward edge도 같은 outer-route planner를 사용해 intermediate node 관통을 차단합니다.
- Subgraph는 `Node.Scope` 단일 membership과 source-order parent forest로 표현하며 빈 subtree와 node/subgraph ID 충돌을 거부합니다.
- Scoped layout은 LR y-band, TD x-band로 sibling frame을 분리하고 cross-scope route는 예약 corridor와 검증된 frame portal만 사용합니다.
- Feedback은 `feedback:`, label이 있는 skip-rank forward edge는 `routed:` legend로 분리하며 두 legend 모두 output bounds에 포함합니다.
- Sequence layout은 participant source order와 message time order만 사용합니다. 일반 message는 label/arrow 2-row pitch, self-message는 전용 right corridor를 사용하며 fragment·activation route search는 수행하지 않습니다.
- Message-only Sequence는 0.5 legacy fast path를 그대로 사용합니다. Fragment document만 ordered `Steps` timeline과 depth-inset frame planner를 사용합니다.
- `loop`·`alt/else`·`opt`는 source-order presentation이며 fragment open·branch·end가 각각 전용 control row를 소비합니다. Empty branch와 malformed nesting은 fail-closed 처리합니다.
- `activate`/`deactivate`는 행을 추가하지 않는 serialized-timeline interval입니다. Pair 사이에 message가 필요하며 fragment 시작·branch·end 경계를 넘지 못합니다.
- Activation bar는 호출 stack의 정합성을 증명하지 않습니다. 설명 대상 source에서 실제 active lifetime이 확인될 때만 시각화합니다.
- `par/and` frame은 branch를 source/display order로만 배치합니다. Branch 사이의 세로 위치는 실제 동시 실행 순서나 happens-before 관계를 주장하지 않습니다.
- `par` title에 `display order only`를 항상 표시하고 각 branch를 `and` separator로 구분합니다. Branch는 nonempty이며 activation state를 다음 branch로 넘길 수 없습니다.
- ER Diagram은 최대 32 entities, 64 relationships, attributes 총 192/entity당 32입니다.
- ER parser는 forward relationship endpoint를 EOF에서 explicit entity block으로 resolve하며 public model에 placeholder를 남기지 않습니다.
- ER renderer는 entity/attribute/relationship/cardinality/key bit를 독립 재검증하고 endpoint마다 source-order port와 rail을 사전 배정합니다.
- PK/FK는 표시 metadata이며 FK target이나 referential integrity를 자동 추론하지 않습니다.
- UNIQUE/NOT NULL은 별도 constraint bit로 보존하고 parser와 renderer가 공유 formatter로 정규화합니다. PK나 field 이름에서 constraint를 자동 추론하지 않습니다.
- ER source는 comment와 token separator를 포함해 ASCII space·tab·LF/CRLF 외 Unicode whitespace와 terminal control·format/bidi·ZWJ·variation selector를 parser 진입 시 거부합니다.
- Self·duplicate relationship은 별도 ports/rails를 사용하고 label은 bounded `relationships:` legend에만 표시합니다.
- Long-hop Sequence label row에서는 label 가독성을 위해 중간 lifeline이 일시적으로 끊길 수 있지만, arrow row의 junction과 다음 time row의 lifeline은 보존합니다.
- Canvas는 하나의 bounded flat cell buffer를 사용하며 row별 또는 edge별 full-canvas clone을 만들지 않습니다.

## Upstream audit에서 확인한 제거 대상

평가 대상 `AlexanderGrooff/mermaid-ascii` v1.5.0에서는 다음이 기본 stdin 또는 선택적 web 경로에서 확인됐습니다.

- malformed `classDef`의 index panic
- label ESC byte의 terminal 출력 통과
- LR chain의 edge별 full-canvas 보관으로 인한 quadratic peak memory 증가
- 선택적 web server, external `git` 실행, mutable global state

맞춤 구현은 해당 코드를 전용하지 않고 지원 문법과 실행 표면을 축소해 위 경로를 구조적으로 제거했습니다.

## Provenance

- 기능 요구는 Mermaid의 공개 텍스트 문법 subset과 일반 개발자 설명 요구에서 정의했습니다.
- upstream 구현을 검토한 동일 주체가 작성했으므로 법적 의미의 clean-room이라고 표현하지 않습니다.
- upstream source code, 함수 구조, golden fixture 또는 test corpus를 복사하지 않은 독립 재작성입니다.
