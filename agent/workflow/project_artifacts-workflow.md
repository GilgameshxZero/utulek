# Project artifacts: `.obj/` and `.bin/`

Standing preference for every project and every future session.

* **Intermediates** go in a project-local `.obj/`: build intermediates, compiler
  caches, downloaded toolchains, scratch files, temporary test data.
* **Build outputs** go in a project-local `.bin/`: APKs, executables, archives,
  or anything intended for use or distribution.
* Add `.obj/` and `.bin/` to the project `.gitignore`; never commit either.
* Do not leave build output in the project root, `app/build/`, or other ad-hoc
  locations. Install and deploy from `.bin/`, not from build intermediates.

## Gradle layout used by maildirt

Root `build.gradle.kts`:

```kotlin
allprojects {
    layout.buildDirectory.set(
        rootProject.layout.projectDirectory.dir(".obj/build/${project.name}"),
    )
}
```

Publish tasks in `app/build.gradle.kts` copy the APK after assembling:

```kotlin
tasks.register<Copy>("publishDebugApk") {
    dependsOn("assembleDebug")
    from(layout.buildDirectory.dir("outputs/apk/debug"))
    include("*.apk")
    into(rootProject.file(".bin"))
    rename { "maildirt-debug.apk" }
}
```

Use a release equivalent with `assembleRelease`, `outputs/apk/release`, and
`maildirt-release.apk`.

## Build environment

* `GRADLE_USER_HOME` lives in the workspace `.obj` (maildirt uses
  `agent/.obj/gradle-home`).
* Pass `--project-cache-dir .obj/gradle-project-cache`.
* Set `kotlin.project.persistent.dir=.obj/kotlin` in `gradle.properties`.
  Create `.obj/kotlin/sessions` before the first build if the Kotlin plugin
  reports a missing directory.
* Build commands: `:app:publishDebugApk` or `:app:publishReleaseApk`, then
  install from `project/maildirt/.bin/`.

## Enforcement

Do not rely on remembering the convention. Give each project a check like
maildirt's `verifyArtifactLayout` task, wired into `preBuild`, that fails the
build when `build/` or `app/build/` appears outside the allowed directories.
Project caches such as `.gradle/` or `.kotlin/` may be created by an editor or
tool and are covered by `.gitignore`; do not let them fail the build. With the
correct build command, the actual build outputs stay in `.obj/` and `.bin/`.
