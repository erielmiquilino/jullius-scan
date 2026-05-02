---
name: release-app
description: Bump the Flutter app's version (default patch + build number) and build a release APK pointing to the production backend. Use when the user asks to "build new apk", "gerar novo apk", "release apk", "release apk com bump", "incrementar versão e gerar apk", "subir versão", or any phrasing that asks for a versioned APK to install on a phone. The skill produces a single APK at mobile/build/releases/jullius-scan-X.Y.Z.apk and leaves pubspec.yaml updated for commit.
---

# release-app

Single responsibility: bump the version in `mobile/pubspec.yaml` and produce a signed-with-debug-key APK pointing to `https://jullius-scan.erielmiquilino.me`.

## When to invoke

- "Gera um APK novo" / "build a new APK"
- "Sobe a versão e gera APK" / "bump and build"
- "Quero instalar a versão atualizada"

## When NOT to invoke

- The user only wants source changes without an APK
- The user is editing native Android config or release signing — those need manual flutter build invocation
- The user wants a debug build (use `flutter run` instead)

## Steps

1. Confirm the version bump segment with the user only if they asked for `major`/`minor` explicitly. **Otherwise default to `patch`.**
2. Run from the repository root:
   ```
   pwsh scripts/release.ps1
   ```
   Use `-Bump minor` or `-Bump major` for those increments. Use `-ApiBaseUrl <url>` only if the user explicitly wants a different backend (e.g., a staging URL in the future).
3. The script:
   - Reads `version: X.Y.Z+N` from `mobile/pubspec.yaml`
   - Increments the chosen segment and the build number (always)
   - Runs `flutter pub get` + `flutter build apk --release --dart-define=API_BASE_URL=...`
   - Copies the result to `mobile/build/releases/jullius-scan-X.Y.Z.apk`
4. Report to the user: the new version string and the absolute path of the APK.
5. Do NOT auto-commit the pubspec.yaml change. Leave it for the user to commit so they can bundle it with whatever feature/fix triggered the release.

## Notes

- The APK is signed with the **debug keystore** (existing project default). It installs fine on any Android device via "Install from unknown sources" but is not Play-Store-distributable. Builds from a different machine will use a different debug key, forcing the user to uninstall before installing. Do not change this without the user asking — it's a one-time decision documented elsewhere.
- The version is read by `package_info_plus` and shown as a discreet `vX.Y.Z+N` label at the bottom of the login screen, so the user can confirm which build is installed.
- If the build fails with "Unable to write snapshot file", run `flutter clean` from `mobile/` and retry — this is a Windows/OneDrive lock issue and not a code problem.
