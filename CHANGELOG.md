# Changelog

## [0.22.2](https://github.com/ctx-hq/ctx/compare/v0.22.1...v0.22.2) (2026-03-31)


### Bug Fixes

* **mcp:** improve post-install env var guidance and stderr output ([eb8d8e4](https://github.com/ctx-hq/ctx/commit/eb8d8e47abf43e3e4302f4c1d13371399ccabab9))

## [0.22.1](https://github.com/ctx-hq/ctx/compare/v0.22.0...v0.22.1) (2026-03-31)


### Bug Fixes

* **upgrade:** show friendly message instead of error when version check fails ([00f9e3b](https://github.com/ctx-hq/ctx/commit/00f9e3b81b8b1cdcad56149ec2f11999a7ad3b5c))

## [0.22.0](https://github.com/ctx-hq/ctx/compare/v0.21.1...v0.22.0) (2026-03-31)


### Features

* add MCP client, protocol layer, secrets management and agent-native enhancements ([5642375](https://github.com/ctx-hq/ctx/commit/5642375a785d76334fe067d3d781645b1d200a59))
* **tui:** rewrite TUI as dual-pane browser with mode-switching architecture ([3a580f8](https://github.com/ctx-hq/ctx/commit/3a580f869d07542c71d60200770647c12b9a421e))

## [0.21.1](https://github.com/ctx-hq/ctx/compare/v0.21.0...v0.21.1) (2026-03-31)


### Bug Fixes

* **build:** replace local bubbles/v2 path with GitHub module reference ([f1ca16f](https://github.com/ctx-hq/ctx/commit/f1ca16f6675e88cdb5ecfda7c57064a0ca1b9c56))

## [0.21.0](https://github.com/ctx-hq/ctx/compare/v0.20.0...v0.21.0) (2026-03-31)


### Features

* add interactive TUI and expand MCP server toolset ([ee6ae8f](https://github.com/ctx-hq/ctx/commit/ee6ae8f28143978660aa4ed1346f178a015dc276))

## [0.20.0](https://github.com/ctx-hq/ctx/compare/v0.19.2...v0.20.0) (2026-03-30)


### Features

* **output:** add --verbose global flag with context-based diagnostic logging ([ca0a38a](https://github.com/ctx-hq/ctx/commit/ca0a38a30c717f9ce2ef16fe20c4ea7162cd7c62))

## [0.19.2](https://github.com/ctx-hq/ctx/compare/v0.19.1...v0.19.2) (2026-03-30)


### Bug Fixes

* **install:** do not skip skill linking when CLI script is declined or fails ([64f2151](https://github.com/ctx-hq/ctx/commit/64f2151dc5e8a4c8528976b86907ca17e9da4c2f))

## [0.19.1](https://github.com/ctx-hq/ctx/compare/v0.19.0...v0.19.1) (2026-03-30)


### Bug Fixes

* **installer:** support YAML manifest parsing from registry ([f67cf97](https://github.com/ctx-hq/ctx/commit/f67cf9708205bb9a9bf3c44752faf32588160941))

## [0.19.0](https://github.com/ctx-hq/ctx/compare/v0.18.0...v0.19.0) (2026-03-30)


### Features

* **unpublish:** add unpublish command to permanently delete packages or versions ([5ffcd1c](https://github.com/ctx-hq/ctx/commit/5ffcd1c411cf97634a94f6ed4740106e598ef1d8))

## [0.18.0](https://github.com/ctx-hq/ctx/compare/v0.17.2...v0.18.0) (2026-03-30)


### Features

* **publish:** switch to whitelist packaging to exclude source code ([63a5273](https://github.com/ctx-hq/ctx/commit/63a527342614f4a9700c84c31a8fbefe4ada7aaa))

## [0.17.2](https://github.com/ctx-hq/ctx/compare/v0.17.1...v0.17.2) (2026-03-30)


### Bug Fixes

* **init:** fix post-init breadcrumb push path and align docs with skill requirement ([7df4cf3](https://github.com/ctx-hq/ctx/commit/7df4cf3ed3a74128c9b450ab1c0c32cd45988daf))

## [0.17.1](https://github.com/ctx-hq/ctx/compare/v0.17.0...v0.17.1) (2026-03-30)


### Bug Fixes

* **ci:** trigger release build after release-please creates tag ([c1591ff](https://github.com/ctx-hq/ctx/commit/c1591ff7f6bc78c40aa7cc7c3199c55aee1fc8f8))
* **init:** preserve existing SKILL.md path and content during directory init ([fb026a7](https://github.com/ctx-hq/ctx/commit/fb026a73975923c0cc23930a05dd96aac6c8bd70))

## [0.17.0](https://github.com/ctx-hq/ctx/compare/v0.16.0...v0.17.0) (2026-03-30)


### Features

* **cli:** add CLI/MCP package init, install state tracking, and publish validation ([7ec5ecb](https://github.com/ctx-hq/ctx/commit/7ec5ecbac729fdc3ff1047614baf00dffde1e675))

## [0.16.0](https://github.com/ctx-hq/ctx/compare/v0.15.0...v0.16.0) (2026-03-30)


### Features

* **cli:** add wrap command and script install adapter for packaging CLI tools as ctx skills ([8e78a60](https://github.com/ctx-hq/ctx/commit/8e78a60e5ebad3906fa68cb7becf6de338d88e51))

## [0.15.0](https://github.com/ctx-hq/ctx/compare/v0.14.0...v0.15.0) (2026-03-30)


### Features

* **cli:** add package transfer, rename, notifications, and org lifecycle commands ([2b6bb5e](https://github.com/ctx-hq/ctx/commit/2b6bb5ef66034196337b70586b884583a5c8624b))
* **cli:** add wrap command and script install adapter for packaging CLI tools as ctx skills ([8e78a60](https://github.com/ctx-hq/ctx/commit/8e78a60e5ebad3906fa68cb7becf6de338d88e51))

## [0.15.0](https://github.com/ctx-hq/ctx/compare/v0.14.0...v0.15.0) (2026-03-30)


### Features

* **cli:** add package transfer, rename, notifications, and org lifecycle commands ([8da372b](https://github.com/ctx-hq/ctx/commit/8da372b))

## [0.14.0](https://github.com/ctx-hq/ctx/compare/v0.13.0...v0.14.0) (2026-03-30)


### Features

* **cli:** add org invitation management and package access control commands ([13e3e06](https://github.com/ctx-hq/ctx/commit/13e3e064db76ebab92330104797490d89d215e9c))

## [0.13.0](https://github.com/ctx-hq/ctx/compare/v0.12.1...v0.13.0) (2026-03-30)


### Features

* **cli:** add logout command and fix silent keychain error swallowing ([dbf9e1e](https://github.com/ctx-hq/ctx/commit/dbf9e1e))

## [0.12.1](https://github.com/ctx-hq/ctx/compare/v0.12.0...v0.12.1) (2026-03-30)


### Bug Fixes

* **registry:** handle SQLite datetime and integer boolean formats from API ([2e87d94](https://github.com/ctx-hq/ctx/commit/2e87d94))

## [0.12.0](https://github.com/ctx-hq/ctx/compare/v0.11.0...v0.12.0) (2026-03-30)


### Features

* **cli:** overhaul ctx push with nil archive fix, state tracking, and batch support ([5b25d26](https://github.com/ctx-hq/ctx/commit/5b25d26))


### Bug Fixes

* **cli:** filter staging excludes, fail batch on errors, fix dry-run side effects and status double output ([0780939](https://github.com/ctx-hq/ctx/commit/0780939))

## [0.11.0](https://github.com/ctx-hq/ctx/compare/v0.10.1...v0.11.0) (2026-03-29)


### Refactoring

* **cli:** rework ctx init to enforce SSOT at ~/.ctx/skills/ ([967cf3b](https://github.com/ctx-hq/ctx/commit/967cf3b))

## [0.10.1](https://github.com/ctx-hq/ctx/compare/v0.10.0...v0.10.1) (2026-03-29)


### Tests

* **registry:** add regression tests for API path, method, and body ([9774203](https://github.com/ctx-hq/ctx/commit/9774203))

## [0.10.0](https://github.com/ctx-hq/ctx/compare/v0.9.0...v0.10.0) (2026-03-29)


### Features

* **cli:** implement visibility setting and unify registry API paths ([1523a86](https://github.com/ctx-hq/ctx/commit/1523a86f5e75b306c76e698b987bdd97b376a90f))

## [0.9.0](https://github.com/ctx-hq/ctx/compare/v0.8.0...v0.9.0) (2026-03-29)


### Features

* **cli:** add single-file skill publishing via ctx push/publish &lt;file.md&gt; ([00a8d1a](https://github.com/ctx-hq/ctx/commit/00a8d1a))

## [0.8.0](https://github.com/ctx-hq/ctx/compare/v0.7.0...v0.8.0) (2026-03-29)


### Features

* **auth:** add whoami command to show current authenticated user ([2aaa16a](https://github.com/ctx-hq/ctx/commit/2aaa16ab14b92dc11d4a00792dafdb2262e28a29))

## [0.7.0](https://github.com/ctx-hq/ctx/compare/v0.6.0...v0.7.0) (2026-03-29)


### Features

* **auth:** auto-open browser on login and display username ([04e62d7](https://github.com/ctx-hq/ctx/commit/04e62d741ccb66cf3bee4aac9ecf5e95187b0792))

## [0.6.0](https://github.com/ctx-hq/ctx/compare/v0.5.0...v0.6.0) (2026-03-29)


### Features

* add background update check and `ctx upgrade` command ([518f030](https://github.com/ctx-hq/ctx/commit/518f03067cba12985fd36afd0cafc920a8f86546))
* **agent:** add detection and config support for 12 new coding agents ([d976da4](https://github.com/ctx-hq/ctx/commit/d976da4a954b3679ff39a01b72a9b4c9e42f3f3d))
* **cli:** add push/sync/org/dist-tag/visibility/enrich commands for Registry v2 ([04d0852](https://github.com/ctx-hq/ctx/commit/04d0852ce6f0ee46277beeee92e0772064294eee))
* initialize ctx CLI project ([c988e8d](https://github.com/ctx-hq/ctx/commit/c988e8da11377e194f87ccf4e98da0719c0bedb0))
* **installer,output:** add link registry, version store, output layer and agent extensions ([a089f97](https://github.com/ctx-hq/ctx/commit/a089f9701124b7da01085501c415563ae731a476))
* **installer:** add Windows install script and branded install URLs ([31f0281](https://github.com/ctx-hq/ctx/commit/31f0281e7489be7a1b52cf38d03dc368c222bfe8))
* **install:** fix skill linking, add --caller flag, post-install guidance ([2c57912](https://github.com/ctx-hq/ctx/commit/2c5791285b977dd4e52152cabf31b3de18d365f6))
* **install:** production-grade zero-friction installer for all platforms ([d03d540](https://github.com/ctx-hq/ctx/commit/d03d5401086acfd9f96664acc4bb56404fa86c48))
* **install:** zero-friction install with auto PATH configuration ([1c192ce](https://github.com/ctx-hq/ctx/commit/1c192ce6fe0401d5222adaa966c5ab7c9fe1efb7))
* **release:** add release pipeline, supply-chain signing, and installer hardening ([9993612](https://github.com/ctx-hq/ctx/commit/99936123749719666dde9a50c0adb005973c27ca))
* **skill:** add cross-platform bootstrap section to SKILL.md ([b32d405](https://github.com/ctx-hq/ctx/commit/b32d405e8d1eeea15d67fd72fa12b6d0310df794))


### Bug Fixes

* address security, race condition and logic issues from code review ([21d962c](https://github.com/ctx-hq/ctx/commit/21d962c4a95db86b607f2310ea6d89f97ab24bb4))
* **ci:** correct hallucinated GitHub Action SHAs and goreleaser deprecation ([c2ac7f0](https://github.com/ctx-hq/ctx/commit/c2ac7f00c5978ed92af6d359bbe284f4f4154536))
* enable true parallel downloads in update, validate parsePackageRef, and sync README for org commands ([b647bca](https://github.com/ctx-hq/ctx/commit/b647bca861a05a3bbc42705c621a7e6013e45e6c))
* **install:** use exact match in checksum grep to avoid SBOM filename collision ([207802a](https://github.com/ctx-hq/ctx/commit/207802af913bee7882adafc94244f862b89616e1))
* **install:** zero-friction install with smart directory resolution ([439e98a](https://github.com/ctx-hq/ctx/commit/439e98ac0e103f09a6d8d514681c529723c98db9))
* **release:** add --yes to cosign sign-blob to skip interactive prompt in CI ([120f9fc](https://github.com/ctx-hq/ctx/commit/120f9fc81c5811ef890212a2957c2491b19ba973))
* resolve all errcheck lint errors and improve error propagation ([8ea6cd5](https://github.com/ctx-hq/ctx/commit/8ea6cd5e977161107faec58aa446c0cae321f0da))
* unignore skills/ctx/ directory so SKILL.md is tracked ([482affd](https://github.com/ctx-hq/ctx/commit/482affdc0e806b35a949218cfd038b082d863bf4))
