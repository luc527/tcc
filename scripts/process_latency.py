import glob
from collections import defaultdict
import sys

from common import parse_latency_data, reindex, to_y

latency_path = sys.argv[1]

latency_data = parse_latency_data(latency_path)

beginning = min(latency_data.keys())
ending    = max(latency_data.keys())
latency_data = reindex(latency_data, beginning, ending)

latency_mins = {t: min(v) for t, v in latency_data.items()}
latency_maxs = {t: max(v) for t, v in latency_data.items()}
latency_avgs = {t: sum(v)/len(v) for t, v in latency_data.items()}

import matplotlib.pyplot as plt

x = range(min(latency_data.keys()), max(latency_data.keys())+1)
latency_mins_y = to_y(latency_mins, x)
latency_maxs_y = to_y(latency_maxs, x)
latency_avgs_y = to_y(latency_avgs, x) 

plt.plot(x, latency_mins_y, 'g', latency_maxs_y, 'r', latency_avgs_y, 'b')
plt.show()