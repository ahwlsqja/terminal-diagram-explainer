# Third-party provenance and distribution

설계 전 비교·위험 분석 대상으로 다음 MIT 프로젝트를 검토했습니다.

- `AlexanderGrooff/mermaid-ascii`, tag `1.5.0`, commit `b1b35f67d6a5dd0699ccfc968c00a763db573076`

이 저장소는 해당 프로젝트의 source code, fixtures, golden output 또는 dependency graph를 포함하지 않습니다. Mermaid 호환 텍스트의 제한된 기능적 인터페이스만 독립 구현합니다.

Graphical MCP App 설계에서 `getpaseo/paseo`의 Apache-2.0 source를 검토했습니다. Markdown fence를 sandboxed Mermaid runtime과 zoomable viewport로 연결하는 구조·보안 경계만 참고했으며 Paseo source code나 asset을 포함하지 않습니다.

Plugin은 Mermaid와 MCP Apps client runtime을 build artifact에 bundle합니다. 배포되는 package/version/license 원문은 `plugins/terminal-diagram-explainer/mcp/dist/THIRD_PARTY_NOTICES.md`에 build 시 결정적으로 생성됩니다.
