# 입력·실행 경계

Go fallback renderer는 Codex가 생성한 작은 설명용 Flowchart, Sequence, ER, State Diagram subset만 처리합니다. Graphical MCP path는 bundled Mermaid 11을 사용하지만 Skill과 server policy가 software explanation 문법·입력 크기·외부 resource를 별도로 제한합니다.

## 불변식

### Graphical MCP path

- `render_diagram` 입력은 256 KiB, 2,048 lines 이하이며 title은 80자 이하입니다.
- Tool은 read-only·idempotent이고 project file이나 durable state를 변경하지 않습니다.
- Server와 widget은 external image/icon metadata, `click`/remote link, `url()`, `@import`, `themeCSS`, init directive, active/encoded HTML을 각각 독립적으로 거부합니다.
- Terminal control, bidi override와 zero-width format character를 render 전에 거부합니다.
- Mermaid는 `securityLevel=strict`, `htmlLabels=false`, `maxEdges=200`, `maxTextSize=250000`으로 초기화합니다.
- Widget은 `default-src 'none'`, `connect-src 'none'`, `img-src data: blob:`, `frame-src 'none'` CSP를 포함하며 외부 script·font·network resource를 로드하지 않습니다.
- Mermaid가 만든 SVG에서도 `script`, `foreignObject`, `image`, link/event handler와 CSS `url()` attribute를 제거합니다.
- UI resource는 presentation layer입니다. Fact ledger와 source of truth는 Skill/model context에 있고 widget state는 authoritative data가 아닙니다.
- MCP Apps UI가 없는 client에서는 local `term-diagram` binary를 5초·2 MiB 제한으로 실행한 terminal preview, text/structured result와 기존 artifact renderer로 fallback합니다. Renderer stderr와 local path는 tool result에 노출하지 않습니다.

### Terminal fallback path

- 입력은 256 KiB 이하입니다.
- 최대 2,048 lines, 48 nodes, 96 edges, 32 subgraphs, 중첩 깊이 8, label 96 cells입니다.
- Sequence Diagram은 최대 16 participants, 96 messages이며 participant와 message label은 최대 96 cells입니다.
- Structured Sequence fragment는 최대 32개, 중첩 깊이 8이며 전체 timeline은 256 steps입니다.
- Explicit activation은 최대 96개, participant별 LIFO depth 8입니다.
- Standalone 기본 canvas는 240×200 cells, plugin wrapper는 120×200 cells이며 hard cap은 512×512 cells입니다. `-fit` Flow는 요청 방향이 bounds를 넘을 때 반대 방향을 한 번 시도하고 두 방향 모두 실패하면 clipping 대신 오류를 반환합니다.
- SCC·feedback 분석은 32,768 고정 work-step budget 안에서 종료합니다.
- 직접 구성된 `Graph`도 renderer 진입점에서 parser의 custom `Limits`와 무관한 hard limit 48 nodes, 96 edges, 32 subgraphs, depth 8, endpoint, ID, label, parent forest, membership을 다시 검증합니다.
- 직접 구성된 Sequence `Diagram`도 renderer 진입점에서 participant/message count, ID·display label uniqueness, endpoint, message kind, label을 다시 검증합니다.
- Extended Sequence `Steps`도 renderer가 variant shape, fragment kind·label, branch cardinality, nesting, message count를 stack replay로 다시 검증합니다. `Messages`와 `Steps`는 nil 기준 상호 배타적입니다.
- invalid/unsupported syntax는 line/column이 있는 오류로 fail-closed 처리합니다.
- 입력 검증·파싱·렌더 실패는 stdout에 아무것도 기록하지 않습니다. OS·pipe·writer가 실제 쓰기 도중 실패한 경우에는 이미 전달된 byte를 회수할 수 없으므로 exit code 1과 stderr 진단으로 알립니다.
- label의 terminal control·bidi·format 문자는 렌더 전에 거부합니다.
- `term-diagram` binary에는 네트워크, HTTP server, shell, subprocess, environment-driven behavior, runtime file write가 없습니다. Plugin의 MCP server는 bundled static widget만 읽고, 선택적 `render-image.sh`/`render-artifacts.sh`만 임시 SVG/PNG/HTML을 만들고 설치된 `sips`·`rsvg-convert`·ImageMagick 중 하나를 호출하며 다운로드하지 않습니다.
- `CGO_ENABLED=0`, `GOPROXY=off`에서 build/test할 수 있으며 `go list -m all`은 자기 모듈 하나만 출력해야 합니다.
- map은 lookup에만 사용하고 배치·출력 순서는 source-order slice로 결정합니다.
- Cycle feedback은 Tarjan SCC membership 안에서 source edge order대로 greedy 분류합니다. Feedback set은 inclusion-minimal이지만 minimum-cardinality라고 주장하지 않습니다.
- Flat feedback route는 LR 아래 gutter와 TD 오른쪽 gutter를 사용합니다. Scoped route는 endpoint의 최소 공통 조상 frame 안의 예약 corridor를 사용하고 root-scope route만 전역 perimeter를 사용하며 label은 bounded legend로 분리합니다.
- 중간 rank를 건너뛰는 forward edge도 같은 outer-route planner를 사용해 intermediate node 관통을 차단합니다.
- 인접-rank forward edge는 bounded median sweep과 edge별 lane을 사용합니다. 혼합 fan-out/fan-in junction은 고유 endpoint edge를 outer route와 manifest로 승격하며, 동일 endpoint의 parallel forward edge는 silent collapse 대신 실패합니다.
- Canvas line cell은 N/E/S/W 연결 방향을 보존하고 최종 Unicode/ASCII corner·tee·junction glyph를 결정적으로 합성합니다.
- Subgraph는 `Node.Scope` 단일 membership과 source-order parent forest로 표현하며 빈 subtree와 node/subgraph ID 충돌을 거부합니다.
- Scoped layout은 LR y-band, TD x-band로 sibling frame을 분리하고 cross-scope route는 최소 공통 조상 frame 안의 예약 corridor와 검증된 frame portal만 사용합니다.
- Feedback은 `feedback:`, skip-rank·혼합 junction·cross-scope forward edge는 `routed:` manifest로 분리합니다. Routed manifest는 label이 없어도 endpoint를 기록하며 두 section 모두 output bounds에 포함합니다.
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
- Composite table constraint는 attribute index와 FK target entity/attribute index로 보존하고 parser EOF 및 renderer 진입에서 독립 재검증합니다. FK는 relationship/port/cardinality를 만들지 않습니다.
- Table constraints는 64 total, entity당 8, 2~8 columns, 기본 240-cell canvas에 맞춘 236-cell text hard limit을 적용합니다.
- State parser는 32 total ordinary/choice states, choice당 8 branches, 64 transitions, 64 policies, ID 64 bytes, canonical state/transition label과 policy detail 96 cells hard limit 및 exact initial invariant를 적용합니다.
- Choice는 explicit `State.Kind`로만 존재하며 exactly-one ordinary inbound, 2~8 unique guarded ordinary outbound, no pseudo/self/choice-chain/policy invariant를 적용합니다.
- State policy는 별도 statement에서 labeled concrete transition의 endpoint·event·guard를 EOF exact match하며 event/함수/enum 이름을 policy로 승격하지 않습니다.
- Policy target event/guard의 quote는 separator ambiguity를 차단하기 위해 거부하며, policy가 없는 ordinary quoted transition label의 기존 동작은 유지합니다.
- State renderer는 direct state kind·choice topology, endpoint kind/index·pseudo orientation·duplicate semantics와 policy transition index/kind/detail/중복을 재검증합니다. Ordinary concrete transition은 bounded lane, choice outbound는 choice당 bounded shared rail/trunk를 예약합니다.
- State cycle/self feedback은 source-order reachability로 분류하며 declaration order를 cycle 의미로 사용하지 않습니다.
- State policy는 metadata legend일 뿐 state·edge·pseudo-state·cycle 분류나 retry/timeout/compensation 실행 보장을 만들지 않습니다.
- Choice guard는 opaque text이며 renderer는 branch exclusivity, priority, default, exhaustive coverage를 계산하거나 보장하지 않습니다.
- ER source는 comment와 token separator를 포함해 ASCII space·tab·LF/CRLF 외 Unicode whitespace와 terminal control·format/bidi·ZWJ·variation selector를 parser 진입 시 거부합니다.
- Self·duplicate relationship은 별도 ports/rails를 사용하고 label은 bounded `relationships:` legend에만 표시합니다.
- Long-hop Sequence label row에서는 label 가독성을 위해 중간 lifeline이 일시적으로 끊길 수 있지만, arrow row의 junction과 다음 time row의 lifeline은 보존합니다.
- Canvas는 하나의 bounded flat cell buffer를 사용하며 row별 또는 edge별 full-canvas clone을 만들지 않습니다.
- SVG backend는 canonical terminal geometry의 line/arrow를 vector primitive로 바꾸고 text를 XML escape합니다. 최대 60,000 source cells에서 fail-closed하며 host code-fence wrapping이나 box-drawing font metrics에 의존하지 않습니다.
- HTML backend는 같은 escaped SVG를 inline으로 포함하고 고정된 local pan·zoom·fit script만 실행합니다. 외부 script, fetch/XHR/WebSocket, CDN 또는 runtime data source가 없습니다.

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
- Paseo의 Apache-2.0 source에서 Markdown fence→sandboxed Mermaid runtime→zoomable viewer 구조와 보안 경계를 검토했지만 source code를 복사하지 않았습니다. Graphical implementation은 MCP Apps와 Mermaid의 공개 API 위에서 독립 작성했습니다.
- Bundled runtime의 실제 third-party license와 version은 plugin의 `mcp/dist/THIRD_PARTY_NOTICES.md`에 build-time 생성합니다.
