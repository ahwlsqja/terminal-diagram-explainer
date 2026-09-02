# 확장 로드맵

확장은 기능 수보다 결정성·자원 상한·terminal safety를 먼저 고정합니다.

## 1. Cycle — 완료 (0.3.0)

- Tarjan SCC membership과 source-edge-order greedy feedback 분류를 사용합니다.
- LR bottom gutter, TD right gutter, self-loop, cycle+tail, disconnected cycle을 지원합니다.
- 32,768 work-step budget과 48 nodes/96 edges 단일 SCC fixture를 검증합니다.
- Feedback label은 inline이 아니라 bounded `feedback:` legend에 표시합니다.
- Skip-rank와 혼합 fan-out/fan-in forward edge도 outer route를 사용하고 endpoint는 `routed:` manifest에 항상 표시합니다.

## 2. Subgraph — 완료 (0.4.0)

- `Node.Scope` 단일 membership과 source-order parent forest를 사용하고 graph-global node ID를 유지합니다.
- `MaxSubgraphs=32`, `MaxSubgraphDepth=8`, `ScopeRef` representability guard를 적용합니다.
- LR y-band, TD x-band, 중첩 frame, child-only parent를 지원합니다.
- Cross-scope route는 endpoint의 최소 공통 조상 frame 안의 corridor와 방향별 portal을 사용하며, feedback·skip-rank route도 bounded corridor를 사용합니다.
- malformed `end`, duplicate membership, cross-subgraph edge, nested CJK label, long TD inline label을 parser/property/golden test로 고정합니다.

## 3. Sequence Diagram — 완료 (0.5.0)

- 독립 participant/message 모델과 앱 header dispatcher로 Flow 경계를 유지합니다.
- participants 16, messages 96, label 96 cells 상한을 parser와 renderer 양쪽에서 검증합니다.
- request/return, fan-out, self-message, 양방향 arrow, Unicode/ASCII를 지원합니다.
- 96개 일반 message는 2-row pitch로 기본 200행 canvas에 들어가며 self-message 혼합은 exact bounds를 넘으면 fail-closed 처리합니다.
- 이 단계에서는 fragment와 activation을 기본 message model과 분리했고, structured fragment는 다음 0.6 단계에서 추가했습니다.

## 4. Structured Sequence Fragments — 완료 (0.6.0)

- Message-only legacy AST와 ordered `Steps` extended AST를 nil 기준 상호 배타 모드로 유지합니다.
- `loop`, `alt/else`, `opt`와 nested frame을 지원합니다.
- fragments 32, depth 8 hard limit을 parser와 renderer 양쪽에서 검증합니다.
- Empty branch, unmatched/duplicate branch, malformed `end`, invalid direct Step variant를 fail-closed 처리합니다.
- 이 단계에서는 activation과 parallel branch를 분리했고, 각각 다음 0.6.1과 0.7 단계에서 계약을 고정해 추가했습니다.

## 5. Explicit Sequence Activation — 완료 (0.6.1)

- `activate`/`deactivate`를 participant별 LIFO interval로 렌더링합니다.
- 총 activations 96, participant depth 8, timeline steps 256 hard limit을 parser와 renderer 양쪽에서 검증합니다.
- Zero-message, unmatched, unclosed activation을 거부합니다.
- Activation은 fragment 경계를 넘지 못하며 branch 내부에서 완결되어야 합니다.
- Active endpoint·nested bar·self rail은 actual attachment 좌표로 재계산합니다.

## 6. Parallel Branch Presentation — 완료 (0.7.0)

- `par/and`를 최소 두 개의 nonempty lexical branch로 지원합니다.
- Frame title의 `display order only` marker로 branch 세로 순서가 실행 순서나 happens-before가 아님을 명시합니다.
- `BranchKind`를 AST에 보존해 parser와 direct renderer가 `alt→else`, `par→and`를 동일하게 검증합니다.
- Activation은 각 branch 안에서 완결되어야 하며 `and` 경계를 넘지 못합니다.

## 7. ER Diagram — 완료 (0.8.0)

- 독립 `internal/er` parser/model과 exact app dispatcher를 사용합니다.
- entities 32, relationships 64, attributes total 192/entity당 32 hard limit을 parser와 renderer 양쪽에서 검증합니다.
- Four cardinalities, PK/FK bit flags, aliases, forward refs, self·duplicate relation, disconnected component를 지원합니다.
- Component-banded vertical tables, endpoint apron ports, source-order rails와 bounded relationship legend를 사용합니다.
- Relationship attributes, inheritance, weak entity, inferred cardinality는 별도 확장 대상으로 남깁니다.

## 8. Backend Explanation Evaluation Pack — 완료 (0.9.0)

- Auth, outbox, checkout, worker retry, cache, schema, tenant boundary, SSoT, ordering, redaction 등 18개 backend/core case를 고정합니다.
- Field/function 이름만으로 FK, parallel, activation을 추론하는 adversarial case를 포함합니다.
- Agent 입력 `prompts.json`과 평가자 전용 `oracles.json`을 분리하고 fact마다 source ID와 anchor를 둡니다.
- Reference source와 제출 결과는 실제 CLI renderer로 재생하고 diagram kind·element cap·required/prohibited notation·stdout·dimensions를 검증합니다.
- `eval-pack`은 fact ID coverage와 기계 판독 가능한 fail-fast 위반을 결정적으로 거부하고, fact 문장 의미의 정합성과 점수 평가는 rubric에 남깁니다.
- Skill은 internal fact ledger, names-are-not-evidence, strong notation direct-evidence gate, renderer stdout verbatim 계약을 적용합니다.

## 9. Batch Evaluation Runner — 완료 (0.10.0)

- Agent artifact submission과 독립 evaluator review를 별도 strict JSON으로 분리합니다.
- Batch v1은 1~3 runs, run당 고정 18 cases를 정확히 한 번씩 요구하고 최대 54개 결과를 순차 replay합니다.
- Reviewer는 의미 축 95점만 입력하며 renderer reproducibility 5점은 static replay 성공 시 runner가 부여합니다.
- 모든 run에 평균 88, case별 75, Fact/SSoT 평균 27, static·semantic fail-fast 0건을 독립 적용해 반복 평균 희석을 막습니다.
- Corpus·submission SHA-256 binding, 모든 depth의 duplicate JSON key 거부, 입력·result·claim·JSON depth 상한을 적용합니다.
- Report는 입력 순서와 무관한 canonical JSON이며 exact rational score variance, case별 artifact distinct count와 safe failure code만 기록합니다.
- Digest binding은 corpus와 artifact 불일치를 검출하지만 evaluator 신원이나 독립 실행 freshness를 인증하지 않습니다.

## 10. ER Attribute Constraints — 완료 (0.11.0)

- `Key`와 분리된 constraint bitset으로 `UNIQUE`, `NOT NULL`을 표현합니다.
- `PK`, `FK`, `UNIQUE`, `NOT NULL` marker unit의 입력 순서는 자유이며 renderer는 `PK FK UNIQUE NOT NULL type name`으로 정규화합니다.
- Parser와 renderer가 단일 attribute formatter를 공유해 96-cell width와 direct-AST 검증의 drift를 막습니다.
- PK·field 이름·언어 type에서 constraint를 추론하지 않고 explicit DDL·ORM schema evidence만 표시합니다.
- Unknown marker/bit, duplicate·부분 `NOT NULL`, lowercase marker, `DEFAULT`·`CHECK`, source control·bidi를 fail-closed 처리합니다.
- 기존 18-case corpus의 positive schema와 negative name-only case를 확장해 strong-notation evidence gate를 유지합니다.

## 11. Composite Schema Constraints — 완료 (0.12.0)

- SQL-like `PRIMARY KEY`, `UNIQUE`, `FOREIGN KEY ... REFERENCES` table constraints를 attribute·relationship과 분리된 AST로 보존합니다.
- 2~8 ordered columns, entity당 8/전체 64 constraints, 기본 240-cell canvas에 맞춘 236-cell canonical row hard limit을 적용합니다.
- Local attributes와 self/forward target entity·attributes를 EOF에서 index로 resolve하고 direct AST도 동일 불변식을 재검증합니다.
- Attributes 다음에 table constraints를 source order로 렌더하고 두 section 사이에 divider를 둡니다.
- Composite FK는 ordered mapping만 표시하며 relationship·cardinality·attribute marker를 추론하지 않습니다.
- 기존 18-case positive schema와 negative name-only case를 확장해 ordered-column evidence gate를 검증합니다.

## 12. State Diagram Core — 완료 (0.13.0)

- 독립 `stateDiagram-v2` parser/model과 exact app dispatcher를 추가합니다.
- Explicit state/alias, TD·LR direction, initial/final pseudo-state, event와 trailing guard를 지원합니다.
- Exactly one initial, optional multiple finals, 32 states/64 transitions, 96-cell label hard limit을 parser와 renderer 양쪽에서 검증합니다.
- State-to-state connector를 bounded gutter로 렌더하고 source-order greedy reachability로 cycle/self feedback을 분류합니다.
- Forward endpoint는 EOF에서 resolve하며 undeclared state와 암묵 state 생성을 거부합니다.
- Enum·함수·status 이름에서 transition/initial/final/guard를 추론하지 않고 fixed-18 corpus에 positive lifecycle과 negative evidence-gap을 고정합니다.
- Composite state, fork/join/history와 transition policy의 구조적 실행 의미는 다음 확장으로 남깁니다.

## 13. Explicit Transition Policy — 완료 (0.14.0)

- 기존 event/guard label을 재해석하지 않는 별도 `policy <exact transition> :: kind "detail"` statement를 추가합니다.
- `retry`, `timeout`, `compensation` 세 kind를 지원하고 policy-before-transition을 EOF에서 endpoint·event·guard exact match로 해소합니다.
- Policy는 transition index metadata로만 보존하며 state·edge·pseudo-state·cycle 분류를 만들거나 바꾸지 않습니다.
- 동일 transition의 동일 kind 중복은 거부하고, 최대 64 policies와 96-cell detail hard limit을 parser와 renderer 양쪽에서 검증합니다.
- Event·함수·enum 이름을 policy로 승격하지 않으며 fixed-18 corpus에 positive retry policy와 negative identifier-only case를 고정합니다.
- Composite/fork/join/history, timer clock origin, retry scheduler, compensation 성공·원자성·idempotency 보장은 별도 확장으로 남깁니다.

## 14. Explicit Choice State — 완료 (0.15.0)

- Mermaid-compatible `state ID <<choice>>`와 alias declaration을 `State.Kind` metadata로 보존합니다.
- Choice마다 ordinary state에서 정확히 1 inbound, 서로 다른 ordinary target으로 2~8 guarded outbound를 요구합니다.
- Guard-only transition은 choice outbound에만 허용하고 guard를 ASCII trim canonical form으로 보존하며 exact duplicate를 거부합니다.
- Choice-to-choice/self/pseudo 연결과 choice incident policy를 parser와 direct-AST renderer 양쪽에서 fail-closed 처리합니다.
- 기존 dense state index와 source-order reachability feedback을 유지하고, choice는 bounded diamond와 inbound 전용 port·choice당 shared fan-out rail/trunk로 렌더링합니다.
- Choice 이름·다중 outbound·guard만으로 decision point를 추론하지 않으며 fixed-18 corpus에 positive explicit choice와 negative identifier-only case를 고정합니다.
- Guard의 의미적 상호배타성·우선순위·default·완전성, fork/join/history/composite는 별도 확장으로 남깁니다.

## 15. Topology-safe Flow와 Viewport Auto-fit — 완료 (0.16.0)

- 인접 rank node order에 source-order stable median sweep을 적용해 풀 수 있는 crossing을 먼저 줄입니다.
- Rank gap을 forward edge 수에 맞춰 확장하고 edge별 lane을 예약합니다.
- Forward route cell component가 여러 source와 여러 target을 동시에 연결할 위험이 있으면 고유 endpoint edge를 outer route와 bounded manifest로 승격하고, 승격할 수 없는 parallel edge는 fail-closed 처리합니다.
- 동일 endpoint의 parallel forward edge는 silent collapse를 거부하고 feedback/outer route로 분리된 edge는 기존 별도 route를 유지합니다.
- Canvas는 N/E/S/W 연결 mask로 corner·tee·junction을 합성해 elbow와 실제 교차를 구분합니다.
- CLI `-width`, `-height`, `-fit`을 추가하고 plugin wrapper는 120×200 viewport에서 반대 방향 auto-fit을 사용합니다.
- Journey 218-cell LR fixture, crossed adjacency, parallel label, directional elbow를 회귀 테스트로 고정합니다.
- 무라벨 skip-rank·혼합 junction도 semantic endpoint를 `routed:` manifest에 기록하고, scoped route는 endpoint의 최소 공통 조상 frame을 벗어나지 않습니다.

## 16. SVG Image Backend — 완료 (0.17.0)

- Canonical terminal geometry의 line glyph를 연속 SVG path, arrow를 polygon, label을 XML-escaped text로 변환합니다.
- SVG viewBox는 terminal cell dimensions에서 결정하며 source cell budget 60,000을 넘으면 fail-closed합니다.
- CLI `-format text|svg`를 제공하고 기존 text stdout 계약은 기본값으로 유지합니다.
- Plugin은 120×200 auto-fit SVG를 로컬 PNG로 변환해 image attachment로 사용하고, image surface가 없을 때만 terminal text로 fallback합니다.
- 두 번째 renderer 실패에서 수동 Unicode 그림을 생성하지 않아 검증되지 않은 geometry가 canonical output으로 섞이지 않게 합니다.

## 17. Interactive HTML Viewer — 완료 (0.18.0)

- CLI `-format html`이 검증된 SVG geometry를 self-contained viewer에 내장합니다.
- Viewer는 keyboard-accessible zoom in/out, fit, 100% controls와 pointer pan·wheel zoom을 제공합니다.
- ResizeObserver가 viewport 변화에 맞춰 fit 상태를 유지하며 외부 script·network·runtime data fetch를 사용하지 않습니다.
- Plugin artifact script는 PNG와 HTML을 같은 source에서 생성해 inline preview와 interactive inspection 선택지를 함께 제공합니다.

## 18. Mermaid MCP App — 완료 (0.19.0)

- Plugin-scoped local stdio MCP server가 read-only `render_diagram` tool과 `ui://terminal-diagram-explainer/viewer-v1.html` resource를 제공합니다.
- Bundled Mermaid가 terminal cell geometry를 거치지 않고 semantic SVG를 생성하며 source 확인, pan, zoom, fit, 100% view를 제공합니다.
- UI는 MCP Apps bridge와 OpenAI compatibility input을 지원하고 UI 미지원 client에는 text/structured result와 기존 artifact fallback을 유지합니다.
- Server/widget 이중 source policy, CSP, SVG post-sanitization, input/resource bounds와 deterministic bundle test를 적용합니다.
- Runtime package download, CDN, Playwright, Chromium과 remote Mermaid service를 사용하지 않습니다.
- 실제 Flow·Sequence·ER·State 215개를 1024-dark·736-light·360-dark에서 645회 렌더하고, narrow fit 50% 미만 106개는 100% zoom·pan·selection-collapse screenshot까지 재생하는 corpus를 repo에 고정합니다.

## 19. TUI-visible fallback — 완료 (0.19.1)

- `render_diagram` 성공 결과가 bounded terminal preview와 `/app` 안내를 함께 반환해 MCP Apps iframe이 없는 Codex CLI/TUI에서도 결과가 보입니다.
- Skill은 tool call 성공과 UI mount 성공을 구분하며, TUI에서는 `terminalFallback`을 최종 답변에 반복하고 inline UI가 보인다고 주장하지 않습니다.
- Desktop App의 interactive UI, UI resource link와 기존 artifact fallback은 유지합니다.

## 20. Official Mermaid artifact fallback — 완료 (0.20.0)

- Codex CLI/TUI tool result에 `@mermaid-js/mermaid-cli@11.16.0`이 만든 PNG image block을 반환합니다.
- 저장형 SVG·PNG는 Mermaid semantic layout을 사용하고, HTML은 bundled Mermaid standalone viewer를 사용합니다.
- `mmdc`와 Puppeteer는 exact lockfile로 `$CODEX_HOME`에 개인 설치하며 plugin·repository에는 browser나 `node_modules`를 재배포하지 않습니다.
- `mmdc`가 없거나 실패할 때만 기존 bounded Go renderer로 내려갑니다.

## 후속 시각 품질 과제

- Dense scoped LR의 outer corridor는 endpoint manifest로 topology를 보존하지만 polyline이 길고 subgraph frame과 같은 stroke를 사용해 scan cost가 큽니다. LCA 내부 compact route search 또는 SVG semantic styling을 별도 개선으로 다룹니다.

## 공통 완료 게이트

- parser/renderer fuzz, race, vet, offline build
- 동일 입력 256회 byte-identical output
- canvas 512×512 hard cap과 clipping 없는 bounds error
- 최대 fixture 2,500 allocations/run 이하
- 입력 4배 증가 시 allocations 4배 이하
- ASCII drawing glyph-only, label codepoint 보존, trailing whitespace 없음
