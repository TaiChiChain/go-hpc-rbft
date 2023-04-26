# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### [2.1.7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.6...v2.1.7) (2023-04-26)


### Bug Fixes

* **view_change.go:** check if need state update rather than directly state update to initial checkpo ([f4ca4c8](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f4ca4c8d7f566a2087764e27dd84b3e57fe94475))

### [2.1.6](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.5...v2.1.6) (2023-04-21)


### Bug Fixes

* **viewchange:** periodically fetch view when in viewChange instead of fetch view once we found view ([128c2ed](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/128c2edd79c8813d02877ad3f2693d375d139f7e))

### [2.1.5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.4...v2.1.5) (2023-04-19)


### Bug Fixes

* **epoch_mgr.go:** reject process epoch change proof with a higher start epoch than current epoch ([f959f1a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f959f1af8eb5aa45f7d61547e41e9b90d58cb480)), closes [#flato-5603](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5603)

### [2.1.4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.3...v2.1.4) (2023-04-17)


### Bug Fixes

* enter conf change when find config batch in batchStore and reject null request in conf chang ([31edecd](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/31edecd809efe5190de07c1009d701592558fd4b)), closes [#flato-5570](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5570)

### [2.1.3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.2...v2.1.3) (2023-04-13)


### Features

* **state_update:** provide epoch change proof when the sync request has a backwardness of epoch change ([be4f90e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/be4f90ee754ef15a1583e93b068b895a3ea37b3d))

### [2.1.2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.1...v2.1.2) (2023-04-11)


### Bug Fixes

* **exec.go:** reject process epoch change related messages in same epoch ([cd9ae7c](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cd9ae7cb97c29f5d82f182c3b4bfc671d76dab04)), closes [#flato-5563](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5563)

### [2.1.1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.1.0...v2.1.1) (2023-04-06)


### Bug Fixes

* re-sign checkpoint after epoch change to avoid invalid signature after cert replace ([14be9e4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/14be9e42778f8d66c631d957b0938478a33ae139)), closes [#flato-5557](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5557)

## [2.1.0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v2.0.0...v2.1.0) (2023-04-03)


### Features

* **consensus:** Tracing with OpenTelemetry ([f62c96f](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f62c96f8d9f0dead61b7084cc3fa9f659244c6e9))

## [2.0.0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.2.0...v2.0.0) (2023-04-03)


### Features

* **fork:** update to v2, modify consensus proto type, remove recovery and adapt to view change ([d43bba4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d43bba4ccb6807f5d0f969ad5d2f96a853e43a8b))

## [1.2.0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.1.1...v1.2.0) (2023-03-30)

### [1.1.1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.1.0...v1.1.1) (2023-03-30)


### Bug Fixes

* catch panic for send on closed channel ([6d7d494](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/6d7d494836d2f2ad2d8dc2a653b2f90b2d0567e4))

## [1.1.0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.10...v1.1.0) (2023-02-22)


### Features

* **go.mod:** update common and txpool ([7f0821e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/7f0821e86ede698032087af13eed33946f7ec22f))

### [1.0.10](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.9...v1.0.10) (2023-02-14)


### Bug Fixes

* **helper.go:** retry waiting checkpoint when received mismatch height ([79ad588](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/79ad5887fe75414d80fd0e24a4860b6910656a5b))

### [1.0.9](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.8...v1.0.9) (2023-02-11)


### Bug Fixes

* **rbft_impl.go:** #flato-5419, no lost transactions after Order ([a7d0076](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a7d0076aac9ab0b02ffdb3983131f400d70db372)), closes [#flato-5419](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5419)

### [1.0.8](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.7...v1.0.8) (2023-02-08)


### Bug Fixes

* **rbft_impl.go:** only generate checkpoint and move watermark when epochChanged or reach checkpoint ([45f9e0a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/45f9e0a2fec3d258676de2abd7b3fde6c4f662bf)), closes [#flato-5420](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5420)

### [1.0.7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.6...v1.0.7) (2023-02-07)


### Bug Fixes

* **node.go:** always close cpChan and confChan when stop node ([fc527e1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/fc527e1d5f2bf087aef19cee39afa956a63073d8))

### [1.0.6](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.5...v1.0.6) (2023-02-07)


### Bug Fixes

* **rbft_impl.go:** reset status when stop rbft ([a2f6905](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a2f6905860c9cb471f29e9744e3a4b1d187f5b9d))

### [1.0.5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.4...v1.0.5) (2023-01-13)


### Bug Fixes

* **epoch:** clear lower epoch record as soon as possible ([487611b](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/487611bdabd29f74dd5291ffd41a60227e207fc7))

### [1.0.4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.3...v1.0.4) (2023-01-12)


### Bug Fixes

* **helper.go:** try to rollback when find mismatch in checkIfNeedStateUpdate ([5d222fd](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5d222fd4910c857242e47bcda97ccb9488b72d69))

### [1.0.3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.2...v1.0.3) (2023-01-04)


### Bug Fixes

* not generate initial config checkpoint with no quorum signatures ([cf6cfba](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cf6cfba76ec90c6cfd5d28b96b7185ea1152d1b1)), closes [#flato-5321](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5321)
* process config checkpoint when received quorum config checkpoint ([1892070](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/18920700d3809ae7637c7527bec18d51d3965801))
* remove ReloadFinished related events ([f221cab](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f221cab84e33b831972bda739aa61eaee9cebee6))
* **view_change:** fetch config checkpoint when found a config batch in x-set ([d770177](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d7701774651cd6da944a835a72d89c2725a4afe0))
* **viewchange:** sync config checkpoint when checkIfNeedStateUpdate and always syncEpoch after vc/re ([8d45f71](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/8d45f713dad16658e8587d1073daa83507055ecd))
* **viewchange:** trigger viewchange when received quorum viewchanges in recovery status ([5b41287](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5b4128714fb3a02a3af1a5618378686045493c0d))

### [1.0.2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.1...v1.0.2) (2023-01-03)


### Bug Fixes

* **recovery:** reuse process resetStateForNewView in resetStateForRecovery to avoid cache some unuse ([86627d2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/86627d2981fbe7845048120317f74acd67b75990)), closes [#flato-5322](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5322) [#flato-4326](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4326)

### [1.0.1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v1.0.0...v1.0.1) (2022-11-21)


### Bug Fixes

* **node.go:** remove committed txs in GetUncommittedTransactions() ([809cab5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/809cab5761d3cd8702375ce399351e43f317b24f))

## [1.0.0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.5...v1.0.0) (2022-11-15)


### Features

* support algorithm upgrade ([f9b69b0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f9b69b09e4bd97527a53725c82e7d5fec394c9a5))

### [0.4.5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.4-2...v0.4.5) (2022-11-01)

### [0.4.4-2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.4-1...v0.4.4-2) (2022-10-26)

### [0.4.4-1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.4...v0.4.4-1) (2022-10-10)


### Bug Fixes

* **rbft_impl:** [#5126](null/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/5126), cache ctx when in config change ([e0b8193](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/e0b819363f40e511e75fbba8afca2fbd2963252a))

### [0.4.4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.3-3...v0.4.4) (2022-09-22)

### [0.4.3-3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.3-2...v0.4.3-3) (2022-09-13)


### Bug Fixes

* #flato-5095, update certStore after cut blocks to continue consensus ([c59e99e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/c59e99eb57e70a4c789b34d6e128f14398af8d10)), closes [#flato-5095](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-5095)

### [0.4.3-2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.3-1...v0.4.3-2) (2022-08-25)


### Bug Fixes

* return txs in recvChan before rbft stop ([e7504e6](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/e7504e69998e953631c4fa46b1ffe66603d27970))

### [0.4.3-1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.3...v0.4.3-1) (2022-08-24)


### Bug Fixes

* **rbft_impl:** checkpoint nil panic ([5942584](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5942584177d783d86e2ce1942223d994c74e7a72))

### [0.4.3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.2...v0.4.3) (2022-08-23)

## [0.4.0-1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.0...v0.4.0-1) (2022-08-01)


### Bug Fixes

* **recovery_mgr.go:** exit sync state when node status is abnormal ([cf94cb5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cf94cb57caf22bcac79f69f1350f8927040e4905)), closes [#flato-4969](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4969)

### [0.4.2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.1...v0.4.2) (2022-08-12)


### Features

* switch consensus algorithm online ([2f78e73](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/2f78e73c89b46dce6ad00323b0cc95416244539b))

### [0.4.1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.0...v0.4.1) (2022-08-08)


### Bug Fixes

* #flato-4804, fetchMissing immediately after receiving the PrePrepare message ([0d7e583](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/0d7e58320e165d17b39ca34de8e93bdecbf764b5)), closes [#flato-4804](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4804)

## [0.4.0-1](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.4.0...v0.4.0-1) (2022-08-01)


### Bug Fixes

* **recovery_mgr.go:** exit sync state when node status is abnormal ([cf94cb5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cf94cb57caf22bcac79f69f1350f8927040e4905)), closes [#flato-4969](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4969)

## [0.4.0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/compare/v0.3.1...v0.4.0) (2022-07-05)


### Bug Fixes

* decrease epoch when generate checkpoint with a low chain height ([7577e85](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/7577e857c25022bd05299e14f9aa9c90b3e61883)), closes [#flato-4741](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4741)

### 0.3.1 (2022-06-16)


### Features

* add metrics for node ID and status ([716fec9](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/716fec920fb29ad697bbda27e3b775411d0cfd3a))
* add stable checkpoint filter event ([80b54d6](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/80b54d66cf71ba0cd699977fdf187bb4cf49e2c8))
* add support for add/delete node ([f5528b3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f5528b3495c6cdee7ad7dd5ad57a0abc5b14cdab))
* **checkpoint:** #flato-4275,adapt new checkpoint protocol ([08cbb04](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/08cbb04c4e47e937aff5ef64960e86b2f9f175be)), closes [#flato-4275](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4275)
* config transaction feature in rbft ([d754c92](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d754c92ae88db46b419ce853a2dd740c1d0bac11)), closes [#flato-899](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-899)
* **crypto:** Hash solution ([d63d287](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d63d287e873e02858d7f50ecf8c2074deef29695))
* do not split the request set before broadcast ([66dc408](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/66dc40883de42df0c13d69f8d8a3e07e4f3e26f3)), closes [#flato-2874](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2874)
* **epoch:** #flato-4506, adapt verifiable epoch change and state update ([78ccfeb](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/78ccfebda8f8a402fadceab0a662117d3926f62c)), closes [#flato-4506](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4506)
* **go.mod:** update go version to 1.17 & add go.sum ([afed51a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/afed51a08316f8f40ccc6653ff1a69bd0dac11d4))
* **go.mod:** update reference ([5bd9eca](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5bd9eca827d210e4292b4178716edac03eba81d5))
* **go.mod:** upgrade reference of flato-txpool to v0.1.1 ([d74a1a9](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d74a1a9b3f91570b8546c31c285f3dbb072dd5e2))
* **hash:** #flato-4540, transaction hash optimization ([6e05a20](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/6e05a20cf022e441301219a33aaceabdcd8157cc)), closes [#flato-4540](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4540)
* inform to stop namespace with delFlag channel if there is a non-recoverable error ([a20dac7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a20dac7e180ce14cb464c2e25aa6eca98192a2c9)), closes [#flato-2446](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2446)
* **noxbft:** #flato-4331, adapt hotstuff ([577f2de](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/577f2dea01d7b70353040cc33f938423598b2bab)), closes [#flato-4331](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4331)
* **rbft_impl.go:** not stop ns after current node has been deleted ([6358f16](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/6358f1617ad04db4e4b3fd46516b7cd21fa0f765)), closes [#flato-3570](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3570)
* reconstruct the checkpoint and epoch-manager ([3efd8d2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/3efd8d2fc3d6287715da5538677b56fdaae0075c)), closes [#flato-1678](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1678)
* release flato-rbft v0.1 ([dbfeb9a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/dbfeb9ab93f9a71dcd3a23bd840cc4c5a7d5b74f))
* send stable checkpoint after config tx executed ([ed1fa70](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/ed1fa70add004a586e4569356af500f8f197dd31)), closes [#flato-1317](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1317)
* signed checkpoint: adapt config checkpoint ([80e7637](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/80e7637d86c0e1acfb75b35f7bddfddc617afac5))
* signed checkpoint: adapt normal case ([873e3db](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/873e3dbc3af0f8061681f26fc435b7df52affc5a))
* signed checkpoint: adapt state update ([4dda07d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/4dda07d9c133a7b9e8683ee478eab79bce78d8ab))
* signed checkpoint: adapt sync state ([df39212](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/df3921254820f5fe2f75144c600d0c3d41694db5))
* signed checkpoint: adapt vc/recovery ([b05d815](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b05d815d5d8686a12cda22a36167e25dbd7dcb98))
* signed checkpoint: remove IsNew flag totally ([556c65d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/556c65d06a165d6ec42d9066a8cf3166baeaa133))
* signed checkpoint: tidy protobuf ([3aadd9a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/3aadd9a3e512aa3179e5682dbbe9936b183a718f))
* **status_mgr.go:** add special status(Inconsistent) to indicate inconsistent checkpoint rather tha ([186d982](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/186d98292b2185960d945f9048f2fd007af8d8c0)), closes [#flato-3129](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3129)
* support metrics feature ([1a5d076](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/1a5d076c7cad7d83c62979fb498a4ac2b5c082a6))
* **unit_test:** update unit tests for rbft core ([3f6325d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/3f6325d57016720ba4ebaf5fe3a6f5364ebc5d5d))
* update the process of checkpoint ([80b9a67](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/80b9a675453c9fea8c9eb2d1014bdf5972cd343e)), closes [#flato-2998](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2998)


### Bug Fixes

* add fetch checkpoint timer event ([9f05b60](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/9f05b60173320d870bc91c8efd78f579a668d81e))
* avoid fetchRequestBatch repeatly in one vc/recovery ([5cb2ab9](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5cb2ab9942d946ccae31bd48fd0bbae9c736f6e6))
* **batch_mgr.go:** remove useless channel ([32e70f8](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/32e70f85861ad8915583efb2491c3b7b1e87b5f9))
* change the method to deal with checkpoint out of range ([da1cef8](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/da1cef8deb266db5c5169eb12a72ca1892600e7b)), closes [#flato-2997](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2997)
* change the place where to remove notification from lower view and epoch ([9c72794](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/9c7279494b67863722dabaf726380480e4636a29)), closes [#flato-2643](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2643)
* change the timeout mechanism of high-watermark ([00093ff](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/00093ff1812e865fbae83be1e5cf2298570367e8)), closes [#flato-2834](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2834)
* check the current view when we process initRecovery event and unit test ([7092c42](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/7092c42b640d114b8e0a2b67b0126926c61414de)), closes [#flato-3820](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3820)
* **consensus:** Clear QPList before persist new QPList in prepare phase of abnormal status ([b71c124](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b71c12470f1955c77c9bc218098719ceb9df9b14))
* **consensus:** the view will increase by one when the node restarts to clear the expired ([5d97620](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5d976209632c62b2f96d82900d05c25f6f3a9ce0))
* data transfer between rbft and service ([ff228c0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/ff228c0b2b1966fc428699cfccfc483dd970a383)), closes [#flato-1158](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1158)
* deadlock of current status mutex ([062018a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/062018a4850a73b314d61499be0110ad969c1e04)), closes [#flato-3973](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3973)
* don't set local before broadcast ([68167ae](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/68167ae93bfc7917f9c52cc2f8e539191568a87a)), closes [#flato-3234](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3234)
* don't stop namespace when find self not in router ([74fc6cc](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/74fc6cc1335a759d537312b0d828f7f980a6de63))
* enter stateUpdate immediately when node is out of date ([56eff52](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/56eff52b4caae78bb9e4983f73dff754a3e4adca))
* **epoch_mgr.go:** clear the notification storage at the start of epoch ([941d579](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/941d579d08eae96b99e0bf7327e7a5beb623604b)), closes [#flato-2433](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2433)
* **epoch-check:** divide clearly the 2 types of epoch-check ([5c7611e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5c7611efe8edb441438e3a5baeb36c322eb779c1)), closes [#flato-1669](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1669)
* **epoch-check:** reject null request and pre-prepare in epoch-check process ([3aefc43](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/3aefc43d56c6f4bc09535139e44d34072e6a550f)), closes [#flato-1924](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1924)
* **epoch-sync:** update stable checkpoint after state updated during recovery ([a71a012](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a71a012966dd32944a0931d32b792b9dcacff639)), closes [#flato-1730](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1730)
* **epoch:** reset storage after the execution of config batch whether v-set was changed or not ([e76a549](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/e76a54979451f7bd38ecbdafd9e212e1f82f672c)), closes [#flato-1729](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1729)
* fetch pqc after config change and view change ([20260ad](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/20260adfbce2da15b373983d258139aeb378978d)), closes [#flato-3020](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3020)
* fix bug when adding several nodes at the same time ([0c7a94b](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/0c7a94b9a98ec12cc057f8577a3c93ed874bc007)), closes [#flato-921](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-921)
* fix checkpoint ([3f135ff](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/3f135ff54324d63472960c1292ebd8c9b5d1d920)), closes [#flato-2827](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2827)
* fix oom caused by destroy minifile ([49f1506](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/49f1506647de432c43d7ff39c54662fe751a3519))
* fix some logger and print ([1d2ae60](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/1d2ae604ba5d88250c13623e668027aa64d47409))
* fix start func and type assertion ([b6937f4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b6937f4d4e3260fe8be7cec9b0e8ca44467f200b))
* fix the problems in smoke tests ([a05818e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a05818ef0bd4ea57e4adbc23e95e5f90ab6ce8a5))
* **go.mod:** upgrade fancylogger to v0.1.2 ([9a82f0f](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/9a82f0f2bb5a65b29e11cf8585c73dc92bc45e43))
* **helper.go:** fix the function to check if current node is primary or not ([b780c21](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b780c214fe2ba87d0815e2a029d7f8579ecf7871)), closes [#flato-1060](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1060)
* **helper.go:** post stable checkpoint when finish sync state ([af39656](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/af39656458a42db19b9a561040206e1274489aea))
* **helper.go:** update metrics when set view ([4093dfc](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/4093dfc3c8b75b2409d7baf48cac880465abf750)), closes [#flato-2463](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2463)
* ignore check of empty batch when findNextCommitTx ([fa6f1df](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/fa6f1df580dd936dc1245987a05da857ebd29de1))
* **initMsgMap:** only init message event map once ([8ee7c7b](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/8ee7c7b0fa9edb4c4920a92c62848b54b69ef8ae))
* make sure that all the events could be received by event channel ([24b84fb](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/24b84fbf273090f53a4d67020888318a04e65dc3))
* method to compare the checkpoints ([7fa428d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/7fa428de8d9586e031ff7912eb4a829dcb18cff0)), closes [#flato-3123](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3123)
* **metrics.go:** replace histogram with summary to record actual txsPerBlock metrics ([03668c0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/03668c002bc07ec562136b09405ac240bf16c7bd))
* modify log level ([b4ae21d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b4ae21de4ad7c68ca9ddef3542a3af269afb040e))
* modify the log-printing, hash -> id ([a31cb2e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a31cb2e9cc3aeb64aea5b4e76fda95d736876bc0))
* modify the process of config-change ([5336783](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5336783501592a54dbaf54268de94e9963cd9aa4)), closes [#flato-2647](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2647)
* modity the order to stop rbft core ([59f425f](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/59f425f1c8986cb820c795127b1f8670129eb160)), closes [#flato-2996](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2996)
* new node should close new-node target when it has found a stable epoch info ([b3495a5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b3495a591620a24887935d54a770368629c77fa6)), closes [#flato-3383](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3383)
* **node_mgr.go:** fix logger ([0c68945](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/0c68945082592e997e5a75e1b1874a1b6fc65853))
* **node.go:** don't reject stateUpdated event even if the given Applied is lower than current Applie ([57a4954](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/57a4954df6aa5a06f94c6051f300375eba9202ad)), closes [#flato-698](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-698)
* **node.go:** modify the log level when received an unexpected ServiceState ([5a4619d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5a4619dc81304bbb7a04ecabb98f7007d5d3f364))
* post checkpoint digest in filter event ([ad77590](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/ad77590b30fa262fffb4adad15ffc360acb2f9d2))
* propose view-change if we have found tx with mis-matched transaction from leader ([103cd6e](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/103cd6ef657bc536701d18e0f21ba804fc233b6c)), closes [#flato-3802](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3802)
* **rbft_impl.go/recovery_mgr.go:** don't stop recoveryRestartTimer when resetStateForRecovery ([0a20424](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/0a204248d8b03d27ae69a23c73fbbc29a0f6eebe))
* **rbft_impl.go:** clean useless batch cache before stateupdate to avoid inconsistent state of tx's ([da32d15](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/da32d15517b69ed914958baafc5437d3e50160a3))
* **rbft_impl.go:** fix the problem after new node finished state upadte ([c0194e7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/c0194e786ea3e7683e63c1cca99122ad8443f32e)), closes [#flato-1370](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1370)
* **rbft_impl.go:** judge status before primaryResubmitTransactions after stable checkpoint ([feb0ee0](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/feb0ee017abe3a7813caeda701ac422673f4a834)), closes [#flato-2354](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2354)
* **rbft_impl.go:** post StableCheckpoint event after normal movewatermark and stateupdated ([17d55d6](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/17d55d68904b248919b13ca05aa36361703a743e))
* **rbft_impl.go:** priamry resubmit txs when normal checkpoint finished ([80ada6b](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/80ada6b613dfe2e77d0131e52da6fe31aa15b2f1)), closes [#flato-2257](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2257)
* **rbft_impl.go:** reject txs directly when node's pool is full ([e49b508](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/e49b50830030eea49781e84068fc9d702366a877)), closes [#flato-2648](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2648)
* **rbft_impl.go:** remove judgement of newNode when postConfState ([74a2585](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/74a2585dccc04cd3c15833d47e3dbc4a9e052a8f))
* **rbft:** fix flato-1041,input delIndex instead of delID when calling func getDelNV ([a8cc180](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a8cc180664c7699b6f268c490499896fe0aad134)), closes [#1041](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/1041)
* receive the checkpoints from replicas with h>low-watermark ([80acddd](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/80acddd676ecabe8fce59f79efe2d1a70ceb1458)), closes [#flato-2987](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2987)
* **recovery_mgr.go:** restore pool in resetStateForRecovery when single node recovery from abnormal ([b42b32a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b42b32a2309ae119c351e9b1c38c32aefdbac015)), closes [#flato-2308](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2308)
* **recovery:** do not init recovery when we receive a notification from primary ([47e6df2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/47e6df2e0f7db2c06e7b0180e270307d6aec523e)), closes [#flato-1859](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1859)
* **recovery:** init recovery when we received a notification from primary with a larger view than se ([09bc208](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/09bc208484986e5606ac43dc15323b3e985f276d)), closes [#flato-1859](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1859)
* **recovery:** update the process of recovery and state-update ([5847ffb](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5847ffb32a835f509056e78645d228157a3d0f9b)), closes [#flato-2358](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2358)
* reject create batch when node is not primary ([6c30ebd](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/6c30ebdf4629d6accfb0d0ef9003c9b4c1102dbe))
* remove batches smaller than initial checkpoint in recovery ([7c024f4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/7c024f4d18c33ed5a1422a10d47a890841e2b7fd)), closes [#flato-2644](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2644)
* remove Extra field from ServiceState as it's user's responsibility to realize stateUpdate. ([5f03174](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5f03174a4aa3ef0ea02a1b31b5882e87bac595a3))
* remove goroutine when we post message to resolve oom ([886af16](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/886af166f7e821326bdac224adb62a5fdd6db30b)), closes [#flato-1794](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1794)
* remove status request channel to avoid blocking status request with normal consensus message ([69e4cd5](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/69e4cd5ade443f20472aba211fc4db73462f82ea))
* remove useless files about gitlab ci ([918f23c](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/918f23cf10096d443f35fee5347a692dddd4bd8d))
* replace flato with flato-event to get correct Transaction structure. ([d06e36d](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d06e36d9340396d2c141db369f47d7356543fd1c))
* reset cacheBatch and relates metrics before state update ([71a1e36](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/71a1e36ebbc16a5c5d1ad57321cb7b3d2ff0a37d)), closes [#flato-2502](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2502)
* reset local checkpoint signature after epoch change in case of cert replace ([85a00a3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/85a00a3beeef5734efd374882b5c000f9d9d6e21))
* Reset txpool with some saving batches rather than restore pool after single recovery ([c81fedb](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/c81fedbf17a6efaf46c959019e92fdcd3010b9b4))
* save useful batches before state update to avoid infinite vc caused by missing request batch ([a06f89b](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/a06f89bf7624d86820f7bd2a948e98d796650eb7)), closes [#flato-3334](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3334)
* **SendFilterEvent:** send recovery finished event whenever recovery has finished ([8245e9f](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/8245e9f2b01af6859d488dacb1db14d9123f335f)), closes [#flato-1736](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1736)
* set poll full status whenever add/remove txs from pool ([7645de7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/7645de71c83ee386e6dbb39cdb4ff6130c0a33b8))
* start new view timer to ensure consensus progress ([75844ac](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/75844ac4e55d39c678f9fac86db6177d039f9f1c)), closes [#flato-2706](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2706)
* stop batch timer when node cannot create batch after batch timeout ([1893ca4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/1893ca443c3cb6a2c11e410b3e9516773a971939))
* stop namespace when find self not in router ([07f9014](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/07f901412b667c9db181fc25ba2db154ca98ac52))
* stop new view timer after finish vc/recovery ([2a4064a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/2a4064afb6318126c2fa5bd7e764663bcf4184e4))
* synchronously modify routers after add/delete node ([f1bed3a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f1bed3ad73853623c44b48f36e8ebe9b948bcb9a)), closes [#flato-1060](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1060)
* the concurrent problem for recovery and sync state ([b77cbc7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b77cbc7a2766d89ec6a05463062c0b30f131c6e2)), closes [#flato-3064](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3064)
* the new node could receive notification messages if its epoch larger than 0 ([6878052](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/6878052949f3544159863d2c6a316129f1737a30)), closes [#flato-3383](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3383)
* the process to stop node ([57f7eb7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/57f7eb75adffea120a9b33f209c460c18ba24f47)), closes [#flato-3135](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3135)
* turn into epoch after chain epoch has been changed ([76c6777](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/76c677768a23c8d0c21a6c0fcc26ab75b85c02aa))
* **unit_test:** fix unit test fetch missing ([e109d69](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/e109d692614cbf66ae98f22a05b3d82a106b0c00))
* update high target every time expired ([cc4c0e3](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cc4c0e3c735ead7b21d71881d021c65dfb0f2e62)), closes [#flato-3711](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3711)
* update the process to sync epoch ([cef9c00](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cef9c00ad529e3ebf53070cf1ecf882604b51afd)), closes [#flato-3050](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3050)
* **updateN:** change the place where NodeMgrUpdatedEvent return when updateN ([e23de24](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/e23de2447f83bf4c1fd888e28b6bc6a918731e90)), closes [#flato-1060](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-1060)
* use remote consistent checkpoint to generate signed checkpoint after state update ([1b69be6](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/1b69be604e153a29261de9fe9c07aa8926d81c72)), closes [#flato-4175](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-4175)
* **view-change:** don't send view-change in some situation ([5ace2f7](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5ace2f752f79bbb1e9cb33bef0c0f97dbdbf6259))
* **viewchange_mgr.go:** delete old neweView msg when received a new valid newView msg ([5181fb2](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/5181fb21d9ddff62e165182c3385d81f45b248ac))
* **viewchange_mgr.go:** node in recovery can jump into viewChange if it find newView ([3ad2909](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/3ad2909d7168402a58472f4d2059df8c092d805a))
* **viewchange_mgr.go:** put un-executed batch into outstandingReqBatches when processNewView to trig ([d03f492](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/d03f492904aff08f6a02a7f132b268f3bd438cf4)), closes [#flato-2514](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2514)
* **viewchange_mgr.go:** reject viewchange/recovery when node is in state transfer ([1397eab](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/1397eabcb212e05976c7d71766f3d4e026c813fc))
* **viewchange.go:** manage the config-change in viewchange ([02bfd98](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/02bfd986b20165421463058dbd3bf8b86f8ac093)), closes [#flato-2576](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2576)
* **viewchange:** record prePreparedTime when construct prePrepare in processNewView ([cfdb86a](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/cfdb86a643300ccc2afa56f05dd9a24f9fcc458f))
* we need extra process for view change when we finished state update ([f5392cb](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/f5392cb6c1f4e552615812854fee04c719df1c25)), closes [#flato-3235](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3235)
* we need to persist checkpoint in state-updated ([b685ce4](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/b685ce43b0ea80fbfa04680598bc5b31f8b71282)), closes [#flato-2843](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-2843)
* we need to start high watermark for abnormal pre-prepare ([c67af06](/git.hyperchain.cn/hyperchain/go-hpc-rbft/commit/c67af06c928fe62dfbacc06adc35c11b759451bf)), closes [#flato-3119](/git.hyperchain.cn/hyperchain/go-hpc-rbft/issues/flato-3119)

<a name="0.3.0-1"></a>
# [0.3.0-1](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.3.0...v0.3.0-1) (2022-06-01)


### Bug Fixes

* decrease epoch when generate checkpoint with a low chain height ([7577e85](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/7577e85)), closes [#flato-4741](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-4741)



<a name="0.3.0"></a>
# [0.3.0](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.47...v0.3.0) (2022-05-30)


### Features

* **epoch:** #flato-4506, adapt verifiable epoch change and state update ([78ccfeb](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/78ccfeb)), closes [#flato-4506](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-4506)



<a name="0.2.47"></a>
## [0.2.47](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.46...v0.2.47) (2022-05-25)


### Features

* **hash:** #flato-4540, transaction hash optimization ([6e05a20](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/6e05a20)), closes [#flato-4540](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-4540)



<a name="0.2.46"></a>
## [0.2.46](http://git.hyperchain.cn/ultramesh/flato-rbft/compare/v0.2.45...v0.2.46) (2022-04-21)


### Bug Fixes

* fix some logger and print ([1d2ae60](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/1d2ae60))


### Features

* **go.mod:** update go version to 1.17 & add go.sum ([afed51a](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/afed51a))
* **noxbft:** #flato-4331, adapt hotstuff ([577f2de](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/577f2de)), closes [#flato-4331](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-4331)



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

* **rbft_impl.go:** batch add requests into txpool to improve the performance of look up bloom_filter ([0c367b2](http://git.hyperchain.cn/ultramesh/flato-rbft/commits/0c367b2)), closes [#flato-3340](http://git.hyperchain.cn/ultramesh/flato-rbft/issues/flato-3340)



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
