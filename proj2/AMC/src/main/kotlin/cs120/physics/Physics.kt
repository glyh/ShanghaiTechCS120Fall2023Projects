package cs120.physics

import kotlin.math.sin
import kotlinx.coroutines.channels.*
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.DurationUnit
import kotlinx.coroutines.*
import java.nio.ByteBuffer
import javax.sound.sampled.AudioFormat
import javax.sound.sampled.AudioSystem
import javax.sound.sampled.DataLine
import javax.sound.sampled.SourceDataLine
import kotlin.time.Duration

const val fs = 44100.0
suspend fun sleep(duration: Duration, start: Channel<Boolean>, end: Channel<Boolean>) {
    start.receive()
    delay(duration)
    end.send(true)
}

val delay_duration = 5.milliseconds
suspend fun preamble(start: Channel<Boolean>, end: Channel<Boolean>) {
    val preambleDuration = 800.milliseconds
    val preambleStartFreq = 1000.0 // hz
    val preambleEndFreq = 5000.0 // hz

    val chirpRate = (preambleEndFreq - preambleStartFreq) / preambleDuration.toDouble(DurationUnit.SECONDS) // hz per second
    val frameCount = (preambleDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()

    val data = (0 until frameCount).map {
        val f = sin(2 * Math.PI * (
                chirpRate / 2.0 / fs / fs * it * it +
                        preambleStartFreq / fs * it))
        (f * Short.MAX_VALUE).toInt().toShort()
    }

    val format = AudioFormat(fs.toFloat(), Short.SIZE_BYTES * 8, 1, true, true)
    val info = DataLine.Info(SourceDataLine::class.java, format)
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }

    start.receive()

    val line = AudioSystem.getLine(info) as SourceDataLine
    line.open(format)
    line.start()

    val cBuf = ByteBuffer.allocate(line.bufferSize)
    var ctSampleTotal = data.size
    var offset = 0

    while(offset < ctSampleTotal) {
        cBuf.clear()
        val ctSampleThisPass = line.available() / Short.SIZE_BYTES
        for(i in 0 until ctSampleThisPass){
            cBuf.putShort(data[i + offset])
        }

        line.write(cBuf.array(), 0, cBuf.position())
        ctSampleTotal -= ctSampleThisPass
        while(line.bufferSize / 2 < line.available())
            delay(delay_duration)
        offset += ctSampleThisPass
    }
    line.drain()
    line.close()

    end.send(true)
}

suspend fun sendData(data: List<Byte>, start: Channel<Boolean>, end: Channel<Boolean>) {
    val modDuration = 300.milliseconds

    val format = AudioFormat(fs.toFloat(), Short.SIZE_BYTES * 8, 1, true, true)
    val info = DataLine.Info(SourceDataLine::class.java, format)
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }

    // OOK, stuff 3 bytes into 2 shorts
    while(data.size % 3 != 0) {
        data.addLast(0)
    }

    start.receive()

    val line = AudioSystem.getLine(info) as SourceDataLine

    line.open(format)
    line.start()

    val cBuf = ByteBuffer.allocate(line.bufferSize)

    var offset = 0
    val samplePerSym = (modDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()
    val totalSample = data.size * samplePerSym
    val totalSampleBytes = totalSample / 2 * 3
    while (offset < totalSampleBytes) {
        val samplesThisPass = line.available() / Short.SIZE_BYTES
        val symThisPass = samplesThisPass / samplePerSym
        val bytesThisPass = symThisPass / 2 * 3
        for(i in 0 until bytesThisPass step 3) {
            val b1 = data[offset+i].toInt()
            val b2 = data[offset+i+1].toInt()
            val b3 = data[offset+i+2].toInt()

            val short1 = ((b1 shl 4) and (b2 shr 4)).toShort()
            val short2 = (((b2 and 0b1111) shl 8) and b3).toShort()

            for(j in 0..samplePerSym) {
                cBuf.putShort(short1)
            }
            for(j in 0..samplePerSym) {
                cBuf.putShort(short2)
            }
        }
        line.write(cBuf.array(), 0, cBuf.position())
        while(line.bufferSize / 2 < line.available()) {
            delay(delay_duration)
        }
        offset += bytesThisPass
    }
    line.drain()
    line.close()

    end.send(true)
}

suspend fun send(a: List<Byte>) {
    val sleepDuration = 500.milliseconds
    val startSig = Channel<Boolean>()
    val afterSleep = Channel<Boolean>()
    val afterPreamble = Channel<Boolean>()
    val afterData = Channel<Boolean>()
    coroutineScope {
        launch {
            startSig.send(true)
        }
        launch {
            sleep(sleepDuration, startSig, afterSleep)
        }
        launch {
            preamble(afterSleep, afterPreamble)
        }
        launch {
            sendData(a, afterPreamble, afterData)
        }
    }
}

fun receive(): Array<Byte> {
    return arrayOf()
}
