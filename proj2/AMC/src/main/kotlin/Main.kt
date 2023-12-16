import java.nio.ByteBuffer
import javax.sound.sampled.*
import kotlin.math.sin

fun main(args: Array<String>) {
    val SAMPLING_RATE = 44100.0f
    val SAMPLE_SIZE = Short.SIZE_BYTES
    // print("${SAMPLE_SIZE}!\n")
    val fFreq = 440.0

    val format = AudioFormat(SAMPLING_RATE, 16, 1, true, true)
    val info = DataLine.Info(SourceDataLine::class.java, format)
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }

    val line = AudioSystem.getLine(info) as SourceDataLine
    line.open(format)
    line.start()

    val cBuf = ByteBuffer.allocate(line.bufferSize)
    // print("${cBuf.capacity()}!\n")
    var ctSampleTotal = SAMPLING_RATE * 5

    var fCyclePosition = 0.0
    while(ctSampleTotal > 0) {
        val fCycleInc = fFreq / SAMPLING_RATE
        cBuf.clear()
        val ctSampleThisPass = line.available() / SAMPLE_SIZE
        // print("${ctSampleThisPass}QwQ\n")
        for(i in 0 until ctSampleThisPass){
            val out = Short.MAX_VALUE * sin(2 * Math.PI * fCyclePosition)
            cBuf.putShort(out.toInt().toShort())
            // print("${cBuf.position()}:\n")
            fCyclePosition += fCycleInc
            if (fCyclePosition > 1) {
                fCyclePosition -= 1
            }
        }

        line.write(cBuf.array(), 0, cBuf.position())
        ctSampleTotal -= ctSampleThisPass
        while(line.bufferSize / 2 < line.available())
            Thread.sleep(1)
    }
    line.drain()
    line.close()
}