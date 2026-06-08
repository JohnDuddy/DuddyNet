# Building a signed release

The release build is minified (R8) and signed with a private keystore. **The
keystore and its passwords are never committed** — signing is driven by a
git-ignored `android/keystore.properties`, and the `.jks` lives outside the repo.

## One-time: create a keystore

```powershell
& "C:\Program Files\Android\Android Studio\jbr\bin\keytool.exe" `
  -genkeypair -v -storetype PKCS12 `
  -keystore C:\path\outside\repo\duddynet-release.jks `
  -alias duddynet -keyalg RSA -keysize 2048 -validity 10000 `
  -dname "CN=DuddyNet, O=DuddyNet, C=US"
```

Then create `android/keystore.properties` (git-ignored). **Use forward slashes**
in the path — a `.properties` file treats `\` as an escape character:

```properties
storeFile=C:/path/outside/repo/duddynet-release.jks
storePassword=********
keyAlias=duddynet
keyPassword=********
```

If `keystore.properties` is absent (e.g. a fresh clone or CI without secrets),
the release build still succeeds but is produced **unsigned** — the Gradle config
guards on the file's presence.

## Build

```powershell
cd android
./gradlew :app:assembleRelease
# -> app/build/outputs/apk/release/app-release.apk  (~3 MB, minified + signed)

# For Play Store upload, build an App Bundle instead:
./gradlew :app:bundleRelease
# -> app/build/outputs/bundle/release/app-release.aab
```

Verify the signature:
```powershell
& "$env:LOCALAPPDATA\Android\Sdk\build-tools\34.0.0\apksigner.bat" `
  verify --print-certs app\build\outputs\apk\release\app-release.apk
```

## ⚠️ Back up the keystore

If you ever publish this app (Play Store or any update channel), **every future
update must be signed with the same key**. Losing the `.jks` or its password means
you can never update that app listing again. Back up both:

- the `.jks` file, and
- the `keystore.properties` (or at least the passwords),

to a secure location (password manager / encrypted backup). They are intentionally
**not** in git.

## Install the release build

Release uses applicationId `net.duddy.duddynet` (debug uses
`net.duddy.duddynet.debug`), so both can coexist on a device.

```powershell
& "$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe" `
  install -r android\app\build\outputs\apk\release\app-release.apk
```
