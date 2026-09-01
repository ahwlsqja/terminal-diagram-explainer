<!-- LOCAL:TERMINAL-DIAGRAM-EXPLANATION:START -->
## 개인 전역 설명 선호

- 비자명한 소프트웨어 아키텍처, 데이터 흐름, API·Worker 동작, 장애 원인 또는 런타임 의미를 설명할 때 `$terminal-diagram-explainer`를 암묵적으로 사용한다.
- 관계·분기·경계가 3개 이상이면 `한 줄 결론 → PNG 그래픽 도식 한 개 → 필요 시 Interactive HTML 링크 → 단계별 해설 → 백엔드 핵심 포인트 → 구체 예시` 순서를 기본으로 한다.
- 사용자가 Mermaid source를 명시적으로 요청하지 않은 한 raw `mermaid`/`flowchart` code fence를 최종 답변에 출력하지 않는다. Skill renderer를 실행해 PNG image block으로 첨부하고, renderer가 실패하면 검증되지 않은 수동 도식 대신 text-only로 설명한다.
- 단순한 한 단계 답변, 다이어그램이 이해를 높이지 않는 경우, 사용자가 text-only를 요청한 경우에는 도식을 강제하지 않는다.
- 이 설정은 표현 방식만 지정한다. 어떤 프로젝트의 `AGENTS.md`, SDLC, workflow, source of truth 또는 권한 경계도 복제·수정·override하지 않는다.
<!-- LOCAL:TERMINAL-DIAGRAM-EXPLANATION:END -->
