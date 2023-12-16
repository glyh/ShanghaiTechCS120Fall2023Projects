import kotlin.math.sin
import kotlinx.coroutines.channels.*
import kotlin.time.Duration
import kotlin.time.Duration.Companion.microseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Duration.*
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.DurationUnit

fun send(a: Array<Byte>) {

}

fun receive(): Array<Byte> {
    return arrayOf()
}

suspend fun preamble(fs: Double /* frame per sec */, c: Channel<Short>) {
    val sleep_duration = 300.milliseconds
    val preamble_duration = 800.milliseconds
    val preamble_start_freq = 1000.0 // hz
    val preamble_end_freq = 5000.0 // hz


    val chirp_rate = (preamble_end_freq - preamble_start_freq) / preamble_duration.toDouble(DurationUnit.SECONDS) // hz per second
    val frame_count = (preamble_duration.toDouble(DurationUnit.SECONDS) * fs).toInt()

    for (offset in 0 until frame_count) {
        val f = sin(2 * Math.PI * (
            chirp_rate / 2.0 / fs / fs * offset * offset +
            preamble_start_freq / fs * offset))
        c.send((f * Short.MAX_VALUE).toInt().toShort())
    }
}

suspend fun send_data(fs: Double /* frame per sec */, info: Array<Byte>, c: Channel<Short>) {
    val mod_duration = 100.milliseconds
    val mod_low_freq = 700
    val mod_high_freq = 18000
    val mod_width = mod_high_freq - mod_low_freq
    val mod_freq_step = 60
    // const mod_duration = 100 * time.Millisecond
    // const mod_low_freq = 700.0
    // const mod_high_freq = 18000.0
    // const mod_width = mod_high_freq - mod_low_freq
    // const mod_freq_step = 60.0
    // // const mod_freq_range_num = 80
    // const mod_freq_range_num = 25
    // const mod_freq_range_width = mod_width / mod_freq_range_num


}