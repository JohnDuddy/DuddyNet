# kotlinx.serialization
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.**
-keepclassmembers class **$$serializer { *; }
-keepclasseswithmembers class net.duddy.duddynet.data.** {
    *** Companion;
}
-keep class net.duddy.duddynet.data.**$$serializer { *; }

# Retrofit
-keepattributes Signature, Exceptions
-keep,allowobfuscation interface retrofit2.Call
-keep,allowobfuscation class retrofit2.Response
-dontwarn okhttp3.**
-dontwarn okio.**
