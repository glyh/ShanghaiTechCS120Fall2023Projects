import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import javax.sound.sampled.*

data class MACFrame(
    val dest: ByteArray,
    val src: ByteArray,
    val type: Byte,
    val payload: ByteArray
)

enum class MACState {
    IDLE,
    RxFrame,
    RxFrameAck,
    TxFrame,
    ACKTimeout,
    LinkError
}

class MACLayer {
    private var currentState: MACState = MACState.IDLE
    private var txPending: Boolean = false
    private var retransmissionCount: Int = 0
    private val maxRetransmissions: Int = 3
    private val TIMEOUT: Long = 5000
    private var rxBuffer: ByteArray = byteArrayOf()

    

//    fun handleState(state: currentState){
//        when(currentState){
//            MACState.IDLE ->{
//
//            }
//        }
//    }

    fun send(dest: ByteArray, src: ByteArray, type: Byte, payload: ByteArray) {
        val frame = MACFrame(dest, src, type, payload)

        txPending = true
        currentState = MACState.TxFrame

        sendToPHY(frame)
    }

    fun receive(frame: MACFrame) {
        if (currentState == MACState.IDLE) {

        }
        if (currentState == MACState.RxFrame) {
            simulateReceiveIdle()
        }

        if (frame.type == 0.toByte()) {
            handleAckFrame()
        } else {
            if (!isFrameCorrupted(frame)) {
                handleDataFrame(frame.payload)

                // Report to upper-layer

                // Send ACK frame immediately
                sendAckFrame(frame.src)
            } else {
                println("Received corrupted frame. Discarding.")
            }
        }
    }
    fun frame_detecting() {

    }

    private fun sendAckFrame(dest: ByteArray) {
        val ackFrame = MACFrame(dest, byteArrayOf(), 0.toByte(), byteArrayOf())

        // Simulate sending ACK frame to PHY layer
        sendToPHY(ackFrame)
    }

    private fun sendToPHY(frame: MACFrame) {
        println("Sending data to PHY layer: ${frame.dest}, ${frame.src}, ${frame.type}, ${String(frame.payload)}")
        receive(frame)

        if (currentState == MACState.TxFrame) {
            startTimeoutTimer()
        }
    }

    private fun startTimeoutTimer() {
        println("Timeout timer started")

        Thread.sleep(TIMEOUT)

        if (!ackReceived()) {
            handleTimeout()
        }
    }

    private fun ackReceived(): Boolean {
        // Simulate checking if an ACK was received
        return false
    }

    private fun handleTimeout() {
        if (retransmissionCount < maxRetransmissions) {
            retransmissionCount++
            println("Retransmitting frame (Attempt $retransmissionCount)")
            // Simulate retransmitting the frame
            sendToPHY(MACFrame(byteArrayOf(), byteArrayOf(), 1.toByte(), byteArrayOf()))
        } else {
            terminateMACWithError()
        }
    }

    private fun terminateMACWithError() {
        println("Maximum retransmissions reached. Terminating MAC with an error.")
    }

    private fun simulateReceiveIdle() {
        println("MAC thread is idle. Simulating receiving/detecting frames while idle.")
    }

    private fun isFrameCorrupted(frame: MACFrame): Boolean {
        // Simulate checking if the received frame is corrupted
        return false
    }

    private fun handleAckFrame() {
        println("Received ACK frame")
        clearTimeoutTimer()
    }

    private fun handleDataFrame(data: ByteArray) {
        println("Received DATA frame, append content to reception buffer: ${String(data)}")
        rxBuffer += data

        // Write the received data to OUTPUT.txt
        writeToOutput(data)
    }

    private fun writeToOutput(data: ByteArray) {
        try {
            FileOutputStream("OUTPUT.txt", true).use { fileOutputStream ->
                fileOutputStream.write(data)
                println("Data appended to OUTPUT.txt")
            }
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    private fun clearTimeoutTimer() {
        println("Clearing TIMEOUT timer")
    }
}

fun main() {
    val macLayer = MACLayer()

    // Example MAC frame data
    val dest = byteArrayOf(1.toByte(), 2.toByte(), 3.toByte())
    val src = byteArrayOf(4.toByte(), 5.toByte(), 6.toByte())
    val type = 1.toByte()
    val payload = readFromInput()

    // Send data to MAC layer
    macLayer.send(dest, src, type, payload)
}

private fun readFromInput(): ByteArray {
    val inputFile = File("INPUT.txt")
    val fileBytes = ByteArray(inputFile.length().toInt())

    try {
        FileInputStream(inputFile).use { fileInputStream ->
            fileInputStream.read(fileBytes)
            println("Data read from INPUT.txt")
        }
    } catch (e: Exception) {
        e.printStackTrace()
    }

    return fileBytes
}
