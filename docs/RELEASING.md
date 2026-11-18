# Release process

Source publication and binary publication are separate deliverables.

## Source

Run tests and `scripts/export-source.py`. Review the allowlisted export, license
files, source history and secret scan before pushing. Create an independent
TwinGlide repository; never publish an unrelated application repository.
Enable secret scanning, push protection and private vulnerability reporting.
No identity, pairing state, logs, developer toolchain or downloaded engine belongs
in Git. There is no configured upload/publish step in the build scripts.

### 上传源码到 GitHub

1. 在自己的 GitHub 账号下新建空仓库 `TwinGlide`，不额外创建 README 或许可证。
2. 解压 `TwinGlide-0.1.0-alpha.5-source.zip`，以其中的 `TwinGlide` 文件夹为仓库根目录。
3. 使用 GitHub Desktop 添加该文件夹；若提示不是 Git 仓库，先选择在此创建仓库，
   提交文件后再 Publish repository。也可使用 Git 提交后推送到刚创建的空仓库。
   提交作者和远端地址使用自己的信息。
4. 首次推送后检查 Actions 的四平台构建。工作流已经准备，但本地测试不能替代
   GitHub 上真正执行成功的记录。
5. 如发布 alpha Release，请明确标注测试版及 `VALIDATION.md` 中尚未验证的场景。
   公开二进制包前，完成下方第三方对应源码材料和签名相关准备。

源码 ZIP 已按白名单导出；开发电脑上的 `.work`、`.test-data`、`dist`、引擎缓存、
配对证书和日志均不在其中。不要直接打包整个开发目录上传。

## Binary

Build separately on Windows and macOS. engine/pins.json locks official artifacts;
never download "latest" at application startup. Test on a clean OS/user profile.

Before attaching a bundled binary to a public GitHub Release:

- Provide matching Deskflow/dependency source and build materials as required by
  the upstream licenses. Archive the exact tags and dependency versions used by
  the official release; preserve all notices. Verify this for each binary.
- Sign Windows installers/executables and sign/notarize the macOS app for a
  polished public release. Keep signing credentials in CI secrets, outside Git.
- Review the actual test matrix in VALIDATION.md; retain any untested limitations.
- Record SHA-256 checksums for downloadable packages and source bundles.
- Test fresh install, pair on both sides, changed DHCP address, reboot/wake,
  rejection, unpairing, source-role switch, screen position and uninstall.

This alpha's Windows desktop mode deliberately does not install a system service
or promise input in UAC/login/lock-screen desktops. Mac requires the user's
Accessibility and local-network approval. Installing on a machine with an active
sharing app must leave it untouched and display the conflict.
