import re
import pandas as pd
from collections import defaultdict

def parse_cpu_data(path):
    data = {}
    with open(path, 'r') as f:
        f.readline() # ignore fst line (header)
        for line in f:
            line = line.rstrip()
            cols = line.split(',')
            timestamp = int(cols[0])
            data[timestamp] = {
                'user': float(cols[1]),
                'system': float(cols[2]),
            }
    return data

def parse_mem_data(path):
    data = {}
    with open(path, 'r') as f:
        f.readline() # ignore fst line (header)
        for line in f:
            line = line.rstrip()
            cols = line.split(',')
            timestamp = int(cols[0])
            data[timestamp] = {
                'uss': int(cols[1]),
            }
    return data

def parse_throughput_data(path):
    throughput_data = defaultdict(list)
    subs_data       = {}
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            if line.startswith('pub'):
                pat = r'pub: (\d+) topic=\d+ pub=\d+ msg=\d+ delayMs=(\d+) frame=(\d+) throughputPsec=([\d.]+)'
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1))
                    throughput_data[timestamp].append({
                        'delay_ms': int(m.group(2)),
                        'frame': int(m.group(3)),
                        'throughput_psec': float(m.group(4)),
                    })
            elif line.startswith('dbg'):
                pat = r'dbg: (\d+) iteration (\d+), topics per conn (\d+)'
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1))
                    subs_data[timestamp] = {
                        'iteration': int(m.group(2)),
                        'topics_per_conn': int(m.group(3)),
                    }
            else:
                print(f'unknown line: {line}')
    return throughput_data, subs_data

def parse_latency_line(line):
    [timestamp_sec, topic, latency_usec] = line.split(',')
    return int(timestamp_sec), int(latency_usec)

def parse_latency_data(path):
    latency_data = defaultdict(list)
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            timestamp_sec, latency_usec = parse_latency_line(line)
            latency_data[timestamp_sec].append(latency_usec / 1000 / 1000) # now in seconds
    return latency_data

def reindex(dic, beginning, ending):
    return {t-beginning: v 
            for t, v in dic.items()
            if beginning <= t <= ending}

def to_y(dic, ran, f=None):
    lis = []
    prev = None
    # forward fill
    for i in ran:
        if i in dic:
            val = f(dic[i]) if f else dic[i]
            lis.append(val)
            prev = val
        else:
            lis.append(prev)
    return lis

def to_df(dic):
    lis = [{'timestamp': t, **v} for t, v in dic.items()]
    return pd.DataFrame(lis)