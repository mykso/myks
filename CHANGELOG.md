# Changelog

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
