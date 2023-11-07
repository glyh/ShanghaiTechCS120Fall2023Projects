import math

SAMPLE_RATE = 44100

MOD_DURATION = 0.8 # Sec
MOD_LOW_FREQ = 1000.0 # Hz
MOD_HIGH_FREQ = 17000.0 # Hz
MOD_WIDTH = MOD_HIGH_FREQ - MOD_LOW_FREQ # Hz
MOD_FREQ_STEP = 100.0 # Hz
MOD_FREQ_RANGE_NUM = 10
MOD_FREQ_RANGE_WIDTH = MOD_WIDTH / MOD_FREQ_RANGE_NUM
STATE_PER_RANGE = int(MOD_FREQ_RANGE_WIDTH / MOD_FREQ_STEP)
SYM_SIZE = STATE_PER_RANGE ** MOD_FREQ_RANGE_NUM
BIT_PER_SYM = math.floor(math.log2(SYM_SIZE))

FREQ_DIFF_LOWER_BOUND = 1.0 / MOD_DURATION

SLEEP_DURATION = 0.5 # Sec
PREAMBLE_DURATION = 0.8 # Sec
PREAMBLE_START_FREQ = 1000.0 # Hz
PREAMBLE_FINAL_FREQ = 5000.0 # Hz

LEN_LENGTH = 2

def random_bit_string_of_length(l: int) -> str:
  import random
  return "".join([str(random.randint(0, 1)) for _ in range(l)])

def convert_base(info: str, bit_per_sym: int) -> list[int]: 
  out = []
  info = info + ('0' * (bit_per_sym - ((len(info) - 1) % bit_per_sym + 1)))
  for i in range(0, len(info), bit_per_sym):
    cur = 0
    for j in range(0, bit_per_sym):
      cur = cur << 1 | (1 if info[i+j] != '0' else 0)
    out.append(cur)
  return out

def play_preamble(): 
  pass

if FREQ_DIFF_LOWER_BOUND > MOD_FREQ_STEP: 
  print(f"Frequency difference {MOD_FREQ_STEP} for modulation is too small compared to the lower limit {FREQ_DIFF_LOWER_BOUND}")
  exit(1)

print(f"We're splitting frequency domain [{MOD_LOW_FREQ} {MOD_HIGH_FREQ}] into {MOD_FREQ_RANGE_NUM} pieces, where each piece is of width {MOD_FREQ_RANGE_WIDTH}")

print(f"Inside each range, there's {STATE_PER_RANGE} states where each state is {STATE_PER_RANGE} Hz apart")

print(f"Rounding down the size of symbol set from {SYM_SIZE} to 2^{BIT_PER_SYM}({1 << BIT_PER_SYM}) for simplicity")