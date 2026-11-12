import java.io.File

plugins {
    id("com.android.application") version "8.7.3" apply false
    id("org.jetbrains.kotlin.android") version "2.0.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.21" apply false
    id("org.jetbrains.kotlin.plugin.serialization") version "2.0.21" apply false
}

allprojects {
    layout.buildDirectory.set(
        rootProject.layout.projectDirectory.dir(".obj/build/${project.name}"),
    )
}

val verifyArtifactLayout by tasks.registering {
    doLast {
        val rootDir = rootProject.projectDir
        val strays = listOf("build", "app/build")
            .map { File(rootDir, it) }
            .filter { it.exists() }
        check(strays.isEmpty()) {
            "Build artifacts found outside .obj/.bin: " +
                strays.joinToString { it.path } +
                ". Remove them and rebuild with the documented command."
        }
    }
}
