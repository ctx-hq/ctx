# Changelog

## [0.30.1](https://github.com/ctx-hq/ctx/compare/v0.30.0...v0.30.1) (2026-04-04)


### Bug Fixes

* **init:** exclude source dirs from import and deduplicate same-name skills ([dfe880b](https://github.com/ctx-hq/ctx/commit/dfe880bfb4f7559296d83560f743bad79037620b))

## [0.30.0](https://github.com/ctx-hq/ctx/compare/v0.29.0...v0.30.0) (2026-04-04)


### Features

* **init:** add --import mode and enhance publish with --changed/--tag flags ([55b545e](https://github.com/ctx-hq/ctx/commit/55b545e63408bb7975d071ab9afa7af07b8b0c68))
* **publish:** unify private-to-public visibility upgrade across all publish paths ([20aa1da](https://github.com/ctx-hq/ctx/commit/20aa1da2fa8e32de3715881e3914a501f2fc1655))

## [0.29.0](https://github.com/ctx-hq/ctx/compare/v0.28.0...v0.29.0) (2026-04-03)


### Features

* improve publish UX with clearer display and smarter security scan ([1c39084](https://github.com/ctx-hq/ctx/commit/1c3908435365e9f149ea93e6bd36779e662872e8))

## [0.28.0](https://github.com/ctx-hq/ctx/compare/v0.27.2...v0.28.0) (2026-04-03)


### Features

* **init:** promote type selection to the first interactive prompt ([98d6e8f](https://github.com/ctx-hq/ctx/commit/98d6e8ff6d2c75755636f5a719fb61841b12719e))
* **output:** add detail view for single objects, improve whoami output ([8cada71](https://github.com/ctx-hq/ctx/commit/8cada7155d56e302c48026529c5171a665533f74))

## [0.27.2](https://github.com/ctx-hq/ctx/compare/v0.27.1...v0.27.2) (2026-04-03)


### Bug Fixes

* **selfupdate:** use GitHub token for update checks to avoid API rate limiting ([cb4cca9](https://github.com/ctx-hq/ctx/commit/cb4cca90de785057d989241d6518879b4fdd582a))

## [0.27.1](https://github.com/ctx-hq/ctx/compare/v0.27.0...v0.27.1) (2026-04-03)


### Bug Fixes

* **output:** add --human format flag and split per-stream color detection ([5f0fa26](https://github.com/ctx-hq/ctx/commit/5f0fa263305d3531d0f7411519fe03fad5459143))

## [0.27.0](https://github.com/ctx-hq/ctx/compare/v0.26.0...v0.27.0) (2026-04-03)


### Features

* **profile:** add multi-profile support and fix login/registry resolve chain ([eb96408](https://github.com/ctx-hq/ctx/commit/eb96408b06c048fb198680bd4ede0f15e068e51e))
* **security:** add package audit command and SHA256 integrity verification ([4105d4a](https://github.com/ctx-hq/ctx/commit/4105d4a0038a700d3042e9d93690ea249d88d550))

## [0.26.0](https://github.com/ctx-hq/ctx/compare/v0.25.0...v0.26.0) (2026-04-02)


### Features

* end-to-end CLI binary package support — artifact download, batch upload, PATH linking ([9f59f5c](https://github.com/ctx-hq/ctx/commit/9f59f5c7b088c90d69b86a7f1f8fe80b942ba941))

## [0.25.0](https://github.com/ctx-hq/ctx/compare/v0.24.2...v0.25.0) (2026-04-02)


### Features

* add token management, stars, artifact uploads, and permission hardening ([0ccf044](https://github.com/ctx-hq/ctx/commit/0ccf044f63fd6e0c41dd5ce4cdacc3a0c86b09bf))
* **init:** add README fetch/publish support with monorepo and GitHub API hardening ([2d40b8a](https://github.com/ctx-hq/ctx/commit/2d40b8a4d2561e180e8763457f53fa3e0bcb6122))
* **init:** add upstream auto-detection for ctx init and harden install pipeline ([f509086](https://github.com/ctx-hq/ctx/commit/f509086979616e8b7935e3aeab370fc9371326ac))
* **workspace:** auto-enrich author, repository, and license during workspace init ([172f59f](https://github.com/ctx-hq/ctx/commit/172f59fc3098c9b1be15ff448074471208259b58))

## [0.24.2](https://github.com/ctx-hq/ctx/compare/v0.24.1...v0.24.2) (2026-04-01)


### Bug Fixes

* **registry:** align CLI types with API ownership model (publisher → owner) ([2aa10a6](https://github.com/ctx-hq/ctx/commit/2aa10a6d78baf82f12eab1fc1c06505a650f7fdd))

## [0.24.1](https://github.com/ctx-hq/ctx/compare/v0.24.0...v0.24.1) (2026-04-01)


### Bug Fixes

* **publish:** include root-level reference files in skill packages and read SKILL.md in init ([37864ab](https://github.com/ctx-hq/ctx/commit/37864ab927325bc1ac3d6772f2f1c934cef27bad))

## [0.24.0](https://github.com/ctx-hq/ctx/compare/v0.23.1...v0.24.0) (2026-04-01)


### Features

* **publish:** auto-detect git metadata and LICENSE for package manifest ([3d2fc2d](https://github.com/ctx-hq/ctx/commit/3d2fc2daba3391908da724cb0050479d9193b2c6))
* **workspace:** add workspace and collection package types for multi-skill repos ([8619d86](https://github.com/ctx-hq/ctx/commit/8619d86f5310e6ab4fdd4cee3389fe21b5442fef))

## [0.23.1](https://github.com/ctx-hq/ctx/compare/v0.23.0...v0.23.1) (2026-03-31)


### Bug Fixes

* **skill:** include refactored description truncation in patch release ([e07bd3d](https://github.com/ctx-hq/ctx/commit/e07bd3d64f1b43d752d757afe399022c163965ea))

## [0.23.0](https://github.com/ctx-hq/ctx/compare/v0.22.2...v0.23.0) (2026-03-31)


### Features

* **mcp:** make skill optional for MCP packages, improve test diagnostics ([3f91943](https://github.com/ctx-hq/ctx/commit/3f9194383535e00109bedca3a651d25c1013a2fa))

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
