# Changelog

## [0.7.2](https://github.com/scottames/thts/compare/v0.7.1...v0.7.2) (2026-03-18)


### Miscellaneous Chores

* **deps:** update ⬆️ github-actions to v3.6.3 ([#34](https://github.com/scottames/thts/issues/34)) ([3f6390f](https://github.com/scottames/thts/commit/3f6390ff9eb7bf7bb9cd4d1d054908ed01713b6f))
* **deps:** update dependency aqua:goreleaser/goreleaser to v2.14.3 ([#33](https://github.com/scottames/thts/issues/33)) ([dd70c45](https://github.com/scottames/thts/commit/dd70c454fa12414631dc8cd19ccbf633b3ee074f))

## [0.7.1](https://github.com/scottames/thts/compare/v0.7.0...v0.7.1) (2026-03-06)


### Miscellaneous Chores

* **deps:** update ⬆️ github-actions to v3.6.2 ([#32](https://github.com/scottames/thts/issues/32)) ([cbda9a2](https://github.com/scottames/thts/commit/cbda9a209437a72ad4644d49d45574d362164138))
* **deps:** update ⬆️ trunk go to v1.26.1 ([#29](https://github.com/scottames/thts/issues/29)) ([beb7f2f](https://github.com/scottames/thts/commit/beb7f2f0fe51666691f0e37b9abe8cc4a2a3b621))
* **deps:** update dependency go to v1.26.1 ([#30](https://github.com/scottames/thts/issues/30)) ([ea66a7c](https://github.com/scottames/thts/commit/ea66a7cc695c00957faaa962963ba0de932ea1e8))

## [0.7.0](https://github.com/scottames/thts/compare/v0.6.0...v0.7.0) (2026-03-02)


### Features

* **agents:** add Claude plan directive toggle ([72e2cc3](https://github.com/scottames/thts/commit/72e2cc36d64e43330b010c08b16bb870cdb0e5fd))


### Miscellaneous Chores

* **deps:** update dependency aqua:goreleaser/goreleaser to v2.14.1 ([#26](https://github.com/scottames/thts/issues/26)) ([a5b6f06](https://github.com/scottames/thts/commit/a5b6f06c8abaf41e5b441098611de97f2615ba89))
* **deps:** update dependency aqua:int128/ghcp to v1.15.2 ([#25](https://github.com/scottames/thts/issues/25)) ([9f2b7d2](https://github.com/scottames/thts/commit/9f2b7d2c411933e1d30f3570e483bb5c11f807ac))

## [0.6.0](https://github.com/scottames/thts/compare/v0.5.0...v0.6.0) (2026-02-20)


### Features

* worktree aware init ([c651634](https://github.com/scottames/thts/commit/c651634f2842c3f50ed94e7b9fc3bc2db9ac0ce5))


### Miscellaneous Chores

* **deps:** update dependency go to v1.26.0 ([#23](https://github.com/scottames/thts/issues/23)) ([4bf6665](https://github.com/scottames/thts/commit/4bf66651e9e665513ab80ac0f2cfb43beed42d1a))

## [0.5.0](https://github.com/scottames/thts/compare/v0.4.0...v0.5.0) (2026-02-15)


### Features

* **instructions:** add a shorthand for path references ([fa9585d](https://github.com/scottames/thts/commit/fa9585d17a9ead09aabd9cb4fbae95cba464c100))
* **instructions:** choice approach to leveraging research ([26f2bdf](https://github.com/scottames/thts/commit/26f2bdf807a501d870eabf8c797276214656c155))
* **state:** namespace repo mappings by config path ([918f04c](https://github.com/scottames/thts/commit/918f04cfd64931a0763e99c4c757c0088dd7045e))


### Bug Fixes

* **agents:** clarify thoughts-analyzer returns inline results only ([b9b96f5](https://github.com/scottames/thts/commit/b9b96f59ee4bdd3f4208ef7ccb23e7ea77fa9403))
* **uninit:** safer path removal ([3dd7132](https://github.com/scottames/thts/commit/3dd71326ed4b37be95fc8d77dc1c35f85aae3c18))


### Miscellaneous Chores

* **deps:** update ⬆️ trunk go to v1.25.7 ([#20](https://github.com/scottames/thts/issues/20)) ([946309c](https://github.com/scottames/thts/commit/946309c0d03b16407f5b5f41305e26dcc17b3392))
* **deps:** update ⬆️ trunk go to v1.26.0 ([#22](https://github.com/scottames/thts/issues/22)) ([e221247](https://github.com/scottames/thts/commit/e2212473826867ff32418a45e9760d29c88f1728))
* **deps:** update dependency go to v1.25.7 ([#21](https://github.com/scottames/thts/issues/21)) ([ace6f3f](https://github.com/scottames/thts/commit/ace6f3f39e51cbea4bc799deb622642ba87c13b2))


### Code Refactoring

* **init:** add profile info to stdout ([77ed2de](https://github.com/scottames/thts/commit/77ed2debead01328c9bd42c4f0945f3b1b156831))

## [0.4.0](https://github.com/scottames/thts/compare/v0.3.1...v0.4.0) (2026-01-30)


### Features

* **agents/init:** dry-run flag ([1b0bd91](https://github.com/scottames/thts/commit/1b0bd91ddae52ac6b37a604c0e091cd5fa74c0ac))
* **opencode:** align dirs and subagent mode metadata ([0a30290](https://github.com/scottames/thts/commit/0a3029034c7863aa65e6c561d7e2a973b64f5547))


### Code Refactoring

* thts agents init/uninit moved into thts init/uninit ([61a6317](https://github.com/scottames/thts/commit/61a63172ee4aee5dfd6d7302338704ed0f78e84a))

## [0.3.1](https://github.com/scottames/thts/compare/v0.3.0...v0.3.1) (2026-01-23)


### Miscellaneous Chores

* **deps:** update ⬆️ gomod patching to v2.23.1 ([#17](https://github.com/scottames/thts/issues/17)) ([46dc2e7](https://github.com/scottames/thts/commit/46dc2e7c0a3914a2489da18e8ba81f383a215464))
* **deps:** update dependency go to v1.25.6 ([#14](https://github.com/scottames/thts/issues/14)) ([31ee44a](https://github.com/scottames/thts/commit/31ee44abe4a3121ecb92d962721bf9bb0a37e2fc))
* **deps:** update github-actions ([#16](https://github.com/scottames/thts/issues/16)) ([2e35bb1](https://github.com/scottames/thts/commit/2e35bb17b77eefa08c61528ba6eaf1882d7cd30d))


### Code Refactoring

* **config:** move repoMappings to state file ([8424af4](https://github.com/scottames/thts/commit/8424af454ea028bd041ee9b64f901a73b6c66b50))

## [0.3.0](https://github.com/scottames/thts/compare/v0.2.0...v0.3.0) (2026-01-22)


### Features

* add support for Gemini CLI ([0127501](https://github.com/scottames/thts/commit/01275012dcaa32cf4466a9fe86155a6d14bb13bb))
* **add:** better content position + input modes ([39989f1](https://github.com/scottames/thts/commit/39989f1fe576dd366d54b2fce0b9c302cfb7adbc))
* **add:** better output for script handling + json output ([b56bf67](https://github.com/scottames/thts/commit/b56bf67b2c123c62cbe3015d80cac88f3085603e))
* **add:** support add/sync in one go ([37c980e](https://github.com/scottames/thts/commit/37c980eb214b24136fcd7f599fdc5c1da114fa2b))
* config dump-default ([3eb0047](https://github.com/scottames/thts/commit/3eb00479fda613aeb32a52bf8fdeea7b1a3f8fd5))
* configurable categories + `add` command ([5414b83](https://github.com/scottames/thts/commit/5414b834755b97f8bad01470b7846f7cdbbb4cc5))
* edit command ([380a8dc](https://github.com/scottames/thts/commit/380a8dcda43e676ac50140003a7e5a6a86487c21))
* instructions via command + cleanup embedded ([7ca0fbb](https://github.com/scottames/thts/commit/7ca0fbbd21de02e8dbde474bd5fcd1f2a18125cd))
* instructions/integration via hooks ([17ee446](https://github.com/scottames/thts/commit/17ee4460d7c8867bf5e67aae57d4446643463c75))
* **show:** add path flag for cd magic ([5bc55dc](https://github.com/scottames/thts/commit/5bc55dcc0eea6966153efa1739ce687aea2f3fc2))
* support setting some config via env ([85f264f](https://github.com/scottames/thts/commit/85f264ffee0c7b9798f8d8e2cb3832c11baefc63))
* **sync:** support custom commit message template ([4822994](https://github.com/scottames/thts/commit/4822994e0d4058874968016188ffdf48b70279ff))
* **sync:** support running in any location ([b9cc975](https://github.com/scottames/thts/commit/b9cc975810e09d9cdf9dce434a4472367678ed10))


### Bug Fixes

* agents uninit filter to agent ([827e55c](https://github.com/scottames/thts/commit/827e55c019cee7e8976295dd1b9b60a96679e09b))
* **claude:** hooks in local settings ([18bf15e](https://github.com/scottames/thts/commit/18bf15ef575d28b2c5c714c6998211dceb051a5c))
* **completion:** plumb add --in ([618f533](https://github.com/scottames/thts/commit/618f53311f853fa8da1dc0a7d34b804c54728b22))
* **hooks:** explicitly enable hooks with gemini ([96137d5](https://github.com/scottames/thts/commit/96137d56d4023107623b50aee672352305218667))
* **hooks:** only on valid repo ([9d937fc](https://github.com/scottames/thts/commit/9d937fcf6fecefdbc2310a9ece99e66eb77b67a8))
* **searchable:** warn when cross file-system ([682d341](https://github.com/scottames/thts/commit/682d34169b39f89ff4f160fa0c86fd86adbf3800))
* small fixes from previous ([bea5d7d](https://github.com/scottames/thts/commit/bea5d7d8fefc1dc8b9e14ece66b527820e185261))
* **status:** respect local-only sync mode ([c9ee2a6](https://github.com/scottames/thts/commit/c9ee2a6aa4cc9b7284bc2c87690adbaeaf5ed754))


### Miscellaneous Chores

* **agents:** add note on testing ([4501520](https://github.com/scottames/thts/commit/45015201cf79489bb7c32bd2fcd01eee17b7c85d))
* **deps:** update ⬆️ github-actions to v6 ([#7](https://github.com/scottames/thts/issues/7)) ([d9b059b](https://github.com/scottames/thts/commit/d9b059bb37bd68b963eaeb30900723f10fd59722))
* **deps:** update ⬆️ trunk go to v1.25.6 ([#13](https://github.com/scottames/thts/issues/13)) ([8981145](https://github.com/scottames/thts/commit/89811452f817da963fc7f9164147e6392dd0894c))
* **deps:** update dependency aqua:goreleaser/goreleaser to v2.13.3 ([#9](https://github.com/scottames/thts/issues/9)) ([f1f1540](https://github.com/scottames/thts/commit/f1f154025a70b000221e8328a9e81c6c41b5ef8d))
* **deps:** update dependency go to v1.25.5 ([#10](https://github.com/scottames/thts/issues/10)) ([38733f3](https://github.com/scottames/thts/commit/38733f362478f022f3419a2baaf0d68f911096a4))
* **deps:** update dependency go to v1.25.5 ([#12](https://github.com/scottames/thts/issues/12)) ([6011f80](https://github.com/scottames/thts/commit/6011f8092432aa02cfb74af3b14babb27445bf5b))
* move repo agents config to global ([7d31db6](https://github.com/scottames/thts/commit/7d31db61ccfd15f04e8f26c5efaa7e5fea88233f))


### Code Refactoring

* **config:** use maintained yaml pkg ([c8ca795](https://github.com/scottames/thts/commit/c8ca795b7552dd18282fa50c1abf2ebc77527342))
* DRY up hooks/agents/skills/commands ([c584bf4](https://github.com/scottames/thts/commit/c584bf407a34e642de992aed67417713acd44f58))

## [0.2.0](https://github.com/scottames/thts/compare/v0.1.1...v0.2.0) (2026-01-13)


### Features

* add config validate command ([f6cb8b8](https://github.com/scottames/thts/commit/f6cb8b86fc72ec8d5e99cbd641fd3d3f165dbeca))
* adding an agent guide + cleanup ([b8b298a](https://github.com/scottames/thts/commit/b8b298a79de2c38ccbae07dcc94d0e10f22df2fd))
* agents init, handle claude symlink to agents ([439ee43](https://github.com/scottames/thts/commit/439ee43f51ce0420318af435f235e2f625279963))
* **agents:** support global config ([d1f7507](https://github.com/scottames/thts/commit/d1f750706a4b9998c2c42cdcf1b3aef633b03c3a))
* cleanup agents ([41ad2c2](https://github.com/scottames/thts/commit/41ad2c205484b401c30bd4b638313a1bd25f9a25))
* **config:** sync mode - yubikey friendly ([54ec23e](https://github.com/scottames/thts/commit/54ec23e9dc4b6af44a86d6049e2d01cd46ef2eea))
* improve instructions + add templates ([31b8be9](https://github.com/scottames/thts/commit/31b8be9eb66ebf1c35b06f1f7f5ea307be0df108))
* option to disable push on sync ([9134e8d](https://github.com/scottames/thts/commit/9134e8d19aa21ab2ccab811a96f544323861ba4f))
* shell completion ([44742da](https://github.com/scottames/thts/commit/44742daada64770e96ca70c10047d1a55b8807b9))
* support codex & opencode in addition to claude ([d0a298a](https://github.com/scottames/thts/commit/d0a298a700850a5eae870b79a9d7bbfad73f678c))


### Bug Fixes

* **agents:** correct global paths + commands for all ([3390214](https://github.com/scottames/thts/commit/3390214a4accbd7cdb3c3eb33840b5ff341da6d0))
* error when no default profile ([47ea61a](https://github.com/scottames/thts/commit/47ea61a4d88470c9bb724e5b0b5c76b7f85ee43f))
* **init:** default repo assignment ([9c32071](https://github.com/scottames/thts/commit/9c3207129da11cebbcb463ffb434f5e5fc77327b))


### Code Refactoring

* extra spacing in messages ([ff133c7](https://github.com/scottames/thts/commit/ff133c793bee51a522263e44c3af6626f8975923))
* merge opencode instructions behavior with codex ([8482eb8](https://github.com/scottames/thts/commit/8482eb8a2796196921eb713b622dc4f7521fd2b4))

## [0.1.1](https://github.com/scottames/thts/compare/v0.1.0...v0.1.1) (2026-01-11)


### Bug Fixes

* setup dup default ([17e1066](https://github.com/scottames/thts/commit/17e1066997abc124ea0ebf4169ac1551460e91cc))


### Code Refactoring

* cleanup ui ([a0b087d](https://github.com/scottames/thts/commit/a0b087d185ff12f29018b05d77c939ab1b3dc7b0))
* profiles + yaml config ([373a555](https://github.com/scottames/thts/commit/373a5551442a905f79b80eb006be5adfc700e885))
* rename to thts ([bf8d760](https://github.com/scottames/thts/commit/bf8d760bdb8c551531086852e5ef28cf3bebab4a))

## [0.1.0](https://github.com/scottames/thts/compare/v0.0.1...v0.1.0) (2026-01-10)

### Features

- claude code integration
  ([dff9969](https://github.com/scottames/thts/commit/dff996918fa227b33e2da71b435809897b71ecfa))
- initial cli
  ([dd152ee](https://github.com/scottames/thts/commit/dd152eeee41e98c97d583e1f799064e656a5f41f))
