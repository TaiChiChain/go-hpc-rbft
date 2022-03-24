# Change Log

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

<a name="0.2.45"></a>
## [0.2.45](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.44...v0.2.45) (2022-03-24)



<a name="0.2.39-1"></a>
## [0.2.39-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.39...v0.2.39-1) (2022-01-29)


### Bug Fixes

* Reset txpool with some saving batches rather than restore pool after single recovery ([c81fedb](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/c81fedb))



<a name="0.2.44"></a>
## [0.2.44](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.43...v0.2.44) (2022-03-24)


### Features

* **crypto:** Hash solution ([d63d287](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/d63d287))



<a name="0.2.43"></a>
## [0.2.43](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.42...v0.2.43) (2022-03-11)


### Bug Fixes

* stop batch timer when node cannot create batch after batch timeout ([1893ca4](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/1893ca4))



<a name="0.2.42"></a>
## [0.2.42](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.41...v0.2.42) (2022-03-11)


### Bug Fixes

* reject create batch when node is not primary ([6c30ebd](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/6c30ebd))



<a name="0.2.41"></a>
## [0.2.41](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.40...v0.2.41) (2022-03-11)


### Features

* **checkpoint:** #flato-4275,adapt new checkpoint protocol ([08cbb04](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/08cbb04)), closes [#flato-4275](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-4275)



<a name="0.2.40"></a>
## [0.2.40](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.39...v0.2.40) (2022-03-01)



<a name="0.2.39-1"></a>
## [0.2.39-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.39...v0.2.39-1) (2022-01-29)


### Bug Fixes

* Reset txpool with some saving batches rather than restore pool after single recovery ([c81fedb](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/c81fedb))



<a name="0.2.39"></a>
## [0.2.39](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.38...v0.2.39) (2022-01-21)


### Bug Fixes

* turn into epoch after chain epoch has been changed ([76c6777](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/76c6777))



<a name="0.2.38"></a>
## [0.2.38](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.37-2...v0.2.38) (2022-01-17)


### Bug Fixes

* reset local checkpoint signature after epoch change in case of cert replace ([85a00a3](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/85a00a3))



<a name="0.2.37-2"></a>
## [0.2.37-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.37-1...v0.2.37-2) (2022-01-07)


### Bug Fixes

* stop new view timer after finish vc/recovery ([2a4064a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/2a4064a))



<a name="0.2.37-1"></a>
## [0.2.37-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.37...v0.2.37-1) (2021-12-14)


### Bug Fixes

* use remote consistent checkpoint to generate signed checkpoint after state update ([1b69be6](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/1b69be6)), closes [#flato-4175](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-4175)



<a name="0.2.37"></a>
## [0.2.37](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.36-2...v0.2.37) (2021-12-09)


### Features

* signed checkpoint: adapt config checkpoint ([80e7637](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/80e7637))
* signed checkpoint: adapt normal case ([873e3db](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/873e3db))
* signed checkpoint: adapt state update ([4dda07d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/4dda07d))
* signed checkpoint: adapt sync state ([df39212](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/df39212))
* signed checkpoint: adapt vc/recovery ([b05d815](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b05d815))
* signed checkpoint: remove IsNew flag totally ([556c65d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/556c65d))
* signed checkpoint: tidy protobuf ([3aadd9a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/3aadd9a))



<a name="0.2.36-2"></a>
## [0.2.36-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.36-1...v0.2.36-2) (2021-12-07)


### Bug Fixes

* don't stop namespace when find self not in router ([74fc6cc](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/74fc6cc))



<a name="0.2.36-1"></a>
## [0.2.36-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.36...v0.2.36-1) (2021-12-02)


### Bug Fixes

* stop namespace when find self not in router ([07f9014](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/07f9014))



<a name="0.2.36"></a>
## [0.2.36](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.35...v0.2.36) (2021-10-26)


### Bug Fixes

* deadlock of current status mutex ([062018a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/062018a)), closes [#flato-3973](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3973)
* **metrics.go:** replace histogram with summary to record actual txsPerBlock metrics ([03668c0](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/03668c0))



<a name="0.2.35"></a>
## [0.2.35](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.34-2...v0.2.35) (2021-09-08)



<a name="0.2.34-2"></a>
## [0.2.34-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.34-1...v0.2.34-2) (2021-08-17)


### Bug Fixes

* check the current view when we process initRecovery event and unit test ([7092c42](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/7092c42)), closes [#flato-3820](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3820)



<a name="0.2.34-1"></a>
## [0.2.34-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.34...v0.2.34-1) (2021-08-10)


### Bug Fixes

* modify log level ([b4ae21d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b4ae21d))



<a name="0.2.34"></a>
## [0.2.34](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.32-3...v0.2.34) (2021-08-06)


### Bug Fixes

* make sure that all the events could be received by event channel ([24b84fb](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/24b84fb))
* modify the log-printing, hash -> id ([a31cb2e](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/a31cb2e))
* propose view-change if we have found tx with mis-matched transaction from leader ([103cd6e](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/103cd6e)), closes [#flato-3802](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3802)



<a name="0.2.33"></a>
## [0.2.33](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.32...v0.2.33) (2021-06-22)


### Features

* **rbft_impl.go:** not stop ns after current node has been deleted ([6358f16](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/6358f16)), closes [#flato-3570](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3570)



<a name="0.2.33"></a>
## [0.2.33](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.32...v0.2.33) (2021-06-22)


### Features

* **rbft_impl.go:** not stop ns after current node has been deleted ([6358f16](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/6358f16)), closes [#flato-3570](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3570)



<a name="0.2.32-3"></a>
## [0.2.32-3](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.32-2...v0.2.32-3) (2021-07-22)


### Bug Fixes

* **viewchange:** record prePreparedTime when construct prePrepare in processNewView ([cfdb86a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/cfdb86a))



<a name="0.2.32-2"></a>
## [0.2.32-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.32-1...v0.2.32-2) (2021-07-05)


### Bug Fixes

* enter stateUpdate immediately when node is out of date ([56eff52](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/56eff52))



<a name="0.2.32-1"></a>
## [0.2.32-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.32...v0.2.32-1) (2021-06-29)


### Bug Fixes

* update high target every time expired ([cc4c0e3](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/cc4c0e3)), closes [#flato-3711](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3711)



<a name="0.2.32"></a>
## [0.2.32](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.31-3...v0.2.32) (2021-06-04)


### Features

* **go.mod:** update reference ([5bd9eca](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5bd9eca))



<a name="0.2.31-3"></a>
## [0.2.31-3](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.31-2...v0.2.31-3) (2021-04-26)


### Bug Fixes

* new node should close new-node target when it has found a stable epoch info ([b3495a5](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b3495a5)), closes [#flato-3383](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3383)



<a name="0.2.31-2"></a>
## [0.2.31-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.31-1...v0.2.31-2) (2021-04-20)


### Bug Fixes

* the new node could receive notification messages if its epoch larger than 0 ([6878052](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/6878052)), closes [#flato-3383](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3383)



<a name="0.2.31-1"></a>
## [0.2.31-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.31...v0.2.31-1) (2021-04-15)


### Performance Improvements

* **rbft:** eliminate missing batch when txpool received all missing txs of certain batch ([65ea86e](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/65ea86e))



<a name="0.2.31"></a>
## [0.2.31](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.30-2...v0.2.31) (2021-04-09)



<a name="0.2.30-2"></a>
## [0.2.30-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.30-1...v0.2.30-2) (2021-04-09)


### Performance Improvements

* **rbft_impl.go:** batch add requests into txpool to improve the performance of look up bloom_filte ([0c367b2](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/0c367b2)), closes [#flato-3340](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3340)



<a name="0.2.30-1"></a>
## [0.2.30-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.30...v0.2.30-1) (2021-04-08)


### Bug Fixes

* save useful batches before state update to avoid infinite vc caused by missing request batch ([a06f89b](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/a06f89b)), closes [#flato-3334](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3334)



<a name="0.2.30"></a>
## [0.2.30](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.29...v0.2.30) (2021-03-25)



<a name="0.2.27-2"></a>
## [0.2.27-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.27-1...v0.2.27-2) (2021-03-04)


### Bug Fixes

* don't set local before broadcast ([68167ae](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/68167ae)), closes [#flato-3234](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3234)
* we need extra process for view change when we finished state update ([f5392cb](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/f5392cb)), closes [#flato-3235](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3235)



<a name="0.2.27-1"></a>
## [0.2.27-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.27...v0.2.27-1) (2021-02-24)


### Bug Fixes

* **viewchange_mgr.go:** reject viewchange/recovery when node is in state transfer ([1397eab](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/1397eab))



<a name="0.2.29"></a>
## [0.2.29](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.28...v0.2.29) (2021-03-23)


### Bug Fixes

* **rbft_impl.go:** clean useless batch cache before stateupdate to avoid inconsistent state of tx's ([da32d15](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/da32d15))



<a name="0.2.28"></a>
## [0.2.28](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.27...v0.2.28) (2021-03-23)


### Performance Improvements

* add detailed metrics to analyze consensus time ([e94b20d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/e94b20d))



<a name="0.2.27"></a>
## [0.2.27](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.26-3...v0.2.27) (2021-02-02)


### Bug Fixes

* the process to stop node ([57f7eb7](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/57f7eb7)), closes [#flato-3135](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3135)


### Features

* update the process of checkpoint ([80b9a67](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/80b9a67)), closes [#flato-2998](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2998)
* **status_mgr.go:** add special status(Inconsistent) to indicate inconsistent checkpoint rather tha ([186d982](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/186d982)), closes [#flato-3129](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3129)



<a name="0.2.26-3"></a>
## [0.2.26-3](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.26-2...v0.2.26-3) (2021-01-27)


### Features

* do not split the request set before broadcast ([66dc408](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/66dc408)), closes [#flato-2874](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2874)



<a name="0.2.26-2"></a>
## [0.2.26-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.26-1...v0.2.26-2) (2021-01-20)


### Bug Fixes

* method to compare the checkpoints ([7fa428d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/7fa428d)), closes [#flato-3123](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3123)



<a name="0.2.26-1"></a>
## [0.2.26-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.26...v0.2.26-1) (2021-01-18)


### Bug Fixes

* we need to start high watermark for abnormal pre-prepare ([c67af06](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/c67af06)), closes [#flato-3119](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3119)



<a name="0.2.26"></a>
## [0.2.26](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.25-3...v0.2.26) (2021-01-13)


### Bug Fixes

* the concurrent problem for recovery and sync state ([b77cbc7](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b77cbc7)), closes [#flato-3064](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3064)



<a name="0.2.25-3"></a>
## [0.2.25-3](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.25-2...v0.2.25-3) (2021-01-06)


### Bug Fixes

* update the process to sync epoch ([cef9c00](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/cef9c00)), closes [#flato-3050](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3050)



<a name="0.2.25-2"></a>
## [0.2.25-2](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.25-1...v0.2.25-2) (2021-01-04)


### Bug Fixes

* fetch pqc after config change and view change ([20260ad](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/20260ad)), closes [#flato-3020](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3020)
* receive the checkpoints from replicas with h>low-watermark ([80acddd](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/80acddd)), closes [#flato-2987](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2987)
* **viewchange_mgr.go:** delete old neweView msg when received a new valid newView msg ([5181fb2](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5181fb2))



<a name="0.2.25-1"></a>
## [0.2.25-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.25...v0.2.25-1) (2020-12-25)


### Bug Fixes

* avoid fetchRequestBatch repeatly in one vc/recovery ([5cb2ab9](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5cb2ab9))
* change the method to deal with checkpoint out of range ([da1cef8](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/da1cef8)), closes [#flato-2997](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2997)
* modity the order to stop rbft core ([59f425f](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/59f425f)), closes [#flato-2996](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2996)



<a name="0.2.25"></a>
## [0.2.25](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.24...v0.2.25) (2020-12-01)


### Bug Fixes

* modify the process of config-change ([5336783](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5336783)), closes [#flato-2647](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2647)



<a name="0.2.24"></a>
## [0.2.24](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.23...v0.2.24) (2020-11-27)


### Bug Fixes

* we need to persist checkpoint in state-updated ([b685ce4](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b685ce4)), closes [#flato-2843](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2843)



<a name="0.2.23"></a>
## [0.2.23](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.22...v0.2.23) (2020-11-24)


### Bug Fixes

* change the timeout mechanism of high-watermark ([00093ff](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/00093ff)), closes [#flato-2834](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2834)
* fix checkpoint ([3f135ff](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/3f135ff)), closes [#flato-2827](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2827)
* **helper.go:** post stable checkpoint when finish sync state ([af39656](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/af39656))



<a name="0.2.22"></a>
## [0.2.22](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.21...v0.2.22) (2020-11-12)


### Bug Fixes

* post checkpoint digest in filter event ([ad77590](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/ad77590))



<a name="0.2.21"></a>
## [0.2.21](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.20...v0.2.21) (2020-11-09)


### Bug Fixes

* add fetch checkpoint timer event ([9f05b60](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/9f05b60))
* set poll full status whenever add/remove txs from pool ([7645de7](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/7645de7))
* start new view timer to ensure consensus progress ([75844ac](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/75844ac)), closes [#flato-2706](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2706)



<a name="0.2.20"></a>
## [0.2.20](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.19...v0.2.20) (2020-10-29)


### Bug Fixes

* change the place where to remove notification from lower view and epoch ([9c72794](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/9c72794)), closes [#flato-2643](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2643)
* remove batches smaller than initial checkpoint in recovery ([7c024f4](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/7c024f4)), closes [#flato-2644](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2644)
* **rbft_impl.go:** reject txs directly when node's pool is full ([e49b508](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/e49b508)), closes [#flato-2648](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2648)



<a name="0.2.19"></a>
## [0.2.19](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.18...v0.2.19) (2020-10-26)


### Bug Fixes

* fix oom caused by destroy minifile ([49f1506](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/49f1506))


### Features

* inform to stop namespace with delFlag channel if there is a non-recoverable error ([a20dac7](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/a20dac7)), closes [#flato-2446](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2446)



<a name="0.2.18"></a>
## [0.2.18](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.17...v0.2.18) (2020-10-22)


### Bug Fixes

* remove goroutine when we post message to resolve oom ([886af16](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/886af16)), closes [#flato-1794](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1794)
* **recovery:** update the process of recovery and state-update ([5847ffb](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5847ffb)), closes [#flato-2358](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2358)


### Features

* add metrics for node ID and status ([716fec9](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/716fec9))



<a name="0.2.17"></a>
## [0.2.17](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.16...v0.2.17) (2020-10-19)



<a name="0.2.16"></a>
## [0.2.16](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.15...v0.2.16) (2020-10-16)


### Bug Fixes

* **viewchange.go:** manage the config-change in viewchange ([02bfd98](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/02bfd98)), closes [#flato-2576](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2576)



<a name="0.2.15"></a>
## [0.2.15](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.14...v0.2.15) (2020-10-14)


### Bug Fixes

* **epoch_mgr.go:** clear the notification storage at the start of epoch ([941d579](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/941d579)), closes [#flato-2433](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2433)



<a name="0.2.14"></a>
## [0.2.14](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.13...v0.2.14) (2020-10-12)


### Bug Fixes

* reset cacheBatch and relates metrics before state update ([71a1e36](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/71a1e36)), closes [#flato-2502](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2502)
* **helper.go:** update metrics when set view ([4093dfc](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/4093dfc)), closes [#flato-2463](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2463)
* **viewchange_mgr.go:** put un-executed batch into outstandingReqBatches when processNewView to trig ([d03f492](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/d03f492)), closes [#flato-2514](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2514)



<a name="0.2.13"></a>
## [0.2.13](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.12...v0.2.13) (2020-09-24)


### Features

* support metrics feature ([1a5d076](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/1a5d076))



<a name="0.2.12"></a>
## [0.2.12](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.11...v0.2.12) (2020-09-17)


### Bug Fixes

* **rbft_impl.go:** judge status before primaryResubmitTransactions after stable checkpoint ([feb0ee0](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/feb0ee0)), closes [#flato-2354](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2354)



<a name="0.2.11"></a>
## [0.2.11](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.10...v0.2.11) (2020-09-14)


### Bug Fixes

* **rbft_impl.go:** priamry resubmit txs when normal checkpoint finished ([80ada6b](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/80ada6b)), closes [#flato-2257](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2257)
* **recovery_mgr.go:** restore pool in resetStateForRecovery when single node recovery from abnormal ([b42b32a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/b42b32a)), closes [#flato-2308](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-2308)



<a name="0.2.10"></a>
## [0.2.10](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.9...v0.2.10) (2020-09-03)


### Bug Fixes

* fix the problems in smoke tests ([a05818e](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/a05818e))
* **batch_mgr.go:** remove useless channel ([32e70f8](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/32e70f8))
* **initMsgMap:** only init message event map once ([8ee7c7b](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/8ee7c7b))
* **view-change:** don't send view-change in some situation ([5ace2f7](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/5ace2f7))


### Features

* **unit_test:** update unit tests for rbft core ([3f6325d](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/3f6325d))



<a name="0.2.9"></a>
## [0.2.9](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.8...v0.2.9) (2020-08-17)


### Features

* reconstruct the checkpoint and epoch-manager ([3efd8d2](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/3efd8d2)), closes [#flato-1678](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1678)



<a name="0.2.8"></a>
## [0.2.8](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.7...v0.2.8) (2020-07-30)


### Bug Fixes

* **epoch-check:** reject null request and pre-prepare in epoch-check process ([3aefc43](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/3aefc43)), closes [#flato-1924](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-1924)



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
