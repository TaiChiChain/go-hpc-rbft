# Change Log

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

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
