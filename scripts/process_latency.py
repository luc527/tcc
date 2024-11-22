import glob
from collections import defaultdict
import sys

def parse_line(line):
    [timestamp_sec, topic, latency_usec] = line.split(',')
    return int(timestamp_sec), int(latency_usec)

def parse_data(lang):
    paths = glob.glob(f'../data/{lang}*')
    for path in paths:
        if '_mem_latency_' in path:
            mempath = path
        elif '_cpu_latency_' in path:
            cpupath = path
        elif '_cli_latency_' in path and '.csv' in path:
            clipath = path
    latency_data = parse_cli_data(clipath)
    return latency_data

def parse_cli_data(path):
    latency_data = defaultdict(list)
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            timestamp_sec, latency_usec = parse_line(line)
            latency_data[timestamp_sec].append(latency_usec / 1000 / 1000) # now in seconds
    return latency_data

lang = sys.argv[1]

latency_data = parse_data(lang)

beginning = min(latency_data.keys())
latency_data = {t-beginning: v for t, v in latency_data.items()}

latency_mins = {t: min(v) for t, v in latency_data.items()}
latency_maxs = {t: max(v) for t, v in latency_data.items()}
latency_avgs = {t: sum(v)/len(v) for t, v in latency_data.items()}

import matplotlib.pyplot as plt

# TODO: backfill

x = range(min(latency_data.keys()), max(latency_data.keys())+1)
latency_mins_y = [latency_mins[t] if t in latency_mins else 0 for t in x]
latency_maxs_y = [latency_maxs[t] if t in latency_maxs else 0 for t in x]
latency_avgs_y = [latency_avgs[t] if t in latency_avgs else 0 for t in x]

plt.plot(x, latency_mins_y, 'g', latency_maxs_y, 'r', latency_avgs_y, 'b')
plt.show()