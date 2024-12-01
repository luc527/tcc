import re
import pandas as pd
from collections import defaultdict

def lines_csv(path):
    ls = lines(path)
    next(ls) # ignore first line (header)
    return ls

def lines(path):
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            yield line

def parse_cpu_line(line):
    cols = line.split(',')
    return {
        'timestamp': int(cols[0]),
        'user': float(cols[1]),
        'system': float(cols[2])
    }

def parse_mem_line(line):
    cols = line.split(',')
    return {
        'timestamp': int(cols[0]),
        'uss': int(cols[1]),
    }

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
    iter_data = {}
    delay_data = defaultdict(list)
    mps_data = defaultdict(lambda: defaultdict(lambda: defaultdict(int)))
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            if line.startswith('dbg'):
                pat = r"dbg: (\d+) iteration \d+, topics per conn (\d+)"
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1)) // 1_000_000
                    iter_data[timestamp] = int(m.group(2))
            elif line.startswith('pub'):
                pat = r"pub: (\d+) send topic=(\d+) pub=(\d+)"
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1)) // 1_000_000
                    topic = int(m.group(2))
                    publisher = int(m.group(3))
                    mps_data[topic][publisher][timestamp] += 1
                else:
                    pat = r"pub: (\d+) secv topic=(\d+) pub=(\d+) delayMs=(\d+)"
                    m = re.match(pat, line)
                    if m:
                        timestamp = int(m.group(1)) // 1_000_000
                        delay_ms = int(m.group(4))
                        delay_data[timestamp].append(delay_ms)
    return mps_data, delay_data, iter_data

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

def prepare_throughput_data(mps_data, iter_data, delay_data, cpu_data, mem_data):
    mps_data_degrouped = [{'topic': topic,
                           'publisher': publisher,
                           'timestamp': timestamp,
                           'count': mps_data[topic][publisher][timestamp]}
                           for topic in mps_data
                           for publisher in mps_data[topic]
                           for timestamp in mps_data[topic][publisher]]

    mps_df = pd.DataFrame(mps_data_degrouped)

    beginning = mps_df.timestamp.min()
    ending    = mps_df.timestamp.max()

    mps_df['timestamp'] -= beginning

    iter_data  = reindex(iter_data, beginning, ending)
    delay_df = to_df(reindex(delay_data, beginning, ending))
    cpu_df = to_df(reindex(cpu_data, beginning, ending))
    mem_df = to_df(reindex(mem_data, beginning, ending))

    # make graph smoother
    cpu_df = cpu_df.rolling(5).mean().bfill()

    x = range(mps_df.timestamp.min(), mps_df.timestamp.max()+1)

    cpu_df = cpu_df.reindex(x, method='nearest')
    mem_df = mem_df.reindex(x, method='nearest')

    return x, mps_df, delay_df, iter_data, cpu_df, mem_df

def plot_cpu_usage(ax, x, cpu_df):
    color = 'black'
    ax.set_ylabel('CPU (%)')
    cpu_usr_y = [cpu_df['user'][t] for t in x]
    cpu_sys_y = [cpu_df['system'][t] for t in x]
    ax.plot(x, cpu_usr_y, linewidth=1, linestyle='--', color=color)
    ax.plot(x, cpu_sys_y, linewidth=1, linestyle=':', color=color)
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')

def plot_mem_usage(ax, x, mem_df, color='black'):
    mem_y = [mem_df['uss'][t] / 1024 / 1024 for t in x]
    ax.set_ylabel('Memória (mb)')
    ax.plot(x, mem_y, color=color, linewidth=1)
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')

def plot_ticks(ax, act_data, max, y_max=None):
    color = 'tab:red'
    ticks = [0, *act_data.keys(), max]
    ax.set_xticks(ticks)
    for line_x, topics_per_conn in act_data.items():
        ax.axvline(line_x, color=color, linewidth=1, linestyle=':', alpha=0.5)
        ax.text(line_x+1, 245, f'{topics_per_conn}', color=color, alpha=0.5)