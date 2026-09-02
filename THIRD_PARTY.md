# Third-party provenance and distribution

설계 전 비교·위험 분석 대상으로 다음 MIT 프로젝트를 검토했습니다.

- `AlexanderGrooff/mermaid-ascii`, tag `1.5.0`, commit `b1b35f67d6a5dd0699ccfc968c00a763db573076`

이 저장소는 해당 프로젝트의 source code, fixtures, golden output 또는 dependency graph를 포함하지 않습니다. Mermaid 호환 텍스트의 제한된 기능적 인터페이스만 독립 구현합니다.

Graphical MCP App 설계에서 `getpaseo/paseo`의 Apache-2.0 source를 검토했습니다. Markdown fence를 sandboxed Mermaid runtime과 zoomable viewport로 연결하는 구조·보안 경계만 참고했으며 Paseo source code나 asset을 포함하지 않습니다.

Plugin은 Mermaid와 MCP Apps client runtime을 build artifact에 bundle합니다. 배포되는 package/version/license 원문은 `plugins/terminal-diagram-explainer/mcp/dist/THIRD_PARTY_NOTICES.md`에 build 시 결정적으로 생성됩니다.

## 선택적 개인·사내 Mermaid CLI runtime

저장형 SVG·PNG와 Codex image block은 별도 설치된 `@mermaid-js/mermaid-cli@11.16.0`을 subprocess로 호출할 수 있습니다. Plugin 배포물에는 CLI `node_modules`, Puppeteer, Chromium을 포함하지 않습니다.

- `@mermaid-js/mermaid-cli@11.16.0`: MIT
- `puppeteer@25.9.0`: Apache-2.0
- 실제 resolved dependency와 integrity: `tools/mermaid-cli/package-lock.json`
- immutable release 위치: `$CODEX_HOME/lib/terminal-diagram-explainer/mermaid-cli-runtime/releases`
- 활성 runtime pointer: `$CODEX_HOME/lib/terminal-diagram-explainer/mermaid-cli-runtime/runtime.json`
- browser cache: 각 immutable release의 `.cache/puppeteer`

설치기는 staging release에 exact lockfile을 `--ignore-scripts`로 설치한 뒤 pinned Puppeteer CLI로 `mmdc`가 실제 사용하는 호환 `chrome-headless-shell` 하나를 명시 설치합니다. Smoke render가 통과한 뒤에만 `runtime.json` pointer를 atomic 교체하므로 설치 중 render는 이전 정상 release를 계속 사용합니다. Plugin은 이 runtime이나 browser를 재배포하지 않으며, render 시 외부 icon pack·URL·사용자 CSS·사용자 config를 전달하지 않습니다. 이 runtime을 사내 image나 plugin bundle에 포함해 재배포하려면 lockfile 전체 dependency와 Chromium의 version별 license/SBOM 검토를 별도로 수행해야 합니다.
