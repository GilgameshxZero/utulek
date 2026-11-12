.PHONY: all windows test test-windows clean android-debug android-test android-release
all windows test test-windows clean:
	$(MAKE) -C native $@

JAVA_HOME ?= /usr/lib/jvm/java-21-openjdk-amd64
ANDROID_HOME ?= /home/gilgamesh/main.syncthing/utulek/agent/.obj/toolchain/android-sdk
GRADLE_USER_HOME ?= /home/gilgamesh/main.syncthing/utulek/agent/.obj/gradle-home
ANDROID_GRADLE = cd android && JAVA_HOME=$(JAVA_HOME) ANDROID_HOME=$(ANDROID_HOME) GRADLE_USER_HOME=$(GRADLE_USER_HOME) ./gradlew --project-cache-dir .obj/gradle-project-cache

android-debug:
	$(ANDROID_GRADLE) :app:publishDebugApk --no-daemon

android-test:
	$(ANDROID_GRADLE) :app:testDebugUnitTest --no-daemon

android-release:
	cd android && JAVA_HOME=$(JAVA_HOME) ANDROID_HOME=$(ANDROID_HOME) GRADLE_USER_HOME=$(GRADLE_USER_HOME) \
		SMSGATEC_SIGNING_PROPERTIES="$${SMSGATEC_SIGNING_PROPERTIES:-$(CURDIR)/android/.signing/signing.properties}" \
		./gradlew --project-cache-dir .obj/gradle-project-cache :app:publishReleaseApk --no-daemon
