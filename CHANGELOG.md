# Changelog

## [1.8.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.7.0...dotsync/v1.8.0) (2025-12-08)

### Features

* **smyklot:** normalize .yaml extensions to .yml ([d0461ef](https://github.com/smykla-labs/.github/commit/d0461ef69418a37162c898dddf75f4e09356555f))

## [1.7.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.6.1...dotsync/v1.7.0) (2025-12-08)

### Features

* **sync-smyklot:** add use_latest option ([1b43374](https://github.com/smykla-labs/.github/commit/1b433747f995b61542f13047514a81983707d103))

### Bug Fixes

* **rulesets:** use empty arrays instead of nil ([fe5b4a7](https://github.com/smykla-labs/.github/commit/fe5b4a765e2a390c8cc14aeb4ebd8c03d1f3aac3))

## [1.6.1](https://github.com/smykla-labs/.github/compare/dotsync/v1.6.0...dotsync/v1.6.1) (2025-12-08)

### Code Refactoring

* **action:** move action.yml to repo root ([1b194cb](https://github.com/smykla-labs/.github/commit/1b194cb636a37879ba584f03161f6d8e133d70d1))

## [1.6.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.5.0...dotsync/v1.6.0) (2025-12-08)

### Features

* **smyklot:** add org config and auto-install ([#20](https://github.com/smykla-labs/.github/issues/20)) ([619e896](https://github.com/smykla-labs/.github/commit/619e8967de408ce8be7ad042c84c74411fb5437f))

## [1.5.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.4.5...dotsync/v1.5.0) (2025-12-07)

### Features

* **settings:** add merge and dual schema ([#19](https://github.com/smykla-labs/.github/issues/19)) ([285d68b](https://github.com/smykla-labs/.github/commit/285d68b229b4b3da976891fc10481840e7bbebb0))

## [1.4.5](https://github.com/smykla-labs/.github/compare/dotsync/v1.4.4...dotsync/v1.4.5) (2025-12-06)

### Bug Fixes

* **settings:** remove invalid ruleset properties ([9467409](https://github.com/smykla-labs/.github/commit/94674096afde9b99f82032c1d22e1cf40a23fa2e))

## [1.4.4](https://github.com/smykla-labs/.github/compare/dotsync/v1.4.3...dotsync/v1.4.4) (2025-12-06)

### Bug Fixes

* **merge:** compact short arrays on single line ([1b4836e](https://github.com/smykla-labs/.github/commit/1b4836e856e01191521b0840e2265415d8a93c2c))

## [1.4.3](https://github.com/smykla-labs/.github/compare/dotsync/v1.4.2...dotsync/v1.4.3) (2025-12-06)

### Bug Fixes

* **merge:** preserve < > & in JSON output ([64e0506](https://github.com/smykla-labs/.github/commit/64e0506583bfc923cda8662ea8d263598f81396a))

## [1.4.2](https://github.com/smykla-labs/.github/compare/dotsync/v1.4.1...dotsync/v1.4.2) (2025-12-06)

### Bug Fixes

* **action:** use empty string for config default ([c2d08f1](https://github.com/smykla-labs/.github/commit/c2d08f18c73765d2700c6eef854a0f7c4a6d3d48))

## [1.4.1](https://github.com/smykla-labs/.github/compare/dotsync/v1.4.0...dotsync/v1.4.1) (2025-12-06)

### Bug Fixes

* **release:** CI matrix and goreleaser append ([#18](https://github.com/smykla-labs/.github/issues/18)) ([cb0f08d](https://github.com/smykla-labs/.github/commit/cb0f08d43142b83f9a58171d45a4a14dbd1cddbc))

## [1.4.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.3.1...dotsync/v1.4.0) (2025-12-06)

### Features

* **release:** add timeout to sync wait step ([617e86d](https://github.com/smykla-labs/.github/commit/617e86df297bca900ee37eea896e02d470cd07fe))
* **release:** pin version and coordinate syncs ([6798f71](https://github.com/smykla-labs/.github/commit/6798f71cb7b1aea5862208c5a27a60dad69239f8))
* **version:** add commit SHA and startup logging ([f06730f](https://github.com/smykla-labs/.github/commit/f06730fcdba455104e9acbb477221ce6bcc9f0ad))

### Code Refactoring

* **ci:** run tests and linters in parallel ([e841dcd](https://github.com/smykla-labs/.github/commit/e841dcd692ae62d17cae74b93c5a63c59cb65aa2))

## [1.3.1](https://github.com/smykla-labs/.github/compare/dotsync/v1.3.0...dotsync/v1.3.1) (2025-12-06)

### Bug Fixes

* **sync:** auto-fetch sync-config from targets ([4134cbf](https://github.com/smykla-labs/.github/commit/4134cbfb5b34fc43400f2a1e26e74392b9d2babf))

## [1.3.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.2.2...dotsync/v1.3.0) (2025-12-06)

### Features

* **files:** add merge support for file sync ([#16](https://github.com/smykla-labs/.github/issues/16)) ([b91d3b8](https://github.com/smykla-labs/.github/commit/b91d3b809a963f0f31fd726db78f1c87951b0efa))
* **settings:** add GitHub Rulesets sync support ([#15](https://github.com/smykla-labs/.github/issues/15)) ([aa8fa08](https://github.com/smykla-labs/.github/commit/aa8fa08baf498e18461b23c2645f4c8fe7ac2f43))

### Code Refactoring

* **schema:** extract schemagen to go run ([f4aaa80](https://github.com/smykla-labs/.github/commit/f4aaa80181c845b928ab7895f49af69246b309f1))
* **schema:** use JSONSchemaExtend methods ([11ecb93](https://github.com/smykla-labs/.github/commit/11ecb93e8ab03f38f4f0665e9043f211470b46f9))

## [1.2.2](https://github.com/smykla-labs/.github/compare/dotsync/v1.2.1...dotsync/v1.2.2) (2025-12-05)

### Bug Fixes

* **actions:** use snake_case for action inputs ([3f1f20d](https://github.com/smykla-labs/.github/commit/3f1f20dc712ab44b0b5d7b000c0ef2adb6e93355))
* **release:** prevent duplicate changelog titles ([e36f1e8](https://github.com/smykla-labs/.github/commit/e36f1e8cb78eb402621c83087664931e58132ae8))
* **release:** remove invalid empty changelogTitle ([ab483a9](https://github.com/smykla-labs/.github/commit/ab483a91729db993105663e11a3e3a37876208fc))

## [1.2.1](https://github.com/smykla-labs/.github/compare/dotsync/v1.2.0...dotsync/v1.2.1) (2025-12-05)

### Bug Fixes

* **cli:** add missing input validation ([#13](https://github.com/smykla-labs/.github/issues/13)) ([baedd48](https://github.com/smykla-labs/.github/commit/baedd48d26fc8ddbce0f34eb0872eb7d6a167023))
* **lint:** disable MD024 for CHANGELOG.md ([c64abbc](https://github.com/smykla-labs/.github/commit/c64abbc024d5ca237f1358e6677afa56294b5986))

## [1.2.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.1.0...dotsync/v1.2.0) (2025-12-05)

### Features

* **actions:** unify dotsync actions ([#12](https://github.com/smykla-labs/.github/issues/12)) ([397f739](https://github.com/smykla-labs/.github/commit/397f739e1c202dd2865ae7cf5d2515acc0c9ec68))

## [1.1.0](https://github.com/smykla-labs/.github/compare/dotsync/v1.0.2...dotsync/v1.1.0) (2025-12-05)

### Features

* **release:** add semantic-release automation ([#11](https://github.com/smykla-labs/.github/issues/11)) ([c5765dc](https://github.com/smykla-labs/.github/commit/c5765dcc53f889eb776a915946626085d4f036a7))
* **settings:** add settings sync to dotsync ([#9](https://github.com/smykla-labs/.github/issues/9)) ([9e70287](https://github.com/smykla-labs/.github/commit/9e7028715043c4d6912b1cd6e1b1ce0fbb3f357e))

### Bug Fixes

* **release:** add GORELEASER_CURRENT_TAG env var ([95422bc](https://github.com/smykla-labs/.github/commit/95422bcf4ffa33a83141c21ef1247f386a85c25b))
* **release:** use full tag with --skip=validate ([7471128](https://github.com/smykla-labs/.github/commit/7471128f0d5e32a6aa3f8261a26493883bdc38bd))
* **workflows:** access matrix.repo.name ([b2c8238](https://github.com/smykla-labs/.github/commit/b2c8238a03fa916591816471b32f22ede8b93192))

### Code Refactoring

* **cli:** deduplicate sync command logic ([6dac25e](https://github.com/smykla-labs/.github/commit/6dac25e9b44fe6895309b024ca8ec23600c94d15))
* **cli:** migrate from Kong to Cobra ([#10](https://github.com/smykla-labs/.github/issues/10)) ([c372a63](https://github.com/smykla-labs/.github/commit/c372a639236480be5f342f3b08287046a3873b82))
