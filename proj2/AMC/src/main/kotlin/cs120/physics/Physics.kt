package cs120.physics

import kotlin.math.sin
import kotlinx.coroutines.channels.*
import kotlin.system.exitProcess
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.DurationUnit
import kotlinx.coroutines.*
import java.nio.ByteBuffer
import javax.sound.sampled.AudioFormat
import javax.sound.sampled.AudioSystem
import javax.sound.sampled.DataLine
import javax.sound.sampled.SourceDataLine

val fs = 44100.0
fun preamble(fs: Double /* frame per sec */): List<Short> {
    val sleep_duration = 300.milliseconds
    val preamble_duration = 800.milliseconds
    val preamble_start_freq = 1000.0 // hz
    val preamble_end_freq = 5000.0 // hz


    val chirp_rate = (preamble_end_freq - preamble_start_freq) / preamble_duration.toDouble(DurationUnit.SECONDS) // hz per second
    val frame_count = (preamble_duration.toDouble(DurationUnit.SECONDS) * fs).toInt()

    return (0 until frame_count).map {
        val f = sin(2 * Math.PI * (
                chirp_rate / 2.0 / fs / fs * it * it +
                        preamble_start_freq / fs * it))
        (f * Short.MAX_VALUE).toInt().toShort()
    }
}

suspend fun play(data: List<Short>) {
    val SAMPLE_SIZE = Short.SIZE_BYTES
    val fFreq = 440.0

    val format = AudioFormat(fs.toFloat(), SAMPLE_SIZE * 8, 1, true, true)
    val info = DataLine.Info(SourceDataLine::class.java, format)
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }

    val line = AudioSystem.getLine(info) as SourceDataLine
    line.open(format)
    line.start()

    val cBuf = ByteBuffer.allocate(line.bufferSize)
    var ctSampleTotal = data.size
    var offset = 0

    var fCyclePosition = 0.0
    while(offset < ctSampleTotal) {
        val fCycleInc = fFreq / fs
        cBuf.clear()
        val ctSampleThisPass = line.available() / SAMPLE_SIZE
        for(i in 0 until ctSampleThisPass){
            cBuf.putShort(data[i + offset])
            fCyclePosition += fCycleInc
            if (fCyclePosition > 1) {
                fCyclePosition -= 1
            }
        }

        line.write(cBuf.array(), 0, cBuf.position())
        ctSampleTotal -= ctSampleThisPass
        while(line.bufferSize / 2 < line.available())
            yield()
        offset += ctSampleThisPass
    }
    line.drain()
    line.close()
}

val mod_duration = 100.milliseconds
val mod_low_freq = 700.0
val mod_high_freq = 18000.0
val mod_width = mod_high_freq - mod_low_freq
val mod_freq_step = 60.0
val mod_freq_range_num = 25
val mod_freq_range_width = mod_width / mod_freq_range_num

fun init(fs: Double) {
    val freq_diff_lower_bound = 1.0 / mod_duration.toDouble(DurationUnit.SECONDS)
    if (freq_diff_lower_bound > mod_freq_step) {
        println("Frequency difference should be bigger")
        exitProcess(1)
    }
}

suspend fun modulate(fs: Double /* frame per sec */, info: Array<Byte>, c: Channel<Short>) {

}

suspend fun send(a: Array<Byte>) {
    var data_channel = Channel<Short>()
    coroutineScope {
        val preamble_sig = preamble(fs)
        val preamble_played = async <Unit> { play(preamble_sig) }
        preamble_played.await()
    }
}

fun receive(): Array<Byte> {
    return arrayOf()
}
