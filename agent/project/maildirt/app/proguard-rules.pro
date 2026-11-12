-keepattributes Signature,InnerClasses,EnclosingMethod,*Annotation*,Exceptions

-dontwarn javax.mail.**
-dontwarn com.sun.mail.**
-dontwarn javax.activation.**
-dontwarn org.bouncycastle.**

-keep class javax.mail.** { *; }
-keep class com.sun.mail.** { *; }
-keep class javax.activation.** { *; }
-keep class org.bouncycastle.** { *; }

-keep class com.gilgamesh.maildirt.MainActivity { *; }
-keep class com.gilgamesh.maildirt.MailNotificationService { *; }
-keep class com.gilgamesh.maildirt.NotificationActionReceiver { *; }

-keep @kotlinx.serialization.Serializable class * { *; }
-keep class **$$serializer { *; }
-keepclassmembers class **$$serializer { *; }
