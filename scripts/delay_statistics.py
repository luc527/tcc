import sys
import re
import numpy as np

delay_data = []

for line in sys.stdin:
    line = line.rstrip()

    pat = r"observation: (\d+) topic=(\d+) delayMs=(\d+)"
    match = re.match(pat, line)
    if match:
        delay_ms = int(match.group(3))
        delay_data.append(delay_ms)

delays = np.array(delay_data)
delays.sort()

median = np.median(delays)
mean = np.mean(delays)
std = np.std(delays)

print(f'count:  {len(delays)}')
print(f'median: {median:.2f}')
print(f'mean:   {mean:.2f} +- {std:.2f} std')
