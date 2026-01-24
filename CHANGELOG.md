# Changelog

## [1.8.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.7.0...v1.8.0) (2026-01-24)


### Features

* add diagnostic logging and improved error tracking ([285e386](https://github.com/dbehnke/urfd-nng-dashboard/commit/285e3866f79fc307315ffee6dff9b0d9d3b69950))
* add RX/TX audio level meters to voice chat UI ([b005874](https://github.com/dbehnke/urfd-nng-dashboard/commit/b005874954d6d58e88b4ce31a49bd3c38d12b98f))
* add session timeout enforcement for transmit ([f12bea9](https://github.com/dbehnke/urfd-nng-dashboard/commit/f12bea9cb097e4a99f146a3a355ef0190ce546cd))
* add UI button to clear saved transmit password ([7d4fdff](https://github.com/dbehnke/urfd-nng-dashboard/commit/7d4fdff5e0a9fa11b2b4fbdc759c376524e410a4))
* add WebSocket connection recovery with exponential backoff ([4697f49](https://github.com/dbehnke/urfd-nng-dashboard/commit/4697f4979bad9621ebc60903352494d1d40dc82a))
* complete voice multiplexing and PTT state management ([52b252a](https://github.com/dbehnke/urfd-nng-dashboard/commit/52b252afec613ca70c4f7ec931dd0df0ddb7061b))
* implement half-duplex logic and complete PTT audio transmission ([35814f6](https://github.com/dbehnke/urfd-nng-dashboard/commit/35814f6d01da6f322620f3360433d2a852e2b42f))
* implement microphone permission handling and opus-recorder integration ([d5d7f02](https://github.com/dbehnke/urfd-nng-dashboard/commit/d5d7f02293ef3acd0a852c1be47d4575b1f3720f))
* implement web voice transmission with Ogg Opus encoding and live dashboard updates ([c7b551b](https://github.com/dbehnke/urfd-nng-dashboard/commit/c7b551b5372ac06e10358899de1168fe0081911f))


### Bug Fixes

* add missing config.yaml ([6788692](https://github.com/dbehnke/urfd-nng-dashboard/commit/6788692f20d9ac8cb0bf9d9023cb0cf4ead157a8))
* add path alias for @ in vite config ([ff76fe4](https://github.com/dbehnke/urfd-nng-dashboard/commit/ff76fe435fb456e20b91128a51b17f516960ccc4))
* add WebSocket handler for real-time recording updates ([4c26656](https://github.com/dbehnke/urfd-nng-dashboard/commit/4c266560bd662d8f90b339c7c5ec519936444119))
* correct libopus.js version to 0.0.1 ([e738d88](https://github.com/dbehnke/urfd-nng-dashboard/commit/e738d88441e4a2eed7ea90589421c44160f1b7bc))
* correct NNG OptionRecvDeadline to use time.Duration(-1) for no timeout ([8ab3b29](https://github.com/dbehnke/urfd-nng-dashboard/commit/8ab3b29f3afd820e9b20cb7676245e02ab8b128a))
* show clear password button whenever password is saved ([71f01e4](https://github.com/dbehnke/urfd-nng-dashboard/commit/71f01e41053f597014089914f4f63a8f578c5484))

## [1.7.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.6.1...v1.7.0) (2026-01-10)


### Features

* add nng control test tool ([7becd73](https://github.com/dbehnke/urfd-nng-dashboard/commit/7becd730d5bedc5ab0cd1c95cb31ba0f66514bf5))


### Bug Fixes

* **callbook:** sanitize callsigns to strip suffixes ([#18](https://github.com/dbehnke/urfd-nng-dashboard/issues/18)) ([210f3d0](https://github.com/dbehnke/urfd-nng-dashboard/commit/210f3d08ccefb7f4308f32a9e67be7e07d462c33))
* **lint:** check close error in test-control ([0fb9feb](https://github.com/dbehnke/urfd-nng-dashboard/commit/0fb9feb6244c2312c93cc7a40ebff18891f70604))

## [1.6.1](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.6.0...v1.6.1) (2026-01-06)


### Bug Fixes

* increase history limit to 100 and restore load more button ([#16](https://github.com/dbehnke/urfd-nng-dashboard/issues/16)) ([b6bd38f](https://github.com/dbehnke/urfd-nng-dashboard/commit/b6bd38f2ce321eb118d26ae9d7b43d175b34d6e8))

## [1.6.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.5.0...v1.6.0) (2026-01-04)


### Features

* Add DMR Dashboard Page ([b56701b](https://github.com/dbehnke/urfd-nng-dashboard/commit/b56701bf06161dd341e680e32a7d335e6ce9c735))
* Add Module Grid to Dashboard ([39cf172](https://github.com/dbehnke/urfd-nng-dashboard/commit/39cf172b7df423a0f3cbd95813ef54ac16a0ec26))
* Add Transmission Timer and Per-Module History Fetch ([4fffc08](https://github.com/dbehnke/urfd-nng-dashboard/commit/4fffc08fbbb02cec97fb6b3a4237310b6a459ad7))
* callbook integration and cache ([676b114](https://github.com/dbehnke/urfd-nng-dashboard/commit/676b114e99ca1b786b1968eaade85964dfa1a519))
* Display DMRID in DMR View ([4074733](https://github.com/dbehnke/urfd-nng-dashboard/commit/4074733899b713d59c5a51edc6e3ca65329ba171))
* Display Last Duration and Auto-Fill Player ([9f82d5f](https://github.com/dbehnke/urfd-nng-dashboard/commit/9f82d5f930eb87b3e0f595fa092344ea8b7b704c))
* Sync Player Context with Filtered View ([38aea39](https://github.com/dbehnke/urfd-nng-dashboard/commit/38aea39f634e0579bd1d29f0d4bdfd11ace72092))


### Bug Fixes

* Add Subscriptions to Client struct to prevent data drop ([e286775](https://github.com/dbehnke/urfd-nng-dashboard/commit/e28677561392b69b68e9f697de975ac4cc08296c))
* **callbook:** update RadioID user.csv URL ([f1e06fc](https://github.com/dbehnke/urfd-nng-dashboard/commit/f1e06fc2fef40dc66b04465a64eb9eafd0e07e85))
* Ensure Player respects Module Filter (Decouple live store) ([27d34dc](https://github.com/dbehnke/urfd-nng-dashboard/commit/27d34dcbb0f06984d491cf952143af19ede6b12b))
* Remove module badge from DMR View ([669e969](https://github.com/dbehnke/urfd-nng-dashboard/commit/669e9698df03b4a9aad3eba3fa5754b067747c56))
* Update Up Next label to reflect current filter ([03abe10](https://github.com/dbehnke/urfd-nng-dashboard/commit/03abe10a36e68ad5bfb220ac97d747f9eeadd500))

## [1.5.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.4.0...v1.5.0) (2025-12-31)


### Features

* Redesign Audio Player (Timeline + Mobile Card) ([#13](https://github.com/dbehnke/urfd-nng-dashboard/issues/13)) ([50b8c17](https://github.com/dbehnke/urfd-nng-dashboard/commit/50b8c17b5123a6f69e491fcec7443d376f11afc4))

## [1.4.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.3.0...v1.4.0) (2025-12-30)


### Features

* Implement Dashboard Pagination and 48h Limit ([#11](https://github.com/dbehnke/urfd-nng-dashboard/issues/11)) ([0c3010f](https://github.com/dbehnke/urfd-nng-dashboard/commit/0c3010f90893c597b794ed9c50eebeabd791acf3))

## [1.3.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.2.1...v1.3.0) (2025-12-27)


### Features

* improve config loading error handling with structured logging and add success messages ([c46ea33](https://github.com/dbehnke/urfd-nng-dashboard/commit/c46ea33e3053178bd00cd462330cc3f29dba190a))

## [1.2.1](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.2.0...v1.2.1) (2025-12-27)


### Bug Fixes

* **docker:** ensure frontend assets are embedded ([#6](https://github.com/dbehnke/urfd-nng-dashboard/issues/6)) ([abb6f5c](https://github.com/dbehnke/urfd-nng-dashboard/commit/abb6f5ce252281554cdeeb90e3b5f41b951ed05b))

## [1.2.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.1.0...v1.2.0) (2025-12-27)


### Features

* implement configurable module descriptions ([4b4db1f](https://github.com/dbehnke/urfd-nng-dashboard/commit/4b4db1fe1379e414d031e96bdbef7b4db2182b04))


### Bug Fixes

* docker build, mermaid diagram, and typescript shim ([a71debf](https://github.com/dbehnke/urfd-nng-dashboard/commit/a71debfd2d9eb22ec93793ade6078ec1fe6f53d7))

## [1.1.0](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.0.2...v1.1.0) (2025-12-26)


### Features

* implement dynamic versioning, docs update, and mobile polish ([d1e1509](https://github.com/dbehnke/urfd-nng-dashboard/commit/d1e1509a3603fc3011e2de47f03b10fb2ee17521))

## [1.0.2](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.0.1...v1.0.2) (2025-12-26)


### Bug Fixes

* **ci:** temporarily disable dockers_v2 to bypass buildx driver issues ([a15a21b](https://github.com/dbehnke/urfd-nng-dashboard/commit/a15a21b13f603314524ace0a16142ac7d23c3c02))

## [1.0.1](https://github.com/dbehnke/urfd-nng-dashboard/compare/v1.0.0...v1.0.1) (2025-12-26)


### Bug Fixes

* **ci:** resolve goreleaser deprecated fields (dockers_v2, formats) ([228bbd4](https://github.com/dbehnke/urfd-nng-dashboard/commit/228bbd48d79b90e190992b70b6a75b8ae5542d66))
* **ci:** resolve goreleaser deprecated fields (dockers_v2, formats) ([9d59348](https://github.com/dbehnke/urfd-nng-dashboard/commit/9d593480f099385ef3b80d69cbdd4af6308d4090))

## 1.0.0 (2025-12-26)


### Bug Fixes

* **backend:** resolve errcheck lint errors ([04241a1](https://github.com/dbehnke/urfd-nng-dashboard/commit/04241a13e171c65b1916057b6cd44306b7bd65b7))
* **ci:** use correct release-type 'go' for release-please ([0f1aefb](https://github.com/dbehnke/urfd-nng-dashboard/commit/0f1aefbd1b10cd230b7cf8463664d2fe86a02e2b))
* **frontend:** add missing test script to package.json ([5a07f83](https://github.com/dbehnke/urfd-nng-dashboard/commit/5a07f83b8270fc1ed604a9eff72a32f5e71a62c0))
* **simulator:** add missing Protocol field to ActiveTalker struct ([dae4cd3](https://github.com/dbehnke/urfd-nng-dashboard/commit/dae4cd359937b389006af13cfc07a5b3ae481574))
