# Change Log

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

<a name="0.2.7"></a>
## [0.2.7](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.6...v0.2.7) (2020-07-24)


### Bug Fixes

* **recovery:** init recovery when we received a notification from primary with a larger view than se ([09bc208](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/09bc208)), closes [#flato-1859](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1859)



<a name="0.2.6"></a>
## [0.2.6](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.5...v0.2.6) (2020-07-22)


### Bug Fixes

* **recovery:** do not init recovery when we receive a notification from primary ([47e6df2](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/47e6df2)), closes [#flato-1859](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1859)



<a name="0.2.5"></a>
## [0.2.5](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.4...v0.2.5) (2020-07-09)


### Bug Fixes

* **SendFilterEvent:** send recovery finished event whenever recovery has finished ([8245e9f](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/8245e9f)), closes [#flato-1736](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1736)
* **unit_test:** fix unit test fetch missing ([e109d69](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/e109d69))



<a name="0.2.4"></a>
## [0.2.4](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.3...v0.2.4) (2020-07-06)


### Bug Fixes

* **epoch:** reset storage after the execution of config batch whether v-set was changed or not ([e76a549](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/e76a549)), closes [#flato-1729](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1729)
* **epoch-sync:** update stable checkpoint after state updated during recovery ([a71a012](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/a71a012)), closes [#flato-1730](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1730)



<a name="0.2.3"></a>
## [0.2.3](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.2...v0.2.3) (2020-06-30)


### Bug Fixes

* **epoch-check:** divide clearly the 2 types of epoch-check ([5c7611e](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5c7611e)), closes [#flato-1669](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1669)



<a name="0.2.2"></a>
## [0.2.2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.1...v0.2.2) (2020-05-26)


### Features

* send stable checkpoint after config tx executed ([ed1fa70](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/ed1fa70)), closes [#flato-1317](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1317)



<a name="0.2.1"></a>
## [0.2.1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.0...v0.2.1) (2020-04-29)


### Bug Fixes

* **rbft_impl.go:** fix the problem after new node finished state upadte ([c0194e7](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/c0194e7)), closes [#flato-1370](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1370)



<a name="0.2.0"></a>
# [0.2.0](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.9...v0.2.0) (2020-04-17)


### Bug Fixes

* remove useless files about gitlab ci ([918f23c](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/918f23c))


### Features

* config transaction feature in rbft ([d754c92](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/d754c92)), closes [#flato-899](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-899)



<a name="0.1.9"></a>
## [0.1.9](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.8...v0.1.9) (2020-02-22)


### Bug Fixes

* data transfer between rbft and service ([ff228c0](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/ff228c0)), closes [#flato-1158](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1158)
* **node_mgr.go:** fix logger ([0c68945](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/0c68945))



<a name="0.1.8"></a>
## [0.1.8](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.7...v0.1.8) (2020-02-04)


### Bug Fixes

* **helper.go:** fix the function to check if current node is primary or not ([b780c21](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b780c21)), closes [#flato-1060](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1060)



<a name="0.1.7"></a>
## [0.1.7](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.6...v0.1.7) (2020-01-20)


### Bug Fixes

* synchronously modify routers after add/delete node ([f1bed3a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/f1bed3a)), closes [#flato-1060](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1060)
* **rbft:** fix flato-1041,input delIndex instead of delID when calling func getDelNV ([a8cc180](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/a8cc180)), closes [#1041](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/1041)
* **rbft_impl.go:** post StableCheckpoint event after normal movewatermark and stateupdated ([17d55d6](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/17d55d6))
* **rbft_impl.go:** remove judgement of newNode when postConfState ([74a2585](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/74a2585))
* **updateN:** change the place where NodeMgrUpdatedEvent return when updateN ([e23de24](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/e23de24)), closes [#flato-1060](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1060)
* **viewchange_mgr.go:** node in recovery can jump into viewChange if it find newView ([3ad2909](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/3ad2909))



<a name="0.1.6"></a>
## [0.1.6](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.5...v0.1.6) (2019-12-24)


### Bug Fixes

* **consensus:** Clear QPList before persist new QPList in prepare phase of abnormal status ([b71c124](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b71c124))
* **consensus:** the view will increase by one when the node restarts to clear the expired ([5d97620](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5d97620))
* **go.mod:** upgrade fancylogger to v0.1.2 ([9a82f0f](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/9a82f0f))



<a name="0.1.5"></a>
## [0.1.5](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.4...v0.1.5) (2019-12-18)


### Bug Fixes

* fix bug when adding several nodes at the same time ([0c7a94b](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/0c7a94b)), closes [#flato-921](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-921)
* remove status request channel to avoid blocking status request with normal consensus message ([69e4cd5](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/69e4cd5))
* **node.go:** modify the log level when received an unexpected ServiceState ([5a4619d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5a4619d))
* **rbft_impl.go/recovery_mgr.go:** don't stop recoveryRestartTimer when resetStateForRecovery ([0a20424](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/0a20424))



<a name="0.1.4"></a>
## [0.1.4](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.3...v0.1.4) (2019-11-26)


### Bug Fixes

* **node.go:** don't reject stateUpdated event even if the given Applied is lower than current Applie ([57a4954](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/57a4954)), closes [#flato-698](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-698)



<a name="0.1.3"></a>
## [0.1.3](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.2...v0.1.3) (2019-10-21)



<a name="0.1.2"></a>
## [0.1.2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.1.1...v0.1.2) (2019-09-19)



<a name="0.1.1"></a>
## 0.1.1 (2019-09-11)


### Bug Fixes

* fix start func and type assertion ([b6937f4](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b6937f4))
* ignore check of empty batch when findNextCommitTx ([fa6f1df](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/fa6f1df))
* remove Extra field from ServiceState as it's user's responsibility to realize stateUpdate. ([5f03174](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5f03174))
* replace flato with flato-event to get correct Transaction structure. ([d06e36d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/d06e36d))


### Features

* add stable checkpoint filter event ([80b54d6](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/80b54d6))
* add support for add/delete node ([f5528b3](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/f5528b3))
* release flato-rbft v0.1 ([dbfeb9a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/dbfeb9a))
* **go.mod:** upgrade reference of flato-txpool to v0.1.1 ([d74a1a9](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/d74a1a9))



# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

## 0.1.0 (2019-08-23)


### Bug Fixes

* fix start func and type assertion ([b6937f4](///commit/b6937f4))
* remove Extra field from ServiceState as it's user's responsibility to realize stateUpdate. ([5f03174](///commit/5f03174))
* replace flato with flato-event to get correct Transaction structure. ([d06e36d](///commit/d06e36d))


### Features

* add stable checkpoint filter event ([80b54d6](///commit/80b54d6))
* add support for add/delete node ([f5528b3](///commit/f5528b3))
* release flato-rbft v0.1 ([dbfeb9a](///commit/dbfeb9a))
