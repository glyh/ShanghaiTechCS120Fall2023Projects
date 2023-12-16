package cs120.physics
import cs120.RingBuffer.RingBuffer

import kotlin.math.sin
import kotlinx.coroutines.channels.*
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.DurationUnit
import kotlinx.coroutines.*
import java.nio.ByteBuffer
import javax.sound.sampled.*
import kotlin.time.Duration
import kotlin.math.ceil
import kotlin.math.max
import com.github.psambit9791.jdsp.transform.FastFourier
import kotlin.math.abs

val delay_duration = 5.milliseconds
const val fs = 44100.0
suspend fun sleep(duration: Duration, start: Channel<Boolean>, end: Channel<Boolean>) {
    start.receive()
    delay(duration)
    end.send(true)
}

val preambleDuration = 800.milliseconds
val preambleWidth = (preambleDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()
const val preambleStartFreq = 1000.0 // hz
const val preambleEndFreq = 5000.0 // hz
val chirpRate = (preambleEndFreq - preambleStartFreq) / preambleDuration.toDouble(DurationUnit.SECONDS) // hz per second

val format = AudioFormat(fs.toFloat(), Short.SIZE_BYTES * 8, 1, true, true)
val info = DataLine.Info(SourceDataLine::class.java, format)

suspend fun preamble(start: Channel<Boolean>, end: Channel<Boolean>) {
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }

    val data = (0 until preambleWidth).map {
        val f = sin(2 * Math.PI * (
                chirpRate / 2.0 / fs / fs * it * it +
                        preambleStartFreq / fs * it))
        (f * Short.MAX_VALUE).toInt().toShort()
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

val modDuration = 300.milliseconds
val modWidth = ceil(modDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()

suspend fun sendData(data: List<Byte>, start: Channel<Boolean>, end: Channel<Boolean>) {

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
    val totalSample = data.size * modWidth
    val totalSampleBytes = totalSample / 2 * 3
    while (offset < totalSampleBytes) {
        val samplesThisPass = line.available() / Short.SIZE_BYTES
        val symThisPass = samplesThisPass / modWidth
        val bytesThisPass = symThisPass / 2 * 3
        for(i in 0 until bytesThisPass step 3) {
            val b1 = data[offset+i].toInt()
            val b2 = data[offset+i+1].toInt()
            val b3 = data[offset+i+2].toInt()

            val short1 = ((b1 shl 4) and (b2 shr 4)).toShort()
            val short2 = (((b2 and 0b1111) shl 8) and b3).toShort()

            for(j in 0..modWidth) {
                cBuf.putShort(short1)
            }
            for(j in 0..modWidth) {
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

val sleepDuration = 500.milliseconds

suspend fun send(a: List<Byte>) {
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
        sendData(a, afterPreamble, afterData)
    }
}

fun maxEnergyFrequency(signalShort: Array<Short>): Double {
    val signal = signalShort.map { it.toDouble() }
    val fft = FastFourier(signal.toDoubleArray())
    fft.transform()
    val energies = fft.getMagnitude(true).toTypedArray()
    val frequencies = fft.getFFTFreq(fs.toInt(), true)
    return frequencies[argMax(energies)]
}

fun <T: Comparable<T>> argMax(a: Array<T>): Int {
    var ans = 0
    var maximum = a[0]
    for (i in 1 until a.size) {
        if (a[i] > maximum) {
            maximum = a[i]
            ans = i
        }
    }
    return ans
}

val sliceDuration = 40.milliseconds
val sliceWidth = ceil(sliceDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()
val sliceInnerDuration = 10.milliseconds
val sliceInnerWidth = ceil(sliceInnerDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()

fun tryDetectPreamble(buf: RingBuffer<Short>): Pair<Boolean, Int> {
    val sliceNum = 10

    val cutoffVariancePreamble = 0.5

    var variance = 0.0
    val toAnalyze = Array<Short>(sliceWidth - 2 * sliceInnerWidth) { 0 }
    var freqShift = 0.0
    for(i in 0 until sliceNum) {
        buf.copyStrideRight(
            i * sliceWidth + sliceInnerWidth,
            sliceWidth - 2 * sliceInnerWidth,
            toAnalyze)

        val maxEnergyFreq = maxEnergyFrequency(toAnalyze)
        val avgFreq = preambleEndFreq - (i + 0.5) * sliceDuration.toDouble(DurationUnit.SECONDS) * chirpRate
        if (i == 0 && abs(avgFreq - maxEnergyFreq) < sliceDuration.toDouble(DurationUnit.SECONDS) * chirpRate) {
            freqShift = maxEnergyFreq - avgFreq
        }
        val delta = (avgFreq - maxEnergyFreq + freqShift) / avgFreq
        variance += delta * delta
    }
    return if (variance < cutoffVariancePreamble) {
        Pair(true, ceil(freqShift * fs).toInt())
    } else {
        Pair(false, 0)
    }
}

fun receive(): Array<Byte> {
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }
    val line = AudioSystem.getLine(info) as TargetDataLine
    line.open(format)

    // TODO: fix below
    val singleSymbolSamplesRequired = ceil(modDuration.toDouble(DurationUnit.SECONDS) * fs).toInt()

    // we may look back for 2 symbols
    val bufSize = max(
        preambleWidth,
        singleSymbolSamplesRequired * 2)
    // reserve some space for error
    val buf = RingBuffer<Short>(bufSize * 3)

    line.start()
    val stream = AudioInputStream(line)

    val bytesPerSlice = sliceWidth * Short.SIZE_BYTES

    val sliceBuf = ByteBuffer.allocate(bytesPerSlice)
    var frameCountAll = 0

    var isIdle = true

    while (true) {
        if (isIdle) {
            stream.read(sliceBuf.array(), 0, bytesPerSlice)
            for(i in 0 until bytesPerSlice step Short.SIZE_BYTES) {
                buf.put(sliceBuf.getShort(i))
                frameCountAll += 1
            }
            if (frameCountAll >= preambleWidth) {
                val (ok, frameCountNew) = tryDetectPreamble(buf)
                if(ok) {
                    isIdle = false
                    frameCountAll = frameCountNew
                }
            }
        } else {
            // TODO
        }
    }
}
