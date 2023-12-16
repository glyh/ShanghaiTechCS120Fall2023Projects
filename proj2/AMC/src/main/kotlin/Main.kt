import java.nio.ByteBuffer
import java.nio.channels.Channel
import javax.sound.sampled.*
import kotlin.math.sin
import kotlin.system.exitProcess

fun send(a: Array<Byte>) {

}

fun receive(): Array<Byte> {
    return arrayOf()
}

fun dummy() {
    val SAMPLING_RATE = 44100.0f
    val SAMPLE_SIZE = Short.SIZE_BYTES
    val fFreq = 440.0

    val format = AudioFormat(SAMPLING_RATE, SAMPLE_SIZE * 8, 1, true, true)
    val info = DataLine.Info(SourceDataLine::class.java, format)
    if (!AudioSystem.isLineSupported(info)) {
        throw IllegalArgumentException("Audio line not supported")
    }

    val line = AudioSystem.getLine(info) as SourceDataLine
    line.open(format)
    line.start()

    val cBuf = ByteBuffer.allocate(line.bufferSize)
    var ctSampleTotal = SAMPLING_RATE * 5

    var fCyclePosition = 0.0
    while(ctSampleTotal > 0) {
        val fCycleInc = fFreq / SAMPLING_RATE
        cBuf.clear()
        val ctSampleThisPass = line.available() / SAMPLE_SIZE
        for(i in 0 until ctSampleThisPass){
            val out = Short.MAX_VALUE * sin(2 * Math.PI * fCyclePosition)
            cBuf.putShort(out.toInt().toShort())
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

fun preamble(c: Channel) {


}
fun main(args: Array<String>) {
    if (args.size != 1) {
        print("""
            |Usage: [prog] 0/1
            |0: Test
            |1: Send
            |2: Receive
        """.trimMargin())
        exitProcess(0)
    }
    if (args[0] == "0") {
        dummy()
    } else {
        print("TODO")
    }
}