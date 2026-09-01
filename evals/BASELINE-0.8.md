# 0.8 Blind Baseline

12개 backend case를 기대 도식과 금지 주장 없이 두 independent agent에게 전달했다.

## Observed Strengths

- Field 이름만 있는 case에서 FK·cardinality를 만들지 않았다.
- `runParallel` 이름 대신 실제 sequential awaits를 따랐고 `par`를 사용하지 않았다.
- `activateFeature` boolean을 activation lifetime으로 오해하지 않았다.
- 단순 pure function과 증거가 부족한 event handler에는 도식을 강제하지 않았다.
- Tenant predicate와 system-wide tenant isolation guarantee를 구분했다.

## Observed Gaps

- 일부 terminal output이 renderer stdout인지 수동 작성인지 평가 산출물만으로 확인할 수 없었다.
- 일부 ER/feedback 표기가 현재 renderer의 canonical legend 형식과 달라 재현성 점수를 확정할 수 없었다.
- Diagram label과 prose claim이 어느 fact에서 왔는지 내부 mapping이 남지 않았다.
- 이름이 강한 의미를 암시하는 adversarial case에서 결과는 정확했지만, 이름 자체는 evidence가 아니라는 공통 규칙이 Skill에 명시돼 있지 않았다.

0.9에서는 renderer stdout verbatim, internal claim ledger, names-are-not-evidence, strong notation evidence gate를 Skill 계약으로 추가한다.

## Post-change forward test

- Adversarial 5-case 재평가에서 FK·parallel·activation·broker·tenant guarantee 오추론은 0건이었다.
- `activateFeature` case의 generic "이후 흐름" node가 제거되고 확인된 true/false branch만 남았다.
- 정상 case에서 cycle legend가 opaque single-letter ID를 노출했고, DDL `REFERENCES`를 ER business verb `has`로 바꾼 사례가 남았다.
- 최종 Skill은 semantic source-derived IDs와 grounded ER relationship labels를 추가 gate로 적용한다.

## Final targeted re-evaluation

- Retry/DLQ cycle legend는 `Backoff`, `Commit`, `Committed`, `Ack` semantic ID를 사용해 opaque ID 문제를 해소했다.
- Schema relationship의 `has`는 재평가 fact packet에 "one customer has zero-many orders"가 직접 포함되어 evidence gate를 통과했다. DDL `REFERENCES`만 제공된 case의 canonical reference source는 계속 `references`를 사용한다.
- Renderer stdout verbatim 적용 과정에서 TD feedback output의 leading blank rows가 드러났고, renderer-level regression으로 수정했다.
- 최초 12-case reference는 Unicode/ASCII 양쪽에서 실제 parser/renderer를 통과했다.
- 0.9 최종 corpus는 positive `par`·activation·nested subgraph, SSoT drift, ordering, redaction 6개를 더해 18개로 확장했다.
- 공개 prompt와 평가자 oracle을 분리하고 실제 agent result를 검증하는 `eval-pack`을 추가했다.
