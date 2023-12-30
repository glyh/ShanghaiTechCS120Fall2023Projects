package cs120.RingBuffer

// import java.util.concurrent.atomic.AtomicInteger

class RingBuffer<T>(bufferSize: Int) {
    private val arr: Array<Any?> = arrayOfNulls(bufferSize+1)
    private var head: Int
    private var tail: Int

    init {
        this.head = 0
        this.tail = 0
    }

    fun len() = arr.size

    fun copyStrideRight(rbegin: Int, count: Int, target: Array<T>) {
        val end = arr.size
        val length =
            if (tail < head) {
                tail + end - head
            } else {
                tail - head
            }
        assert(length >= count)
        val rEdge =
            if (tail < rbegin) {
                tail - rbegin + end
            } else {
                tail - rbegin
            }

        @Suppress("UNCHECKED_CAST")
        if (rEdge >= count) {
            arr.copyInto(target as Array<Any?>, 0, rEdge - count, rEdge)
        } else {
            arr.copyInto(target as Array<Any?>, count - rEdge, 0, rEdge)
            arr.copyInto(target as Array<Any?>, 0, end - (count - rEdge), end)
        }
    }

    fun put(item: T) {
        tail = (tail + 1) % arr.size
        arr[tail] = item
        if (head == tail) {
            head = (head + 1) % arr.size
        }
    }
}