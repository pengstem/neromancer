package dev.mygo

import android.Manifest
import android.annotation.SuppressLint
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Native Android microphone recorder capturing 16kHz 16-bit mono PCM WAV files
 * ready for DeepSeek POST /v1/tools/asr (<= 8 MiB / <= 120s).
 */
class VoiceRecorderPlugin : MyGoPlugin {
    private val recording = AtomicBoolean(false)
    private var recorder: AudioRecord? = null
    private var workerThread: Thread? = null
    private var pcmBuffer: ByteArrayOutputStream? = null
    private var startedAtMs: Long = 0L

    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        when (method) {
            "start" -> start(reply)
            "stop" -> stop(reply)
            else -> reply.failure(PluginFailure("NoMethod", method))
        }
    }

    private fun start(reply: PluginReply) {
        if (recording.get()) {
            reply.success(JSONObject().put("started", true))
            return
        }
        PluginHost.requestPermissions(arrayOf(Manifest.permission.RECORD_AUDIO)) { granted ->
            if (!granted) {
                reply.failure(PluginFailure("PermissionDenied", "请先授予麦克风录音权限后再使用语音速记"))
                return@requestPermissions
            }
            try {
                beginCapture()
                reply.success(JSONObject().put("started", true))
            } catch (e: Exception) {
                cleanupRecorder()
                reply.failure(PluginFailure("RecordFailed", e.message ?: e.toString()))
            }
        }
    }

    @SuppressLint("MissingPermission")
    private fun beginCapture() {
        val sampleRate = 16000
        val minBuf = AudioRecord.getMinBufferSize(
            sampleRate,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT
        )
        val bufSize = maxOf(minBuf, 4096)
        val rec = AudioRecord(
            MediaRecorder.AudioSource.MIC,
            sampleRate,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
            bufSize
        )
        if (rec.state != AudioRecord.STATE_INITIALIZED) {
            rec.release()
            throw IllegalStateException("AudioRecord 初始化失败")
        }

        val out = ByteArrayOutputStream()
        recorder = rec
        pcmBuffer = out
        startedAtMs = System.currentTimeMillis()
        recording.set(true)
        rec.startRecording()

        val maxPcmBytes = 7 * 1024 * 1024 // Stay safely below DeepSeek ASR 8 MiB limit
        val t = Thread {
            val chunk = ByteArray(bufSize)
            while (recording.get() && out.size() < maxPcmBytes) {
                val n = rec.read(chunk, 0, chunk.size)
                if (n > 0) {
                    synchronized(out) {
                        out.write(chunk, 0, n)
                    }
                }
            }
        }
        t.start()
        workerThread = t
    }

    private fun stop(reply: PluginReply) {
        if (!recording.getAndSet(false)) {
            reply.failure(PluginFailure("NotRecording", "当前没有进行中的录音"))
            return
        }
        try {
            recorder?.stop()
        } catch (_: Exception) {
        }
        try {
            workerThread?.join(1000)
        } catch (_: Exception) {
        }

        val durationMs = maxOf(200L, System.currentTimeMillis() - startedAtMs).toInt()
        val pcmBytes = pcmBuffer?.let {
            synchronized(it) { it.toByteArray() }
        } ?: ByteArray(0)
        cleanupRecorder()

        if (pcmBytes.isEmpty()) {
            reply.failure(PluginFailure("EmptyAudio", "未录到有效声音，请靠近麦克风再试一次"))
            return
        }

        try {
            val dir = File(PluginHost.context.cacheDir, "voice").apply { mkdirs() }
            val wavFile = File(dir, "voice_${System.nanoTime()}.wav")
            writeWavFile(wavFile, pcmBytes, 16000, 1, 16)
            reply.success(
                JSONObject()
                    .put("path", wavFile.absolutePath)
                    .put("duration_ms", durationMs)
            )
        } catch (e: Exception) {
            reply.failure(PluginFailure("WriteFailed", e.message ?: e.toString()))
        }
    }

    private fun cleanupRecorder() {
        try {
            recorder?.release()
        } catch (_: Exception) {
        }
        recorder = null
        workerThread = null
        pcmBuffer = null
    }

    private fun writeWavFile(file: File, pcm: ByteArray, sampleRate: Int, channels: Int, bitsPerSample: Int) {
        val byteRate = sampleRate * channels * bitsPerSample / 8
        val blockAlign = (channels * bitsPerSample / 8).toShort()
        val hdr = ByteBuffer.allocate(44).order(ByteOrder.LITTLE_ENDIAN)
        hdr.put("RIFF".toByteArray(Charsets.US_ASCII))
        hdr.putInt(36 + pcm.size)
        hdr.put("WAVE".toByteArray(Charsets.US_ASCII))
        hdr.put("fmt ".toByteArray(Charsets.US_ASCII))
        hdr.putInt(16)
        hdr.putShort(1.toShort()) // PCM
        hdr.putShort(channels.toShort())
        hdr.putInt(sampleRate)
        hdr.putInt(byteRate)
        hdr.putShort(blockAlign)
        hdr.putShort(bitsPerSample.toShort())
        hdr.put("data".toByteArray(Charsets.US_ASCII))
        hdr.putInt(pcm.size)

        FileOutputStream(file).use { fos ->
            fos.write(hdr.array())
            fos.write(pcm)
        }
    }
}
