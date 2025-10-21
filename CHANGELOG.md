# Changelog

## [5.0.0](https://github.com/mykso/myks/compare/v4.11.4...v5.0.0) (2025-10-20)


### ⚠ BREAKING CHANGES

* **cmd:** replace `all`, `sync`, and `render` commands with a single `render` command ([#570](https://github.com/mykso/myks/issues/570))

### Features

* add config-in-root option that sets root-dir to the config file location ([40ffc88](https://github.com/mykso/myks/commit/40ffc88eeb84d25ade452c75b9db806c368118a5))
* add kbld support ([#626](https://github.com/mykso/myks/issues/626)) ([40ffc88](https://github.com/mykso/myks/commit/40ffc88eeb84d25ade452c75b9db806c368118a5))
* **cmd:** replace `all`, `sync`, and `render` commands with a single `render` command ([#570](https://github.com/mykso/myks/issues/570)) ([40ffc88](https://github.com/mykso/myks/commit/40ffc88eeb84d25ade452c75b9db806c368118a5))


### Bug Fixes

* **deps:** update module golang.org/x/term to v0.36.0 ([#624](https://github.com/mykso/myks/issues/624)) ([f3dff86](https://github.com/mykso/myks/commit/f3dff866e2370811f4779b031e83f7226353e8f5))

## [4.11.4](https://github.com/mykso/myks/compare/v4.11.3...v4.11.4) (2025-09-22)


### Bug Fixes

* **deps:** update golang dependencies (non-major) ([#605](https://github.com/mykso/myks/issues/605)) ([8c17243](https://github.com/mykso/myks/commit/8c1724339e2160ee4196d33bb72c94c32ebe3f06))

## [4.11.3](https://github.com/mykso/myks/compare/v4.11.2...v4.11.3) (2025-09-08)


### Bug Fixes

* **deps:** update golang dependencies (non-major) ([#595](https://github.com/mykso/myks/issues/595)) ([367ec2f](https://github.com/mykso/myks/commit/367ec2fa47d28d72340ddc4c3e0698b824d98726))
* **deps:** update module github.com/alecthomas/chroma/v2 to v2.20.0 ([#575](https://github.com/mykso/myks/issues/575)) ([06a55bc](https://github.com/mykso/myks/commit/06a55bc19739d3cbd0fad0323cd2ef78c6ffe3c2))
* **deps:** update module github.com/stretchr/testify to v1.11.0 ([#591](https://github.com/mykso/myks/issues/591)) ([6b1c572](https://github.com/mykso/myks/commit/6b1c572677c2facfc6472b62bbbd4173a16c7c1d))
* **deps:** update module golang.org/x/term to v0.33.0 ([#558](https://github.com/mykso/myks/issues/558)) ([b07a3ba](https://github.com/mykso/myks/commit/b07a3ba178373f6cf7ea231a61a87de5255754d2))
* **deps:** update module golang.org/x/term to v0.34.0 ([#584](https://github.com/mykso/myks/issues/584)) ([b1a3ad8](https://github.com/mykso/myks/commit/b1a3ad8dfed635997c95ab2b32d11d0081a19a90))
* warn on orphan Helm configs and values files ([#572](https://github.com/mykso/myks/issues/572)) ([2bb4f13](https://github.com/mykso/myks/commit/2bb4f13f820ec49027657d0a3a5ad4055087befe))

## [4.11.2](https://github.com/mykso/myks/compare/v4.11.1...v4.11.2) (2025-07-11)


### Bug Fixes

* **deps:** update module golang.org/x/sync to v0.16.0 ([#554](https://github.com/mykso/myks/issues/554)) ([e9c06d3](https://github.com/mykso/myks/commit/e9c06d301af15f066f2b8f815b1af45fa6675c48))
* **smart-mode:** fix environment-specific prototype change detection ([#555](https://github.com/mykso/myks/issues/555)) ([7067392](https://github.com/mykso/myks/commit/70673928ab2b21ce9bd40db4c19c6c63a42bc801))

## [4.11.1](https://github.com/mykso/myks/compare/v4.11.0...v4.11.1) (2025-07-09)


### Bug Fixes

* **deps:** update module github.com/alecthomas/chroma/v2 to v2.19.0 ([#549](https://github.com/mykso/myks/issues/549)) ([3c360c9](https://github.com/mykso/myks/commit/3c360c9aa34dcaa02779f128b8181c35b9293449))
* **smart-mode:** fix env path matching ([#551](https://github.com/mykso/myks/issues/551)) ([49d1423](https://github.com/mykso/myks/commit/49d14233f685b4fe3eb238c8496b540fc634cc1c))

## [4.11.0](https://github.com/mykso/myks/compare/v4.10.0...v4.11.0) (2025-07-05)


### Features

* support global helm values ([#547](https://github.com/mykso/myks/issues/547)) ([67e1863](https://github.com/mykso/myks/commit/67e1863ad9e7160a8d82350c1fa1809c0d152688))

## [4.10.0](https://github.com/mykso/myks/compare/v4.9.0...v4.10.0) (2025-07-04)


### Features

* provide chart name in myks context data ([#545](https://github.com/mykso/myks/issues/545)) ([a447073](https://github.com/mykso/myks/commit/a447073b0a0f0b6fd731a5e781a25daa14451345))

## [4.9.0](https://github.com/mykso/myks/compare/v4.8.5...v4.9.0) (2025-06-20)


### Features

* cleanup rendered manifests per application ([#538](https://github.com/mykso/myks/issues/538)) ([e33f76d](https://github.com/mykso/myks/commit/e33f76d2635fd1df1591a430ee095a6f4c60cf55))


### Bug Fixes

* **smart-mode:** detection of changes in rendered ([#536](https://github.com/mykso/myks/issues/536)) ([2a78684](https://github.com/mykso/myks/commit/2a78684e26617eb663215d42093e7ef42e85005e))

## [4.8.5](https://github.com/mykso/myks/compare/v4.8.4...v4.8.5) (2025-06-20)


### Bug Fixes

* **deps:** update github.com/cppforlife/go-cli-ui digest to 47874c9 ([#530](https://github.com/mykso/myks/issues/530)) ([344a30c](https://github.com/mykso/myks/commit/344a30c7f73c9936c82b077514b9cebeba2df6d5))


### Reverts

* git-auto-commit-action to v5.2.0 ([#535](https://github.com/mykso/myks/issues/535)) ([b54667d](https://github.com/mykso/myks/commit/b54667de03ad00d1baebc634a97f5a8f5d4e8582))

## [4.8.4](https://github.com/mykso/myks/compare/v4.8.3...v4.8.4) (2025-06-07)


### Bug Fixes

* consider changes to the rendered directory in the smart mode ([#525](https://github.com/mykso/myks/issues/525)) ([9eda7d0](https://github.com/mykso/myks/commit/9eda7d0001b00dd0677a853dd8e368e6e8f5f1ea))
* **deps:** update module golang.org/x/sync to v0.15.0 ([#527](https://github.com/mykso/myks/issues/527)) ([56d7fcf](https://github.com/mykso/myks/commit/56d7fcfbad8671c88b39f694292019d6a49b344d))

## [4.8.3](https://github.com/mykso/myks/compare/v4.8.2...v4.8.3) (2025-05-30)


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.44.0 ([#515](https://github.com/mykso/myks/issues/515)) ([a9f0471](https://github.com/mykso/myks/commit/a9f04712493a2c88a0985a7b8ce591956f525fd6))
* **deps:** update module github.com/alecthomas/chroma/v2 to v2.18.0 ([#519](https://github.com/mykso/myks/issues/519)) ([9916ba7](https://github.com/mykso/myks/commit/9916ba7324d438b2c9d3ed80d01a40237e3341db))
* **deps:** update module golang.org/x/term to v0.32.0 ([#511](https://github.com/mykso/myks/issues/511)) ([7f321de](https://github.com/mykso/myks/commit/7f321de2bd7d03ef71113ce2de32a796ede78e3c))
* ensure .myks.yaml config is read before plugin initialization ([#522](https://github.com/mykso/myks/issues/522)) ([7d2f706](https://github.com/mykso/myks/commit/7d2f70651b41c294916494c58ca4d3fead9e1be7))

## [4.8.2](https://github.com/mykso/myks/compare/v4.8.1...v4.8.2) (2025-05-09)


### Bug Fixes

* **deps:** update golang dependencies (non-major) ([#505](https://github.com/mykso/myks/issues/505)) ([f3320e8](https://github.com/mykso/myks/commit/f3320e87aabace0ca1d8406729922954591be0c0))
* **deps:** update module carvel.dev/ytt to v0.51.2 ([#502](https://github.com/mykso/myks/issues/502)) ([846482c](https://github.com/mykso/myks/commit/846482c55163190362733fc0a0a191f4551eed5b))
* **deps:** update module carvel.dev/ytt to v0.52.0 ([#509](https://github.com/mykso/myks/issues/509)) ([721dda4](https://github.com/mykso/myks/commit/721dda4e21e8f7e0549e7078ac747910f9a2fba8))
* **deps:** update module github.com/alecthomas/chroma/v2 to v2.17.0 ([#497](https://github.com/mykso/myks/issues/497)) ([a83ab5c](https://github.com/mykso/myks/commit/a83ab5c48afd890b28cba3bcb4b9a5559c093239))
* typo in cleanup help message ([c70f949](https://github.com/mykso/myks/commit/c70f949c6ec6afe83d18ba443b20662d865677e4))

## [4.8.1](https://github.com/mykso/myks/compare/v4.8.0...v4.8.1) (2025-04-16)


### Bug Fixes

* allow prototypes without vendir configuration ([#490](https://github.com/mykso/myks/issues/490)) ([071b780](https://github.com/mykso/myks/commit/071b780b7d30a624dfd4734038b57884366dab53))
* **deps:** update module github.com/hashicorp/go-version to v1.7.0 ([#474](https://github.com/mykso/myks/issues/474)) ([3d1d30d](https://github.com/mykso/myks/commit/3d1d30d3f96fb729c8214db7e9801e5a140e5367))
* **deps:** update module golang.org/x/sync to v0.13.0 ([#485](https://github.com/mykso/myks/issues/485)) ([ef3c5ab](https://github.com/mykso/myks/commit/ef3c5abf1aa7d615330015246ae3183fcd42b0ce))
* **deps:** update module golang.org/x/term to v0.31.0 ([#487](https://github.com/mykso/myks/issues/487)) ([bb21f15](https://github.com/mykso/myks/commit/bb21f15ad773f2c41a986b1e7c74fe5bebb5c50b))

## [4.8.0](https://github.com/mykso/myks/compare/v4.7.0...v4.8.0) (2025-04-04)


### Features

* add min-version check and config option ([#473](https://github.com/mykso/myks/issues/473)) ([5a15a9d](https://github.com/mykso/myks/commit/5a15a9da126c019f22304dd83e55a3248e41143e))


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.43.2 ([#470](https://github.com/mykso/myks/issues/470)) ([ad63b7e](https://github.com/mykso/myks/commit/ad63b7ebbff478789449ba0c7e85b49580fdad8b))
* **deps:** update module github.com/alecthomas/chroma/v2 to v2.16.0 ([#471](https://github.com/mykso/myks/issues/471)) ([fd70c49](https://github.com/mykso/myks/commit/fd70c49452b1ca91c0fd9350b408b805c0ddb7a1))

## [4.7.0](https://github.com/mykso/myks/compare/v4.6.1...v4.7.0) (2025-04-01)


### Features

* add common vendir configuration directory ([#465](https://github.com/mykso/myks/issues/465)) ([959dec1](https://github.com/mykso/myks/commit/959dec11314eae6afff537319ba4cb584b9bf09b))

## [4.6.1](https://github.com/mykso/myks/compare/v4.6.0...v4.6.1) (2025-03-28)


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.43.1 ([#452](https://github.com/mykso/myks/issues/452)) ([e9417b0](https://github.com/mykso/myks/commit/e9417b08015c7aace2b597bc97a3cf011b98dc8b))
* **deps:** update module github.com/rs/zerolog to v1.34.0 ([#445](https://github.com/mykso/myks/issues/445)) ([c3b3d0b](https://github.com/mykso/myks/commit/c3b3d0b82d253c14a8833d4622a53cd643a8676a))
* **deps:** update module github.com/spf13/viper to v1.20.0 ([#426](https://github.com/mykso/myks/issues/426)) ([52bb724](https://github.com/mykso/myks/commit/52bb72478d3f0d755abe2e7e65b00c97b11e980d))
* **deps:** update module github.com/spf13/viper to v1.20.1 ([#448](https://github.com/mykso/myks/issues/448)) ([08a2061](https://github.com/mykso/myks/commit/08a2061378358ea5a765b11fc4427c3a9421a786))
* **security:** fix G404, G204, and add error handling to hashString ([9a59e2d](https://github.com/mykso/myks/commit/9a59e2dae931f18b5deafecf9b3c147e70ea8d20))
* **smart-mode:** detect renames ([#450](https://github.com/mykso/myks/issues/450)) ([25455eb](https://github.com/mykso/myks/commit/25455eb5ce6ff4decdbfe75f471d2164379716c5))

## [4.6.0](https://github.com/mykso/myks/compare/v4.5.1...v4.6.0) (2025-03-12)


### Features

* extend vendir step with proto, env and app data values ([#422](https://github.com/mykso/myks/issues/422)) ([3456e0c](https://github.com/mykso/myks/commit/3456e0c72fa47481129a70c08fd03a5ae362bf21))


### Bug Fixes

* **deps:** update module golang.org/x/sync to v0.12.0 ([#414](https://github.com/mykso/myks/issues/414)) ([a6c5409](https://github.com/mykso/myks/commit/a6c5409148a595c4c23617b6cb5951f24dc50f03))
* **deps:** update module golang.org/x/term to v0.30.0 ([#415](https://github.com/mykso/myks/issues/415)) ([946be2c](https://github.com/mykso/myks/commit/946be2c70cef27b1013f1dff73ccbc28e611485c))

## [4.5.1](https://github.com/mykso/myks/compare/v4.5.0...v4.5.1) (2025-03-04)


### Bug Fixes

* revert "feat: specify "any" type for .application in schema ([#399](https://github.com/mykso/myks/issues/399))" ([#411](https://github.com/mykso/myks/issues/411)) ([b6bbd59](https://github.com/mykso/myks/commit/b6bbd59ce44fb55aae9694f8b67960fd5c53d397))

## [4.5.0](https://github.com/mykso/myks/compare/v4.4.2...v4.5.0) (2025-02-28)


### Features

* specify "any" type for .application in schema ([#399](https://github.com/mykso/myks/issues/399)) ([ddcf2d4](https://github.com/mykso/myks/commit/ddcf2d41f4e3af282cbf23be9123c493d80086e1))


### Bug Fixes

* **deps:** update module github.com/spf13/cobra to v1.9.1 ([#405](https://github.com/mykso/myks/issues/405)) ([baf77bd](https://github.com/mykso/myks/commit/baf77bd275fef89a1d37ab186b93985741060d52))
* **deps:** update module golang.org/x/sync to v0.11.0 ([#393](https://github.com/mykso/myks/issues/393)) ([0afc509](https://github.com/mykso/myks/commit/0afc5094e986823160e66392b111988b0d1ba910))
* **deps:** update module golang.org/x/term to v0.29.0 ([#394](https://github.com/mykso/myks/issues/394)) ([f3664f1](https://github.com/mykso/myks/commit/f3664f1d8a961d06f8fcdde4e447e27abb0d66ac))

## [4.4.2](https://github.com/mykso/myks/compare/v4.4.1...v4.4.2) (2025-02-02)


### Bug Fixes

* correctly assign apps to nested envs ([#389](https://github.com/mykso/myks/issues/389)) ([39f185d](https://github.com/mykso/myks/commit/39f185d881db3e24729e77e60441baf537e97047))

## [4.4.1](https://github.com/mykso/myks/compare/v4.4.0...v4.4.1) (2025-01-31)


### Bug Fixes

* **release:** testing latest changes ([d441782](https://github.com/mykso/myks/commit/d441782b10e99711de3624cb23bc3821982a037f))

## [4.4.0](https://github.com/mykso/myks/compare/v4.3.2...v4.4.0) (2025-01-28)


### Features

* add argocd.project.enabled option ([753ad11](https://github.com/mykso/myks/commit/753ad1112e9331cec7ae2b976a01f10a86add058))


### Bug Fixes

* skip creation of empty ArgoCD environment file ([dfcb78b](https://github.com/mykso/myks/commit/dfcb78b7f87d475e94c02c4008b8da709402636d))
* sort ArgoCD Application YAML ([600cfa9](https://github.com/mykso/myks/commit/600cfa9feda23c5455db799ff917a6a96e253d76))
* use argocd.project.name if set ([8e0fabd](https://github.com/mykso/myks/commit/8e0fabd4b107af4cffa733fce9463cb6da34a7b9))

## [4.3.2](https://github.com/mykso/myks/compare/v4.3.1...v4.3.2) (2025-01-27)


### Bug Fixes

* ensure buildDependencies is nullable ([#378](https://github.com/mykso/myks/issues/378)) ([4e65a3e](https://github.com/mykso/myks/commit/4e65a3ea59ec2429b18b0ae1434f514ae23c9f09))

## [4.3.1](https://github.com/mykso/myks/compare/v4.3.0...v4.3.1) (2025-01-23)


### Bug Fixes

* **deps:** update module github.com/alecthomas/chroma/v2 to v2.15.0 ([#370](https://github.com/mykso/myks/issues/370)) ([1916955](https://github.com/mykso/myks/commit/19169557ed40f7dc561dbd52db5d518b7e4a4d01))
* **deps:** update module golang.org/x/term to v0.28.0 ([#368](https://github.com/mykso/myks/issues/368)) ([cb36943](https://github.com/mykso/myks/commit/cb369437cf77cf4bb939a72115fa79e4c9e7e247))
* make per-chart helm options nullable ([#374](https://github.com/mykso/myks/issues/374)) ([4fd9713](https://github.com/mykso/myks/commit/4fd9713374519dec6593808b325480b4e7655dff))

## [4.3.0](https://github.com/mykso/myks/compare/v4.2.6...v4.3.0) (2025-01-01)


### Features

* allow multiple files for app and env-data on each level ([#366](https://github.com/mykso/myks/issues/366)) ([3b2f68f](https://github.com/mykso/myks/commit/3b2f68f382a32c18297dc9b612f21a2a4c6cb250))

## [4.2.6](https://github.com/mykso/myks/compare/v4.2.5...v4.2.6) (2024-12-27)


### Bug Fixes

* **deps:** update module carvel.dev/ytt to v0.51.1 ([#358](https://github.com/mykso/myks/issues/358)) ([e339a90](https://github.com/mykso/myks/commit/e339a90bf6fb0f38b883d44cfd9515e206f31926))
* **docs:** overhaul help messages ([#362](https://github.com/mykso/myks/issues/362)) ([aa1f941](https://github.com/mykso/myks/commit/aa1f941d13dc341ec5f58c6e1f1a7cc684ff3a6e))
* skip ArgoCD Application plugin if not set ([#365](https://github.com/mykso/myks/issues/365)) ([302667e](https://github.com/mykso/myks/commit/302667e607d5b7730b1dd28d5aca2cd584de53a8))

## [4.2.5](https://github.com/mykso/myks/compare/v4.2.4...v4.2.5) (2024-12-10)


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.43.0 ([#357](https://github.com/mykso/myks/issues/357)) ([ebf51cd](https://github.com/mykso/myks/commit/ebf51cdc5ed433a3440e0c8082648e7295a198a6))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#351](https://github.com/mykso/myks/issues/351)) ([a47b2ed](https://github.com/mykso/myks/commit/a47b2ed3fa1d77d49153a47246c5e09704919514))
* **deps:** update module golang.org/x/sync to v0.10.0 ([#354](https://github.com/mykso/myks/issues/354)) ([7ed5e20](https://github.com/mykso/myks/commit/7ed5e20a8f01ba373ccd70c6761f4d77a9d1cd1a))
* **deps:** update module golang.org/x/sync to v0.9.0 ([#345](https://github.com/mykso/myks/issues/345)) ([3abe265](https://github.com/mykso/myks/commit/3abe2650a586f83dc5417f7a63107bf0dee10743))
* **deps:** update module golang.org/x/term to v0.26.0 ([#346](https://github.com/mykso/myks/issues/346)) ([edd9944](https://github.com/mykso/myks/commit/edd994416da29ba288dbf79a788aff2c6f317cae))
* **deps:** update module golang.org/x/term to v0.27.0 ([#355](https://github.com/mykso/myks/issues/355)) ([78fe9b1](https://github.com/mykso/myks/commit/78fe9b1cb6dd17004e54cbfd81ee19fa42337fdb))

## [4.2.4](https://github.com/mykso/myks/compare/v4.2.3...v4.2.4) (2024-11-07)


### Bug Fixes

* **deps:** update module carvel.dev/ytt to v0.51.0 ([#342](https://github.com/mykso/myks/issues/342)) ([836e1fd](https://github.com/mykso/myks/commit/836e1fda537aefb1d66a7e0c3e5146529df9d09a))
* **deps:** update module golang.org/x/term to v0.25.0 ([#338](https://github.com/mykso/myks/issues/338)) ([b94de99](https://github.com/mykso/myks/commit/b94de998a349113a66feab8793ea8906d9449374))

## [4.2.3](https://github.com/mykso/myks/compare/v4.2.2...v4.2.3) (2024-09-11)


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.41.0 ([#325](https://github.com/mykso/myks/issues/325)) ([e7e3d51](https://github.com/mykso/myks/commit/e7e3d51ff432baaad51ec210f6cca5a992632d31))
* **deps:** update module carvel.dev/vendir to v0.41.1 ([#332](https://github.com/mykso/myks/issues/332)) ([04b6fc4](https://github.com/mykso/myks/commit/04b6fc45ec5fb60a3bc753ed12649afc53939e7d))
* **deps:** update module carvel.dev/vendir to v0.42.0 ([#334](https://github.com/mykso/myks/issues/334)) ([1149696](https://github.com/mykso/myks/commit/1149696e76e7e89c7a883556ada8dc3299d2c191))
* **deps:** update module github.com/creasty/defaults to v1.8.0 ([#329](https://github.com/mykso/myks/issues/329)) ([1032aed](https://github.com/mykso/myks/commit/1032aeda8ffa97f7bb0655423ebf2bce580954e8))
* **deps:** update module golang.org/x/sync to v0.8.0 ([#327](https://github.com/mykso/myks/issues/327)) ([9b8f7e3](https://github.com/mykso/myks/commit/9b8f7e3dd8ecd0f7ade67eaa4294567e5e915b31))
* **deps:** update module golang.org/x/term to v0.23.0 ([#328](https://github.com/mykso/myks/issues/328)) ([2de0605](https://github.com/mykso/myks/commit/2de0605a87fd4d43bc4782d22046f2dc58ba5137))
* **deps:** update module golang.org/x/term to v0.24.0 ([#333](https://github.com/mykso/myks/issues/333)) ([0c879da](https://github.com/mykso/myks/commit/0c879da0a09dae3685ea2240f9f0c4df0740b88e))

## [4.2.2](https://github.com/mykso/myks/compare/v4.2.1...v4.2.2) (2024-07-16)


### Bug Fixes

* **deps:** update module carvel.dev/ytt to v0.50.0 ([#324](https://github.com/mykso/myks/issues/324)) ([588d600](https://github.com/mykso/myks/commit/588d6004274d9bc9a7d93692a499d24ac5b6221d))
* **deps:** update module golang.org/x/term to v0.22.0 ([#321](https://github.com/mykso/myks/issues/321)) ([2f63e3c](https://github.com/mykso/myks/commit/2f63e3c965e12d7df59651d8bedb04ad25893113))

## [4.2.1](https://github.com/mykso/myks/compare/v4.2.0...v4.2.1) (2024-06-17)


### Bug Fixes

* **deps:** update module github.com/spf13/cobra to v1.8.1 ([#317](https://github.com/mykso/myks/issues/317)) ([8be47e3](https://github.com/mykso/myks/commit/8be47e3b393b1c1ddbc93ce513a55bb0de013f0f))

## [4.2.0](https://github.com/mykso/myks/compare/v4.1.3...v4.2.0) (2024-06-13)


### Features

* per-chart helm configuration ([#310](https://github.com/mykso/myks/issues/310)) ([79004ab](https://github.com/mykso/myks/commit/79004ab871767c1ac184a5e0c2392d8ec2b1e3cc))


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.40.2 ([#313](https://github.com/mykso/myks/issues/313)) ([7e71793](https://github.com/mykso/myks/commit/7e717933e5870e392bb0dd195600bb3c50dac222))
* **deps:** update module carvel.dev/ytt to v0.49.1 ([#314](https://github.com/mykso/myks/issues/314)) ([10825c2](https://github.com/mykso/myks/commit/10825c2db5e62daa9be0415b00ea155e81c167b9))

## [4.1.3](https://github.com/mykso/myks/compare/v4.1.2...v4.1.3) (2024-06-05)


### Bug Fixes

* **deps:** update module github.com/alecthomas/chroma/v2 to v2.14.0 ([#301](https://github.com/mykso/myks/issues/301)) ([fc2ca8a](https://github.com/mykso/myks/commit/fc2ca8a8dbfd20aa5e8f6d3fd567c05f9fcf65e8))
* **deps:** update module github.com/rs/zerolog to v1.33.0 ([#303](https://github.com/mykso/myks/issues/303)) ([9176268](https://github.com/mykso/myks/commit/9176268cc5def5e8ecd52d52bcb97260dff004c7))
* **deps:** update module github.com/spf13/viper to v1.19.0 ([#305](https://github.com/mykso/myks/issues/305)) ([8a6d8b7](https://github.com/mykso/myks/commit/8a6d8b79c95248f4b6aebecf37fefef19294b4c5))
* **deps:** update module golang.org/x/term to v0.21.0 ([#307](https://github.com/mykso/myks/issues/307)) ([5452ab9](https://github.com/mykso/myks/commit/5452ab9204113205d55e5ab9aa6167ab7ae422ec))

## [4.1.2](https://github.com/mykso/myks/compare/v4.1.1...v4.1.2) (2024-05-20)


### Bug Fixes

* adjust the initial .gitignore for the latest changes ([#294](https://github.com/mykso/myks/issues/294)) ([2e1940f](https://github.com/mykso/myks/commit/2e1940f6f0bef24015235971b3c1660a89bb0550))

## [4.1.1](https://github.com/mykso/myks/compare/v4.1.0...v4.1.1) (2024-05-15)


### Bug Fixes

* switch generated data values to schema to lower priority ([#290](https://github.com/mykso/myks/issues/290)) ([8f86a11](https://github.com/mykso/myks/commit/8f86a1169debdccebe1459192df0dd200629038d))

## [4.1.0](https://github.com/mykso/myks/compare/v4.0.1...v4.1.0) (2024-05-15)


### Features

* enchance cleanup command to take care of cache ([#288](https://github.com/mykso/myks/issues/288)) ([da36786](https://github.com/mykso/myks/commit/da3678625d8287c4e972fbc208b5a3e3817ebd15))

## [4.0.1](https://github.com/mykso/myks/compare/v4.0.0...v4.0.1) (2024-05-12)


### Bug Fixes

* empty ytt rendering step output is not an error but warning ([8ba573a](https://github.com/mykso/myks/commit/8ba573a9da6b501ebd969a314f7035124d19e142))
* propagate errors from rendering steps ([ca8f9ca](https://github.com/mykso/myks/commit/ca8f9ca89e8efa6b06d3ce31d734ecf6743995b7))

## [4.0.0](https://github.com/mykso/myks/compare/v3.4.4...v4.0.0) (2024-05-11)

### ℹ Upgrading to 4.0.0

In the new major version we introduced a central cache for external sources downloaded by vendir. With that, we also changed location of most of the myks-managed files. Here is the list of changes (examples are using the file system structure created by the `myks init` command):

* `<env>/_apps/<app>/.myks` and `<env>/_apps/<app>/vendor` directories are moved under the `.myks` directory in the repository root. For example:
  * contents of `envs/mykso/dev/_apps/argocd/.myks` is moved under `.myks/envs/mykso/dev/_apps/argocd`,
  * `envs/mykso/dev/_apps/argocd/vendor` directory is now `.myks/envs/mykso/dev/_apps/argocd/vendor`.
* `vendor` directories now contain links to directories in the central cache instead of files and directories as before.
* The new `.myks/vendir-cache` directory contains cache entries named using (upon availability) vendir contents type, name, version and config hash.

There is nothing required to be done before you can start using the new myks. However, there are a few things to keep in mind:

1. The first run of the new version will download all the sources used in your project, it might take a while.
2. The old files are not cleaned up, so you can easily rollback to the old myks if needed. Otherwise, you have to do remove the files manually:

   ```shell
   # First, inspect what will be removed:
   find envs -name .myks
   # Then remove:
   find envs -name .myks -exec rm -rf {} \;
   # First, inspect what will be removed:
   find envs -name vendor
   # Then remove:
   find envs -name vendor -exec rm -rf {} \;
   ```

### ⚠ BREAKING CHANGES

* central cache with symlinks ([#274](https://github.com/mykso/myks/issues/274))

### Features

* central cache with symlinks ([#274](https://github.com/mykso/myks/issues/274)) ([fd450cd](https://github.com/mykso/myks/commit/fd450cda1ff2cef145ed557a87c06fc25ded9ef2))
* embed ytt ([#272](https://github.com/mykso/myks/issues/272)) ([2520648](https://github.com/mykso/myks/commit/25206487e0fe4cba4eab27e3e315c545fb94fc90))


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.40.1 ([#270](https://github.com/mykso/myks/issues/270)) ([fec8d50](https://github.com/mykso/myks/commit/fec8d507b58cb9088375a0dd412c8f392d74f4da))
* **deps:** update module golang.org/x/term to v0.20.0 ([#278](https://github.com/mykso/myks/issues/278)) ([7074c8e](https://github.com/mykso/myks/commit/7074c8eb0c043f71e1a014b4b2685e94116efa65))
* remove ytt dependency everywhere ([#284](https://github.com/mykso/myks/issues/284)) ([8265abc](https://github.com/mykso/myks/commit/8265abc720174462c05460aabd74b880bb53e1f1))
* ship example prototypes with `lazy` flag enabled ([1f91aa0](https://github.com/mykso/myks/commit/1f91aa01c749900851e87a40fca19bfab70ca490))

## [3.4.4](https://github.com/mykso/myks/compare/v3.4.3...v3.4.4) (2024-04-07)


### Bug Fixes

* **deps:** update golang.org/x/exp digest to c0f41cb ([#264](https://github.com/mykso/myks/issues/264)) ([59981d0](https://github.com/mykso/myks/commit/59981d0215a82e93529daa4a35fed28508d19e46))
* **deps:** update module golang.org/x/sync to v0.7.0 ([#262](https://github.com/mykso/myks/issues/262)) ([d1d231d](https://github.com/mykso/myks/commit/d1d231d40e0542347c95a24c7543d3c3fcbffb17))
* **deps:** update module golang.org/x/term to v0.19.0 ([#263](https://github.com/mykso/myks/issues/263)) ([2e28691](https://github.com/mykso/myks/commit/2e286919fbf542a6ac566ab748c044c07fd5e459))
* **deps:** update, use upstream vendir ([f2a1159](https://github.com/mykso/myks/commit/f2a115979fc2edc2a7ca4aaac08023a75b8a4a35))

## [3.4.3](https://github.com/mykso/myks/compare/v3.4.2...v3.4.3) (2024-03-29)


### Bug Fixes

* **deps:** update golang.org/x/exp digest to a685a6e ([#258](https://github.com/mykso/myks/issues/258)) ([6ea07fe](https://github.com/mykso/myks/commit/6ea07fe5a7475079f0ea19ac33928dbc152b4067))
* **deps:** update golang.org/x/exp digest to a85f2c6 ([#254](https://github.com/mykso/myks/issues/254)) ([ead1322](https://github.com/mykso/myks/commit/ead13221f10166ae70c68e87d5fdfcc33d874fc4))
* **deps:** update golang.org/x/exp digest to c7f7c64 ([#252](https://github.com/mykso/myks/issues/252)) ([b1968c7](https://github.com/mykso/myks/commit/b1968c782fc85752fb00be0ffbd0fd55e853f28f))
* edge case when processing all environments ([2e17117](https://github.com/mykso/myks/commit/2e17117e98555708ae2f8257579c72f045eace91))

## [3.4.2](https://github.com/mykso/myks/compare/v3.4.1...v3.4.2) (2024-03-13)


### Bug Fixes

* **deps:** update golang.org/x/exp digest to 814bf88 ([#247](https://github.com/mykso/myks/issues/247)) ([01a68ce](https://github.com/mykso/myks/commit/01a68ce71b554a3965c9cfd6bb361c454c2bea70))
* **deps:** update module github.com/alecthomas/chroma/v2 to v2.13.0 ([#250](https://github.com/mykso/myks/issues/250)) ([305a16d](https://github.com/mykso/myks/commit/305a16d2e1e591786e510e3a7974394f3db471f8))
* **deps:** update module golang.org/x/term to v0.18.0 ([#249](https://github.com/mykso/myks/issues/249)) ([8043b46](https://github.com/mykso/myks/commit/8043b4651b90e3d13e385724a9a3d14e344e44ce))

## [3.4.1](https://github.com/mykso/myks/compare/v3.4.0...v3.4.1) (2024-02-20)


### Bug Fixes

* **deps:** update module carvel.dev/vendir to v0.40.0 ([#244](https://github.com/mykso/myks/issues/244)) ([5ab69b4](https://github.com/mykso/myks/commit/5ab69b49e156c8f9bcae7f9d0375b3b27163b606))

## [3.4.0](https://github.com/mykso/myks/compare/v3.3.1...v3.4.0) (2024-02-18)


### Features

* **ui:** shell completion for envs and apps ([#240](https://github.com/mykso/myks/issues/240)) ([ef03b96](https://github.com/mykso/myks/commit/ef03b965bb4d7924ab5dc8ce45e0e92423fe0aae))


### Bug Fixes

* **deps:** update golang.org/x/exp digest to 2c58cdc ([#233](https://github.com/mykso/myks/issues/233)) ([e9c329a](https://github.com/mykso/myks/commit/e9c329a49a0f28c4e8d607d20f629d256a3e01c9))
* **deps:** update golang.org/x/exp digest to ec58324 ([#241](https://github.com/mykso/myks/issues/241)) ([c017fc5](https://github.com/mykso/myks/commit/c017fc5ff47ed9cb4ad189e83cc425c78db829ad))
* **deps:** update module github.com/rs/zerolog to v1.32.0 ([#232](https://github.com/mykso/myks/issues/232)) ([b0e0822](https://github.com/mykso/myks/commit/b0e08229818344d66a5ec524f00b036f3d0cf07d))
* **deps:** update module golang.org/x/term to v0.17.0 ([#237](https://github.com/mykso/myks/issues/237)) ([33ad190](https://github.com/mykso/myks/commit/33ad190dd0e13aa5f97ecf97caad179a14246275))

## [3.3.1](https://github.com/mykso/myks/compare/v3.3.0...v3.3.1) (2024-01-22)


### Bug Fixes

* **deps:** update golang.org/x/exp digest to 1b97071 ([#221](https://github.com/mykso/myks/issues/221)) ([d1affb1](https://github.com/mykso/myks/commit/d1affb1aab507b30922f3c618d7133aa69489141))
* **deps:** update module carvel.dev/vendir to v0.39.0 ([#225](https://github.com/mykso/myks/issues/225)) ([3a99a5d](https://github.com/mykso/myks/commit/3a99a5da407ee0f1fb992516e14f554b6079dc27))
* **ui:** provide better error message on multiple vendir configs ([143cdf1](https://github.com/mykso/myks/commit/143cdf1d1eb80b65d1c0dc2bda75abc4f52c368b))

## [3.3.0](https://github.com/mykso/myks/compare/v3.2.2...v3.3.0) (2024-01-13)

### Features

- **ci:** sign checksum file with GPG ([a8673e9](https://github.com/mykso/myks/commit/a8673e99214582aa841b0914d5b1db0104054106))

### Bug Fixes

- **ci:** create release PR with PAT, remove lint and test checks ([16c01bb](https://github.com/mykso/myks/commit/16c01bb200e0383d246c331d9ac55039bae18351))
- **ci:** lint and test on main before creating release PR ([f42dcf6](https://github.com/mykso/myks/commit/f42dcf66b1a37ebb8ffde04166670dafb497f400))

## [3.2.2](https://github.com/mykso/myks/compare/v3.2.1...v3.2.2) (2024-01-12)

### Bug Fixes

- **deps:** update golang.org/x/exp digest to 0dcbfd6 ([#213](https://github.com/mykso/myks/issues/213)) ([dee31a0](https://github.com/mykso/myks/commit/dee31a00e327f8936bb2215cb9ee0bc819a8d6d0))
- **deps:** update golang.org/x/exp digest to db7319d ([#217](https://github.com/mykso/myks/issues/217)) ([69e3a30](https://github.com/mykso/myks/commit/69e3a30129bb142d5eb73c0487771a3cc0e77960))
- don't fail on attempt to clean up non existing files ([#216](https://github.com/mykso/myks/issues/216)) ([5a20896](https://github.com/mykso/myks/commit/5a208965dd2349c27ff4fc3eb0be040ca663be15))

## [3.2.1](https://github.com/mykso/myks/compare/v3.2.0...v3.2.1) (2024-01-10)

### Bug Fixes

- decrease amount of git-related errors and warnings ([#208](https://github.com/mykso/myks/issues/208)) ([6635495](https://github.com/mykso/myks/commit/6635495f8f36c1f8c4ce2c5377e5eb8e88df499d))

# [3.2.0](https://github.com/mykso/myks/compare/v3.1.0...v3.2.0) (2024-01-07)

### Bug Fixes

- **deps:** update github.com/cppforlife/go-cli-ui digest to 9954948 ([#201](https://github.com/mykso/myks/issues/201)) ([9836249](https://github.com/mykso/myks/commit/98362499ffdca43a68e750b88df699837e1e99e4))
- **deps:** update golang.org/x/exp digest to be819d1 ([#204](https://github.com/mykso/myks/issues/204)) ([9928ad1](https://github.com/mykso/myks/commit/9928ad108873b2359b2a7acfb984f6b8fa3e9132))
- **deps:** update module golang.org/x/sync to v0.6.0 ([#205](https://github.com/mykso/myks/issues/205)) ([d3b8ea0](https://github.com/mykso/myks/commit/d3b8ea031edb4131f82da4fb7bc74bf321a5a4e5))
- **deps:** update module golang.org/x/term to v0.16.0 ([#206](https://github.com/mykso/myks/issues/206)) ([002cfe0](https://github.com/mykso/myks/commit/002cfe0a2f197c184481d9b192a3addddcf7b2eb))
- **sync:** allow local paths in vendir config ([#191](https://github.com/mykso/myks/issues/191)) ([73233eb](https://github.com/mykso/myks/commit/73233ebd7327e8c0ea8653362006e3db2102023f))

### Features

- **cleanup:** added dedicated command ([#198](https://github.com/mykso/myks/issues/198)) ([48fa589](https://github.com/mykso/myks/commit/48fa589f94fb0129b58213bf1377cd6fa898acec)), closes [#130](https://github.com/mykso/myks/issues/130)
- **vendir:** embed vendir into myks ([#199](https://github.com/mykso/myks/issues/199)) ([95ecfa8](https://github.com/mykso/myks/commit/95ecfa89ab625a31131579ed1866d62f4cb0ae58))

# [3.1.0](https://github.com/mykso/myks/compare/v3.0.4...v3.1.0) (2023-12-29)

### Bug Fixes

- **deps:** update golang.org/x/exp digest to 02704c9 ([#160](https://github.com/mykso/myks/issues/160)) ([e9fe05e](https://github.com/mykso/myks/commit/e9fe05eb9646d8452e1fd60c0e03216747af73d8))
- **plugins:** improve logging ([#190](https://github.com/mykso/myks/issues/190)) ([ba31a3b](https://github.com/mykso/myks/commit/ba31a3bc4587feb5b07035768dd6bfd5d5718abd))
- **smart_mode:** add static files directory ([#193](https://github.com/mykso/myks/issues/193)) ([3f52709](https://github.com/mykso/myks/commit/3f52709d3c10f5bff1317f2d6bbd1b851c77c4d9))

### Features

- Execute helm dependency build ([#154](https://github.com/mykso/myks/issues/154)) ([c2c0baa](https://github.com/mykso/myks/commit/c2c0baa26f2e3204d1e7f2da7264f44cfdbd1445)), closes [#146](https://github.com/mykso/myks/issues/146)

## [3.0.4](https://github.com/mykso/myks/compare/v3.0.3...v3.0.4) (2023-12-26)

### Bug Fixes

- **deps:** update module github.com/alecthomas/chroma to v2 ([#185](https://github.com/mykso/myks/issues/185)) ([dd2d579](https://github.com/mykso/myks/commit/dd2d5790fbf347825047853a2cd21c73bdd06ee5))

## [3.0.3](https://github.com/mykso/myks/compare/v3.0.2...v3.0.3) (2023-12-25)

### Bug Fixes

- **deps:** update module github.com/alecthomas/chroma to v2 ([#184](https://github.com/mykso/myks/issues/184)) ([6e6eede](https://github.com/mykso/myks/commit/6e6eedee42a91e7cda734f8550012f1bd7a13bd4))
- sync on integration tests ([#177](https://github.com/mykso/myks/issues/177)) ([5d2e636](https://github.com/mykso/myks/commit/5d2e6364c4b2d1fc0dd35e51cd0af4eeacfa0dc8))
- **sync:** create vendor directory if not exists ([#159](https://github.com/mykso/myks/issues/159)) ([fd8e878](https://github.com/mykso/myks/commit/fd8e878900883263d4a67fa01103f411c0b3484d))

## [3.0.2](https://github.com/mykso/myks/compare/v3.0.1...v3.0.2) (2023-12-24)

### Bug Fixes

- **sync:** cover more cases of weird paths in vendir.yml configs ([#158](https://github.com/mykso/myks/issues/158)) ([722ce01](https://github.com/mykso/myks/commit/722ce01d5dc93b1e0437250297581e4e58839111))
- **sync:** trim path separator from vendir directories ([#156](https://github.com/mykso/myks/issues/156)) ([7299eae](https://github.com/mykso/myks/commit/7299eaee427213aa9ed0d929cbb2359c20764168)), closes [#155](https://github.com/mykso/myks/issues/155)

## [3.0.1](https://github.com/mykso/myks/compare/v3.0.0...v3.0.1) (2023-12-22)

### Bug Fixes

- improve logging for plugins ([#153](https://github.com/mykso/myks/issues/153)) ([cd6e41f](https://github.com/mykso/myks/commit/cd6e41f52852bbf0bce45b91c057f718668fb952))

# [3.0.0](https://github.com/mykso/myks/compare/v2.2.0...v3.0.0) (2023-12-22)

### Bug Fixes

- consistent error logging with stderr and offending cmd ([#143](https://github.com/mykso/myks/issues/143)) ([d9ed5ad](https://github.com/mykso/myks/commit/d9ed5ad48dc62715afd3a59d4bc66eaec720cf76))
- **smart mode:** detect changes in untracked files ([#144](https://github.com/mykso/myks/issues/144)) ([524a3c5](https://github.com/mykso/myks/commit/524a3c5c9cedf83147f3c8ce645696f86a6262ce))

### Features

- plugin implementation ([#148](https://github.com/mykso/myks/issues/148)) ([f23a41b](https://github.com/mykso/myks/commit/f23a41b3d0843928540f3e53b06f21ac4ad7abbb))
- **sync:** deprecate sync.useCache and flip its default value ([#150](https://github.com/mykso/myks/issues/150)) ([dcce7fc](https://github.com/mykso/myks/commit/dcce7fc071c6cf5ca387d2fd1a0f00a63117dd95))
- **sync:** remove sync.useCache ([#151](https://github.com/mykso/myks/issues/151)) ([3962b73](https://github.com/mykso/myks/commit/3962b736caadbed54b1f216c70f7cbb4e82e64f6))

### BREAKING CHANGES

- **sync:** users have to remove the sync section from their configurations.

# [2.2.0](https://github.com/mykso/myks/compare/v2.1.1...v2.2.0) (2023-12-11)

### Features

- implement prototype overwrites in the envs tree ([#110](https://github.com/mykso/myks/issues/110)) ([c3e550d](https://github.com/mykso/myks/commit/c3e550d038964d190b5640cd2ecf6bdaf3f9e3e8)), closes [#109](https://github.com/mykso/myks/issues/109)
- include namespace in output file name ([#141](https://github.com/mykso/myks/issues/141)) ([42fa566](https://github.com/mykso/myks/commit/42fa5667dd46083ebf6d0d200c9a6aa91874f54c))
- new plugin for copying static files ([#132](https://github.com/mykso/myks/issues/132)) ([0f7c9dc](https://github.com/mykso/myks/commit/0f7c9dc4854cc2e16d349963282fb11bd941feab))

## [2.1.1](https://github.com/mykso/myks/compare/v2.1.0...v2.1.1) (2023-11-21)

### Bug Fixes

- **clean-up:** Call env cleanup method when executing render and sync ([#125](https://github.com/mykso/myks/issues/125)) ([c7cf621](https://github.com/mykso/myks/commit/c7cf6217df73c7897099dd62a85040a8731f6483)), closes [#100](https://github.com/mykso/myks/issues/100)

# [2.1.0](https://github.com/mykso/myks/compare/v2.0.4...v2.1.0) (2023-10-31)

### Bug Fixes

- **init:** ignore top level .myks folder ([#116](https://github.com/mykso/myks/issues/116)) ([b39eb8d](https://github.com/mykso/myks/commit/b39eb8df0a9cc22524314431b61bb7474608d270))
- **render:** fix panic if metadata.name is not set ([#123](https://github.com/mykso/myks/issues/123)) ([cdf3266](https://github.com/mykso/myks/commit/cdf3266132064b5f3627714199227cd75ed4cfa9))
- **smart-mode:** correct detection of changes in "\_env/argocd" ([#111](https://github.com/mykso/myks/issues/111)) ([ec5dd1d](https://github.com/mykso/myks/commit/ec5dd1d99b7fddf2a452c8b86609775e8222b851))
- **smart-mode:** filter out deleted envs and apps ([93fa2e3](https://github.com/mykso/myks/commit/93fa2e3786250b4e0c305af0923204d2bf1e9341)), closes [#114](https://github.com/mykso/myks/issues/114)

### Features

- add metadata to ytt steps ([#119](https://github.com/mykso/myks/issues/119)) ([6443a20](https://github.com/mykso/myks/commit/6443a2097dc061074a9c6f3ca019a96054c51f4f))
- **global-ytt:** log ytt output on error ([#120](https://github.com/mykso/myks/issues/120)) ([a22f7aa](https://github.com/mykso/myks/commit/a22f7aa0ea9dc27985e906c6f0f5213cddcd6697))
- **ui:** add smart-mode.only-print flag for debugging ([0a6349a](https://github.com/mykso/myks/commit/0a6349a974fca2317f7d51adae337a3dce0dc4c0))

## [2.0.4](https://github.com/mykso/myks/compare/v2.0.3...v2.0.4) (2023-09-29)

### Bug Fixes

- **release:** trigger ([b6dd8bb](https://github.com/mykso/myks/commit/b6dd8bba5458b004c6e58c3365a6e969f2222db2))

## [2.0.3](https://github.com/mykso/myks/compare/v2.0.2...v2.0.3) (2023-09-29)

### Bug Fixes

- **ui:** sanitize environment search paths ([#108](https://github.com/mykso/myks/issues/108)) ([e2262fb](https://github.com/mykso/myks/commit/e2262fb5eb2831f737e723b20b70c5bd0d553462))

## [2.0.2](https://github.com/mykso/myks/compare/v2.0.1...v2.0.2) (2023-09-28)

### Bug Fixes

- **ui:** correctly process ALL environments with a custom set of applications ([b7dd45a](https://github.com/mykso/myks/commit/b7dd45a4163ab31989ca1e30358abd4035cac7b0))
- **ui:** corretly set step file number prefix ([f58d689](https://github.com/mykso/myks/commit/f58d689154373eb25dfc6cbb77526dcc1b244d9a))
- **ui:** render everything if Smart Mode failed ([0a310b4](https://github.com/mykso/myks/commit/0a310b4ff8ba908ca071258c3c2ab923e802523d))
- **ui:** use correct rendering step name in logs ([7cc93ce](https://github.com/mykso/myks/commit/7cc93ce33bd187a9faac348e81af406353cccf46))

## [2.0.1](https://github.com/mykso/myks/compare/v2.0.0...v2.0.1) (2023-09-21)

### Bug Fixes

- move rendereded env-data.yaml to temporary dir ([93426a3](https://github.com/mykso/myks/commit/93426a31e53c052e46ad1e3cae4e248455c26749))
- **ui:** provide more precise information about init errors ([244e8b8](https://github.com/mykso/myks/commit/244e8b8bb8d83d0965cd7745a3aef12b0d2abea5))

# [2.0.0](https://github.com/mykso/myks/compare/v1.2.0...v2.0.0) (2023-09-19)

### Bug Fixes

- Add documentation the myks sync step ([#38](https://github.com/mykso/myks/issues/38)) ([e61a10c](https://github.com/mykso/myks/commit/e61a10ce0ae76667f9c25205954fbef6063fd29e)), closes [#37](https://github.com/mykso/myks/issues/37)
- apply smart mode logic only to supported commands ([#83](https://github.com/mykso/myks/issues/83)) ([2bc754f](https://github.com/mykso/myks/commit/2bc754fb32bf05f030480c42bbfc00ee7119a677))
- argocd source plugin config type in schema ([520156d](https://github.com/mykso/myks/commit/520156d19ed792ef8ac2d14f603c75f53e768cd5))
- cleanup vendir folder ([#90](https://github.com/mykso/myks/issues/90)) ([a20df1a](https://github.com/mykso/myks/commit/a20df1a2181a399dc9a2bc5b728384958eab067a))
- consistent behavior on rendering ALL applications ([#79](https://github.com/mykso/myks/issues/79)) ([2aab516](https://github.com/mykso/myks/commit/2aab51684c48184da4bee3296755e1c393470f58))
- correct sources for the global-ytt rendering step ([#50](https://github.com/mykso/myks/issues/50)) ([5a0e4d7](https://github.com/mykso/myks/commit/5a0e4d73d43803e78ca289efe34113cf63365540))
- create myks data schema file on init and on every run ([#84](https://github.com/mykso/myks/issues/84)) ([976291e](https://github.com/mykso/myks/commit/976291e530ee828249095ff399db5b15953a1efc))
- data values of prototype of argocd app ([b5d7ff9](https://github.com/mykso/myks/commit/b5d7ff9f182ce025a1ee8121d017472b4eb6c238))
- do not fail on absent rendered directory ([eaf1202](https://github.com/mykso/myks/commit/eaf1202056bbae89169e739b2dd8cb6b2a772b3f))
- do not fail without vendir configs ([2f73cda](https://github.com/mykso/myks/commit/2f73cda5a6f473d4ca2741a710ddf32b00db6935))
- do not override ArgoCD defaults set by user ([#74](https://github.com/mykso/myks/issues/74)) ([f2cf4ce](https://github.com/mykso/myks/commit/f2cf4ce5e20e6e492889c28515fe83ab6e7445c4)), closes [#70](https://github.com/mykso/myks/issues/70)
- **docker:** do not build arm64, it is not supported ([3971ae7](https://github.com/mykso/myks/commit/3971ae741cb16848e621ed7211b784dacd4b6211))
- **docker:** specify full image tag ([f3222e5](https://github.com/mykso/myks/commit/f3222e594b7ef5c37f409adac67d8a2e31086906))
- formatting ([fd65f05](https://github.com/mykso/myks/commit/fd65f054df788818c8e50c2585a7db1bce160605))
- generate ArgoCD secret only if enabled ([4b3ed11](https://github.com/mykso/myks/commit/4b3ed113c9a4dae5546b8cd49cd85926d0daff43))
- helm value file merge ([#33](https://github.com/mykso/myks/issues/33)) ([3c9c0ea](https://github.com/mykso/myks/commit/3c9c0eaa7c68c02eed9c5140395a233ffb339e4d)), closes [#32](https://github.com/mykso/myks/issues/32)
- init Globe core attributes earlier ([#85](https://github.com/mykso/myks/issues/85)) ([20c48fd](https://github.com/mykso/myks/commit/20c48fdf24a96eb655e5a06b2ceefe991539aa31))
- log errors during vendir sync ([5dc1b5e](https://github.com/mykso/myks/commit/5dc1b5e4fcd8f3a8b3edc9ac089d2fd068dbad59))
- make render errors appear in the log with full error message ([c325da2](https://github.com/mykso/myks/commit/c325da2b854b7a47c3cf2c5dab383fcb4972740a))
- process map keys instead of values ([3b86a03](https://github.com/mykso/myks/commit/3b86a03c43632b8ea0e25713984ef8dce325321e))
- reduce usage of pointers to cope with race conditions ([#88](https://github.com/mykso/myks/issues/88)) ([d734933](https://github.com/mykso/myks/commit/d734933aa5bd738cc9991b463894eeecd6be1810))
- search in the default envs directory ([ef4a75e](https://github.com/mykso/myks/commit/ef4a75ed192ac7f4be103a5b14e57e3aac327c16))
- skip helm rendering ([80a8eb5](https://github.com/mykso/myks/commit/80a8eb58dc8cbd040a28e310b8ef5d8e4c54c8b7))
- **smart-mode:** detect changes when myks root is in subdirectory ([0522b67](https://github.com/mykso/myks/commit/0522b67b5496d684e36b18269acf656169d63118))
- update data-schema.ytt.yaml according to the latest Myks changes ([5ef9d34](https://github.com/mykso/myks/commit/5ef9d34e035164e4b617be3e4001d1387c3fd1db))
- use ArgoCD application path relatively to git root ([92f0617](https://github.com/mykso/myks/commit/92f0617029802aad66d9851d0926a033b300f083))

### Features

- add a finalizer to ArgoCD project CR ([acf67fd](https://github.com/mykso/myks/commit/acf67fd064578d4d20df52ab931edc48ccba69a5))
- add argocd-apps prototype ([6772744](https://github.com/mykso/myks/commit/677274464f3f1695f1a10ce182c980d5d427640d))
- add arm binaries ([a74d63e](https://github.com/mykso/myks/commit/a74d63ec85a25413a67969004ca5e665119765e6))
- add common overlays example to assets ([39965c5](https://github.com/mykso/myks/commit/39965c56def4320af299ac308244f73d5557b0a8))
- add example environment configs ([8edba12](https://github.com/mykso/myks/commit/8edba129281317e6728d4a9ded061968a3c53c25))
- add flag to control parallelism ([#40](https://github.com/mykso/myks/issues/40)) ([144f5fd](https://github.com/mykso/myks/commit/144f5fd8fc7a7bd6df5241b5421cd079e5cf39f8))
- add git branch detection and refactor data schema ([05e41d4](https://github.com/mykso/myks/commit/05e41d4ec128e134b2684d0a1edc4398661e381d))
- add init command and a data schema file ([c11c27a](https://github.com/mykso/myks/commit/c11c27a49923d20be3781cde0d52989d141e9474))
- add prototypes in the init command ([f31471e](https://github.com/mykso/myks/commit/f31471eeba615640d1e98eaf66d0e52f31f5bfb4))
- add step for rendering ytt packages ([#36](https://github.com/mykso/myks/issues/36)) ([d1078c6](https://github.com/mykso/myks/commit/d1078c63e2e35449c45f20790c3a216fe78cc401))
- Add vendir authentication via environment ([b0c50c2](https://github.com/mykso/myks/commit/b0c50c265348e9b62d064d61740573bf9df1b4f6))
- Add vendir sync caching ([24ff41c](https://github.com/mykso/myks/commit/24ff41ccbb4b002f9f1f434879c0720bd6990924))
- Added docker image ([ae8988d](https://github.com/mykso/myks/commit/ae8988dbcacbf81e553b7669c6b69f377c75b459))
- Added Smart Mode that Automatically detects changed Environment… ([#62](https://github.com/mykso/myks/issues/62)) ([e404b6b](https://github.com/mykso/myks/commit/e404b6b6b6850d60e7739749bb7f5653cb58bbed))
- always write data-schema file ([fa83bee](https://github.com/mykso/myks/commit/fa83beee160797cb1b9d953458370fd4f4c1db70))
- ArgoCD support ([#41](https://github.com/mykso/myks/issues/41)) ([e45d585](https://github.com/mykso/myks/commit/e45d585728e0893648d4ebdf403cc2404872ab0a))
- configure ArgoCD Application finalizers and source.plugin ([#56](https://github.com/mykso/myks/issues/56)) ([80940aa](https://github.com/mykso/myks/commit/80940aa5048f27df8c6060b7ebda77a8aac3eb1e))
- create initial .myks.yaml and print configs ([#87](https://github.com/mykso/myks/issues/87)) ([215ccd3](https://github.com/mykso/myks/commit/215ccd3f51231c20a07c92350eb7d0379dfd0784))
- detect additional and missing applications ([#89](https://github.com/mykso/myks/issues/89)) ([2c7e101](https://github.com/mykso/myks/commit/2c7e1015eb3ec5e9665e202e6b48cba38611f6a1))
- do not convert git URL protocol ([2823eb6](https://github.com/mykso/myks/commit/2823eb67dc4e26aebe1ddcdd02b7f6caf184d158))
- dump configuration as ytt values ([af65436](https://github.com/mykso/myks/commit/af654360516e9cf6931911026253d80f8182cfd7))
- fail on non existing apps ([#52](https://github.com/mykso/myks/issues/52)) ([87aafa3](https://github.com/mykso/myks/commit/87aafa34b85580b06799fcd9fae1e2d063c87ffd)), closes [#3](https://github.com/mykso/myks/issues/3)
- fine-grained ArgoCD project destination ([04d3b78](https://github.com/mykso/myks/commit/04d3b78ac99d785bbb453d6e3d0dad53c3cce09e))
- get git repo URL ([c9b726a](https://github.com/mykso/myks/commit/c9b726a07cc80d5025c5e5dbf7a52f657db198ee))
- **helm:** add support for helm capabilities ([#48](https://github.com/mykso/myks/issues/48)) ([1a13ee1](https://github.com/mykso/myks/commit/1a13ee1886baf5355b0a5db317b8940ce97c61f3)), closes [#31](https://github.com/mykso/myks/issues/31)
- **init:** allow overwriting of data ([#49](https://github.com/mykso/myks/issues/49)) ([f3f5983](https://github.com/mykso/myks/commit/f3f59832463b256535eb712e5c662b3f913bfe4c))
- provide argocd-specific configuration with prototypes ([06e5e5c](https://github.com/mykso/myks/commit/06e5e5cba1350285a5b0e9670c8689d3dc351d41))
- provide example default values for all environments ([0f9cab6](https://github.com/mykso/myks/commit/0f9cab6df81275fee4b3d20d55e78484bf1a5693))
- Push images to docker hub and ghcr ([#65](https://github.com/mykso/myks/issues/65)) ([10bdc63](https://github.com/mykso/myks/commit/10bdc6317e9423907c37c64d86fef91d3c4e97f7))
- Refactoring to make log output more intelligible. ([#39](https://github.com/mykso/myks/issues/39)) ([71cd34c](https://github.com/mykso/myks/commit/71cd34c02451a28aaf3a85f585f9b15cf33edd21))
- release 2.0 ([b7b486d](https://github.com/mykso/myks/commit/b7b486ddac8a14de5a73c7706e383a9cbf944c9d))
- **smart-mode:** configuration option for smart-mode base revision ([#95](https://github.com/mykso/myks/issues/95)) ([4400184](https://github.com/mykso/myks/commit/440018402e4a66b98f14739d4dacab20dac1f33f))
- **smart-mode:** precisely select envs and apps for processing ([#96](https://github.com/mykso/myks/issues/96)) ([ffb47ad](https://github.com/mykso/myks/commit/ffb47ad081af08a68dd930c65e438a3db0454a8a))
- support multiple content items in vendir configs ([#92](https://github.com/mykso/myks/issues/92)) ([fc50be0](https://github.com/mykso/myks/commit/fc50be0497576836b49b42e1e125df3b11fe03e9))
- tweak prefix logic on argo cr to allow for project names like "… ([#72](https://github.com/mykso/myks/issues/72)) ([af01180](https://github.com/mykso/myks/commit/af011801eb29b6ea708af72cf971ac46c8c7b0fd))
- validate root directory ([f81b719](https://github.com/mykso/myks/commit/f81b719f56b2bea9e005c15b1afff339db519d34))
- vendir sync caching ([7279cc7](https://github.com/mykso/myks/commit/7279cc758ea8567262e692262caf815103b86cd4))

### Performance Improvements

- **docker:** ignore not needed files ([3479804](https://github.com/mykso/myks/commit/3479804909eff852b75b2f25848ffd024cf21ee8))

### BREAKING CHANGES

- release 2.0
  This is an empty commit to trigger a major release.

# [1.2.0](https://github.com/mykso/myks/compare/v1.1.0...v1.2.0) (2023-06-03)

### Features

- **cli:** add version information ([#27](https://github.com/mykso/myks/issues/27)) ([a6e16f6](https://github.com/mykso/myks/commit/a6e16f62529c3652065bc101f5b64d948b4142c4))

### Performance Improvements

- process environments and applications in parallel ([#28](https://github.com/mykso/myks/issues/28)) ([f319827](https://github.com/mykso/myks/commit/f3198276f1b54a38ec59aa5eff81ed871b1a4a0a)), closes [#9](https://github.com/mykso/myks/issues/9)

# [1.1.0](https://github.com/mykso/myks/compare/v1.0.0...v1.1.0) (2023-06-03)

### Features

- **cli:** implement `all` subcommand ([#24](https://github.com/mykso/myks/issues/24)) ([ced9961](https://github.com/mykso/myks/commit/ced99618e3399e4d2f177c93caac91463e44496f))

# 1.0.0 (2023-06-03)

### Bug Fixes

- **deps:** update module github.com/spf13/viper to v1.16.0 ([54a6af5](https://github.com/mykso/myks/commit/54a6af5c7abd344b55762a855672a5b25d15c54a))
- helm rendering step ([cb480f4](https://github.com/mykso/myks/commit/cb480f481540134a9991226b112bb7e46e43a34d))
- Install vendir & fix bin name in test action ([#15](https://github.com/mykso/myks/issues/15)) ([57b969b](https://github.com/mykso/myks/commit/57b969b48685b4dc917c76dbca44c30dddf57a62))

### Features

- Expose GitHub action interface ([#12](https://github.com/mykso/myks/issues/12)) ([4ce978e](https://github.com/mykso/myks/commit/4ce978e44db1ea09b4ac5ee2655d4ae53c406103))
